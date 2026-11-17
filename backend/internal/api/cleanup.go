// Package api 存量重复清洗：scan（重复扫描与保留计划）→ execute（快照+原子清理）→ restore（回滚）。
// 规划：.claude/plan/db-import-clean-dedupe.md 任务 3.2/3.3。
// 口径（纪律 1 推论）：tags 永不物理删——remove 预标 delete 的行走快照+物理删（tags 置 void），
// 有履历/扫码的行走 void（资产改报废保留，码作废）。
package api

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"mutag/backend/internal/domain"
	"mutag/backend/internal/repo"
)

// cleanupMember 扫描组成员明细。
type cleanupMember struct {
	TagID     string `json:"tag_id"`
	Name      string `json:"name"`
	Keeper    string `json:"keeper"`
	BuyDate   string `json:"buy_date"`
	CreatedAt string `json:"created_at"`
	Status    string `json:"status"`
	Events    int64  `json:"events"`
	Scans     int64  `json:"scans"`
	Keep      bool   `json:"keep"`    // 保留策略结果
	Action    string `json:"action"`  // delete（无履历垃圾）| void（有履历，改报废）| ""（保留行）
}

// cleanupGroup 每个重复键一组。
type cleanupGroup struct {
	Key         string          `json:"key"`
	Name        string          `json:"name"`
	Keeper      string          `json:"keeper"`
	BuyDate     string          `json:"buy_date"`
	Members     []cleanupMember `json:"members"`
	Clusters    int             `json:"clusters"` // created_at 时间簇数（同簇数=1 疑似合法 qty 拆分）
	Expected    int             `json:"expected"` // align 模式的目标条数（all 模式 = 0 不比对）
	RemoveCount int             `json:"remove_count"`
}

// cleanupScanRequest 扫描入参。
type cleanupScanRequest struct {
	Mode         string       `json:"mode"`          // all | align
	KeepStrategy string       `json:"keep_strategy"` // earliest_1 | latest_1 | manual
	Items        []importItem `json:"items"`         // align 模式的权威解析结果（期望条数 = Σqty）
}

// cleanupScan POST /api/admin/cleanup/scan —— 全库重复扫描与保留计划草案（只读）。
func (d Deps) cleanupScan(c *gin.Context) {
	var req cleanupScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "参数错误"})
		return
	}
	if req.KeepStrategy == "" {
		req.KeepStrategy = "earliest_1"
	}
	if req.Mode != "all" && req.Mode != "align" {
		c.JSON(400, gin.H{"error": "mode 须 all|align"})
		return
	}
	ctx := c.Request.Context()

	// 库内全部 active 资产（量级千内，内存分组）
	rows, err := d.Store.DB.QueryContext(ctx,
		`SELECT a.tag_id, a.name, COALESCE(a.keeper,''), COALESCE(a.buy_date,''),
		        COALESCE(a.created_at,''), a.status,
		        (SELECT count(*) FROM asset_events e WHERE e.asset_id = a.asset_id),
		        (SELECT count(*) FROM scan_logs s WHERE s.tag_id = a.tag_id)
		 FROM assets a WHERE a.status = ?`, domain.AssetInUse)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	all := []cleanupMember{}
	for rows.Next() {
		var m cleanupMember
		if err := rows.Scan(&m.TagID, &m.Name, &m.Keeper, &m.BuyDate,
			&m.CreatedAt, &m.Status, &m.Events, &m.Scans); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		all = append(all, m)
	}
	if err := rows.Err(); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// align 模式的期望分布
	expected := make(map[string]int)
	if req.Mode == "align" {
		for _, u := range expandUnits(req.Items) {
			expected[u.Key]++
		}
	}

	// 按键分组
	groupMap := make(map[string]*cleanupGroup)
	order := []string{}
	for _, m := range all {
		k := dedupeKey(m.Name, m.Keeper, m.BuyDate)
		g, ok := groupMap[k]
		if !ok {
			g = &cleanupGroup{Key: k, Name: m.Name, Keeper: m.Keeper, BuyDate: m.BuyDate, Expected: expected[k]}
			groupMap[k] = g
			order = append(order, k)
		}
		g.Members = append(g.Members, m)
	}
	sort.Strings(order)

	groups := make([]cleanupGroup, 0, len(order))
	sumDelete, sumVoid, sumKeep := 0, 0, 0
	for _, k := range order {
		g := groupMap[k]
		// 只输出"重复"组：count>1（all）或 超出期望（align）
		isDup := len(g.Members) > 1 || (req.Mode == "align" && g.Expected > 0 && len(g.Members) > g.Expected)
		if !isDup {
			continue
		}
		// 时间簇：created_at 逐分钟归桶
		g.Clusters = timeClusters(g.Members)
		// 保留策略（manual 交前端，后端按 earliest_1 预排；keep 列照标，前端可改）
		keepN := 1
		if req.Mode == "align" && g.Expected > 0 {
			keepN = g.Expected // 对齐模式保留至期望条数
		}
		if req.KeepStrategy == "latest_1" {
			latest := g.Members
			sort.Slice(latest, func(i, j int) bool { return latest[i].CreatedAt > latest[j].CreatedAt })
			for i := range latest {
				g.Members[i].Keep = i < keepN
			}
		} else { // earliest_1 / manual 预排
			earliest := append([]cleanupMember(nil), g.Members...)
			sort.Slice(earliest, func(i, j int) bool { return earliest[i].CreatedAt < earliest[j].CreatedAt })
			keepSet := make(map[string]bool)
			for i, m := range earliest {
				if i < keepN {
					keepSet[m.TagID] = true
				}
				_ = m
			}
			for i := range g.Members {
				g.Members[i].Keep = keepSet[g.Members[i].TagID]
			}
		}
		// action 预标：保留行空；清理行——无履历且无扫码 → delete；否则 → void
		for i := range g.Members {
			m := &g.Members[i]
			if m.Keep {
				sumKeep++
				continue
			}
			if m.Events <= 1 && m.Scans == 0 {
				// 仅 create event（导入自身写的那条）= 无业务履历 → 物理删
				m.Action = "delete"
				sumDelete++
			} else {
				m.Action = "void"
				sumVoid++
			}
		}
		g.RemoveCount = len(g.Members) - keepN
		if g.RemoveCount < 0 {
			g.RemoveCount = 0
		}
		groups = append(groups, *g)
	}

	c.JSON(200, gin.H{
		"groups":  groups,
		"summary": gin.H{"delete": sumDelete, "void": sumVoid, "keep": sumKeep},
	})
}

// timeClusters created_at 按分钟归桶计数（同簇=1 疑似合法 qty 拆分信号）。
func timeClusters(members []cleanupMember) int {
	buckets := make(map[string]bool)
	for _, m := range members {
		if len(m.CreatedAt) >= 16 {
			buckets[m.CreatedAt[:16]] = true
		}
	}
	if len(buckets) == 0 {
		return 1
	}
	return len(buckets)
}

// cleanupExecuteRequest 执行入参（plan 由前端基于 scan 结果调整后回传）。
type cleanupExecuteRequest struct {
	Mode          string   `json:"mode"`
	KeepStrategy  string   `json:"keep_strategy"`
	DeleteTagIDs  []string `json:"delete_tag_ids"` // 物理删（快照兜底）
	VoidTagIDs    []string `json:"void_tag_ids"`   // 改报废保留（码作废）
	ConfirmCount  int      `json:"confirm_count"`  // = len(delete)+len(void)，不符 400 拒绝
}

// cleanupExecute POST /api/admin/cleanup/execute —— 快照 + 原子清理（规划任务 3.3）。
func (d Deps) cleanupExecute(c *gin.Context) {
	var req cleanupExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "参数错误"})
		return
	}
	if req.ConfirmCount != len(req.DeleteTagIDs)+len(req.VoidTagIDs) {
		c.JSON(400, gin.H{"error": fmt.Sprintf("confirm_count(%d) 与 plan 总数(%d) 不符", req.ConfirmCount, len(req.DeleteTagIDs)+len(req.VoidTagIDs))})
		return
	}
	if len(req.DeleteTagIDs) == 0 && len(req.VoidTagIDs) == 0 {
		c.JSON(400, gin.H{"error": "plan 为空"})
		return
	}
	ctx := c.Request.Context()
	actor := "admin"
	// suffix 兼作快照表名段（_bak_{suffix}_assets）——纯字母数字，防标识符语法坑
	suffix := time.Now().Format("0102") + strings.ToLower(fmt.Sprintf("%x", time.Now().UnixNano()))[:8]

	planJSON, _ := json.Marshal(gin.H{"delete": req.DeleteTagIDs, "void": req.VoidTagIDs})
	summaryJSON, _ := json.Marshal(gin.H{"delete": len(req.DeleteTagIDs), "void": len(req.VoidTagIDs)})
	job := &repo.CleanupJob{
		JobID: suffix, Actor: actor, Mode: req.Mode, KeepStrategy: req.KeepStrategy,
		Summary: string(summaryJSON), Planned: string(planJSON), Status: "planned",
	}
	if err := d.Store.Cleanup().CreateJob(ctx, job); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// void 组：资产改报废（保留履历）+ 码作废（复用既有 SetStatus 链路）
	for _, tagID := range req.VoidTagIDs {
		if err := d.Store.Assets().SetStatus(ctx, tagID, domain.AssetScrap); err != nil {
			c.JSON(500, gin.H{"error": "void " + tagID + ": " + err.Error(), "job_id": suffix})
			return
		}
		note := "重复资产清理（批次 " + suffix + "）"
		if a, err := d.Store.Assets().GetByTagID(ctx, tagID); err == nil {
			_ = d.Store.Events().Create(ctx, &domain.AssetEvent{
				AssetID: a.AssetID, Type: domain.EventScrap, Actor: actor,
				NewValue: map[string]any{"status": "scrap"}, Note: &note,
			})
		}
	}

	// delete 组：快照 + 物理删（tags 置 void 保留号簿）
	removed, voided, err := d.Store.Cleanup().SnapshotAndExecute(ctx, suffix, req.DeleteTagIDs)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error(), "job_id": suffix})
		return
	}
	if err := d.Store.Cleanup().MarkExecuted(ctx, suffix, suffix, removed, voided+int64(len(req.VoidTagIDs))); err != nil {
		c.JSON(500, gin.H{"error": err.Error(), "job_id": suffix})
		return
	}
	c.JSON(200, gin.H{
		"job_id": suffix, "deleted": removed, "voided": voided + int64(len(req.VoidTagIDs)),
		"kept": len(req.DeleteTagIDs) + len(req.VoidTagIDs), // 语义：本轮处理条数
	})
}

// cleanupJobs GET /api/admin/cleanup/jobs —— 清理历史（审计）。
func (d Deps) cleanupJobs(c *gin.Context) {
	jobs, err := d.Store.Cleanup().List(c.Request.Context())
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"jobs": jobs})
}

// cleanupRestore POST /api/admin/cleanup/jobs/:id/restore —— 从快照回补（规划任务 3.3）。
func (d Deps) cleanupRestore(c *gin.Context) {
	jobID := c.Param("jobID")
	job, err := d.Store.Cleanup().Get(c.Request.Context(), jobID)
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	if job.SnapshotSuffix == "" {
		c.JSON(400, gin.H{"error": "该 job 无快照（未物理删除）"})
		return
	}
	if job.Status == "restored" {
		c.JSON(400, gin.H{"error": "已回滚过，勿重复操作"})
		return
	}
	restored, err := d.Store.Cleanup().RestoreFromSnapshot(c.Request.Context(), job.SnapshotSuffix)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if err := d.Store.Cleanup().MarkRestored(c.Request.Context(), jobID); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"job_id": jobID, "restored": restored})
}
