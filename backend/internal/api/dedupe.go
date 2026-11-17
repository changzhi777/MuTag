// Package api 导入去重：键归一化 + 库内计数 + min/max 幂等分类。
// 规划：.claude/plan/db-import-clean-dedupe.md 任务 2.2/2.3。
// 语义：dedup=true 时每键跳过 min(C_db, C_in) 条、新建 max(0, C_in-C_db) 条；dedup=false 全量新建。
package api

import (
	"context"
	"database/sql"
	"sort"
	"strings"

	"mutag/backend/internal/domain"
)

// dedupeKey 归一化键：norm(name)|norm(keeper)|buy_date(YYYY-MM-DD)。
// 名称/保管人小写 + trim + 压缩连续空白；日期保持入库格式（TEXT 字典序）。
func dedupeKey(name, keeper, buyDate string) string {
	norm := func(s string) string {
		return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
	}
	return norm(name) + "|" + norm(keeper) + "|" + strings.TrimSpace(buyDate)
}

// dedupeUnit 展平后的导入单元（qty 已拆开，一件一条）。
type dedupeUnit struct {
	Key     string
	Name    string
	Keeper  string
	BuyDate string
	Seq     string // 来源行序号（响应明细展示用）
}

// expandUnits 把 importItem 列表按 qty 拆成单元序列（空名行跳过，qty<1 视为 1）。
func expandUnits(items []importItem) []dedupeUnit {
	var units []dedupeUnit
	for _, rec := range items {
		name := strings.TrimSpace(rec.Name)
		if name == "" {
			continue
		}
		qty := rec.Qty
		if qty < 1 {
			qty = 1
		}
		bd, keeper := "", ""
		if rec.BuyDate != nil {
			bd = strings.TrimSpace(*rec.BuyDate)
		}
		if rec.Keeper != nil {
			keeper = strings.TrimSpace(*rec.Keeper)
		}
		for j := 0; j < qty; j++ {
			units = append(units, dedupeUnit{
				Key: dedupeKey(name, keeper, bd), Name: name, Keeper: keeper, BuyDate: bd, Seq: rec.Seq,
			})
		}
	}
	return units
}

// loadExistingCounts 库内 active 资产按键计数（全量拉取内存重建，资产量级千内代价可忽略；
// 规避 66+ 参数 IN 组合查询的方言差异与注入面）。仅 status=in_use（作废/报废资产不参与去重）。
func loadExistingCounts(ctx context.Context, db *sql.DB) (map[string]int, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT name, COALESCE(keeper, ''), COALESCE(buy_date, '') FROM assets WHERE status = ?`,
		domain.AssetInUse)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int)
	for rows.Next() {
		var name, keeper, bd string
		if err := rows.Scan(&name, &keeper, &bd); err != nil {
			return nil, err
		}
		out[dedupeKey(name, keeper, bd)]++
	}
	return out, rows.Err()
}

// classify 按 min/max 幂等语义把单元分派到 create / skip。
// 同键多条按输入顺序消耗库内额度：额度内跳过、超额新建。
func classify(units []dedupeUnit, existing map[string]int, dedup bool) (create, skip []dedupeUnit) {
	used := make(map[string]int)
	for _, u := range units {
		if !dedup {
			create = append(create, u)
			continue
		}
		if used[u.Key] < existing[u.Key] {
			skip = append(skip, u)
		} else {
			create = append(create, u)
		}
		used[u.Key]++
	}
	return
}

// dedupeGroup precheck 响应的按键分组（每键一行摘要）。
type dedupeGroup struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Keeper   string `json:"keeper"`
	BuyDate  string `json:"buy_date"`
	Incoming int    `json:"incoming"`
	Existing int    `json:"existing"`
	Verdict  string `json:"verdict"` // exact（将跳过部分或全部）/ partial（库内不足将补差）/ new（全新）
}

// buildGroups 汇总每键 incoming/existing 并判 verdict（按 incoming 排序保证响应稳定）。
func buildGroups(units []dedupeUnit, existing map[string]int) []dedupeGroup {
	inc := make(map[string]*dedupeGroup)
	order := []string{}
	for _, u := range units {
		g, ok := inc[u.Key]
		if !ok {
			g = &dedupeGroup{Key: u.Key, Name: u.Name, Keeper: u.Keeper, BuyDate: u.BuyDate}
			inc[u.Key] = g
			order = append(order, u.Key)
		}
		g.Incoming++
	}
	sort.Strings(order)
	groups := make([]dedupeGroup, 0, len(order))
	for _, k := range order {
		g := inc[k]
		g.Existing = existing[k]
		switch {
		case g.Incoming <= g.Existing:
			g.Verdict = "exact"
		case g.Existing > 0:
			g.Verdict = "partial"
		default:
			g.Verdict = "new"
		}
		groups = append(groups, *g)
	}
	return groups
}

// countSuspect 疑似重复数：create 集合中与库内「同名同保管人但日期不同（含库方缺失）」的键数。
// 仅作 precheck 标注（verdict=loose），不参与 skip 逻辑。
func countSuspect(create []dedupeUnit, existing map[string]int) int {
	loose := make(map[string]bool)
	for k := range existing {
		parts := strings.SplitN(k, "|", 3)
		if len(parts) == 3 {
			loose[parts[0]+"|"+parts[1]] = true
		}
	}
	hit := make(map[string]bool)
	for _, u := range create {
		nk := dedupeKey(u.Name, u.Keeper, "")
		if loose[strings.TrimSuffix(nk, "|")] && !hit[u.Key] {
			hit[u.Key] = true
		}
	}
	return len(hit)
}
