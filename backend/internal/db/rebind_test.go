package db

// RebindPG 单测：? → $n 转换、引号内保护、参数个数守卫。

import "testing"

func TestRebindPG(t *testing.T) {
	cases := []struct {
		name  string
		query string
		n     int
		want  string
	}{
		{"单参数", "SELECT * FROM t WHERE a = ?", 1, "SELECT * FROM t WHERE a = $1"},
		{"多参数有序", "INSERT INTO t (a,b,c) VALUES (?, ?, ?)", 3, "INSERT INTO t (a,b,c) VALUES ($1, $2, $3)"},
		{"引号内不动", "SELECT * FROM t WHERE note = 'what?' AND a = ?", 1,
			"SELECT * FROM t WHERE note = 'what?' AND a = $1"},
		{"无参数原样", "SELECT 1", 0, "SELECT 1"},
		{"参数多于占位符原样返回", "SELECT 1", 2, "SELECT 1"},
		{"多位序号", "INSERT INTO t (a,b,c,d,e,f,g,h,i,j,k) VALUES (?,?,?,?,?,?,?,?,?,?,?)", 11,
			"INSERT INTO t (a,b,c,d,e,f,g,h,i,j,k) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := RebindPG(c.query, c.n); got != c.want {
				t.Fatalf("RebindPG(%q, %d) = %q, want %q", c.query, c.n, got, c.want)
			}
		})
	}
}
