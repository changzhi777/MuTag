// rebind_driver.go PG 形态的占位符适配层：database/sql 风格 `?` → PG 风格 `$1..$n`。
// 裁决依据：repo 层 SQL 双方言一份（? 风格），方言差异收敛在 internal/db——
// pgx 全链路（含 simple protocol）只认 $n，故在此驱动包装层统一 rebind，repo 零感知。
package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"strconv"
	"strings"
)

// rebindingConnector 包装 pgx stdlib connector，注入 rebind 行为。
type rebindingConnector struct {
	inner driver.Connector
}

func (c rebindingConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &rebindingConn{Conn: conn}, nil
}

func (c rebindingConnector) Driver() driver.Driver { return c.inner.Driver() }

// OpenWithRebind 以 rebind 包装打开 PG 连接（pgx stdlib driver 之上）。
func OpenWithRebind(connector driver.Connector) *sql.DB {
	return sql.OpenDB(rebindingConnector{inner: connector})
}

// rebindingConn 转发连接并 rebind 查询/执行（覆盖 QueryerContext/ExecerContext 路径）。
type rebindingConn struct {
	driver.Conn
}

var _ driver.QueryerContext = (*rebindingConn)(nil)
var _ driver.ExecerContext = (*rebindingConn)(nil)

func (c *rebindingConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if q, ok := c.Conn.(driver.QueryerContext); ok {
		return q.QueryContext(ctx, RebindPG(query, len(args)), args)
	}
	return nil, driver.ErrSkip
}

func (c *rebindingConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if e, ok := c.Conn.(driver.ExecerContext); ok {
		return e.ExecContext(ctx, RebindPG(query, len(args)), args)
	}
	return nil, driver.ErrSkip
}

// Prepare 兜底（pgx 走 QueryerContext/ExecerContext，极少触发；rebind 后转发）。
func (c *rebindingConn) Prepare(query string) (driver.Stmt, error) {
	stmt, err := c.Conn.Prepare(query)
	if err != nil {
		return nil, err
	}
	return &rebindingStmt{Stmt: stmt}, nil
}

type rebindingStmt struct {
	driver.Stmt
}

// NumInput 返回 -1：绕过 database/sql 的参数个数校验（rebind 动态生成 $n）。
func (s *rebindingStmt) NumInput() int { return -1 }

func (s *rebindingStmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.Stmt.Query(args)
}

func (s *rebindingStmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.Stmt.Exec(args)
}

// RebindPG 将 `?` 占位符按出现顺序替换为 `$1..$n`（引号字符串内的 ? 不动）。
// n 须与占位符个数一致（database/sql 保证 args 与占位符一一对应）。
func RebindPG(query string, n int) string {
	if !strings.Contains(query, "?") || n <= 0 {
		return query
	}
	var b strings.Builder
	b.Grow(len(query) + n*2)
	argIdx := 0
	inQuote := false
	for i := 0; i < len(query); i++ {
		ch := query[i]
		switch {
		case ch == '\'':
			inQuote = !inQuote
			b.WriteByte(ch)
		case ch == '?' && !inQuote:
			argIdx++
			if argIdx > n {
				return query // 占位符多于参数：原样返回让驱动报出真实错误
			}
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(argIdx))
		default:
			b.WriteByte(ch)
		}
	}
	if argIdx != n {
		return query
	}
	return b.String()
}
