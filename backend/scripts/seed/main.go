// seed 一次性导入脚本：读转换后的 JSON（由 scripts/xlsx_to_json.py 从 xlsx 生成），
// 一物一码拆分入库。数量>1 的采购行拆为 N 条独立资产，各配一码，附 create 履历。
// 幂等保护：库中已有资产时拒绝执行。
//
// 两步执行:
//   python3 scripts/xlsx_to_json.py ../../固定资产需打标签.xlsx scripts/seed/data.json
//   go run ./scripts/seed
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"mutag/backend/internal/db"
	"mutag/backend/internal/domain"
	"mutag/backend/internal/repo"
)

// seedRecord xlsx 一行采购记录（xlsx_to_json.py 输出格式）。
type seedRecord struct {
	Seq     string  `json:"seq"`
	Name    string  `json:"name"`
	BuyDate *string `json:"buy_date"` // YYYY-MM-DD 或 null
	Qty     int     `json:"qty"`
	Unit    *string `json:"unit"`
	Keeper  *string `json:"keeper"`
}

func parseDate(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	t, err := time.ParseInLocation("2006-01-02", *s, time.Local)
	if err != nil {
		return nil
	}
	return &t
}

func main() {
	file := flag.String("file", "scripts/seed/data.json", "seed JSON 路径（相对 backend/）")
	actor := flag.String("actor", "seed", "create 履历的操作人")
	flag.Parse()

	ctx := context.Background()

	database, err := db.OpenSQLite("data/mutag.db")
	if err != nil {
		log.Fatalf("sqlite: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(ctx, database); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	store := repo.NewStore(database)
	assets := store.Assets()
	tags := store.Tags()
	events := store.Events()

	// 幂等保护：已有资产拒绝重复导入
	if n, err := store.Count(ctx); err != nil {
		log.Fatalf("count: %v", err)
	} else if n > 0 {
		log.Fatalf("库中已有 %d 条资产，拒绝重复导入（如需重导请先删除 data/mutag.db）", n)
	}

	raw, err := os.ReadFile(*file)
	if err != nil {
		log.Fatalf("read json: %v", err)
	}
	var records []seedRecord
	if err := json.Unmarshal(raw, &records); err != nil {
		log.Fatalf("parse json: %v", err)
	}

	created := 0
	tagSeq := 0
	for _, rec := range records {
		// 一物一码：数量拆分为 N 条独立资产
		for j := 0; j < rec.Qty; j++ {
			tagSeq++
			tagID := fmt.Sprintf("MT-2026-%06d", tagSeq)

			tag := &domain.Tag{
				TagID:   tagID,
				QrScene: "t:" + tagID,
				UrlCode: tagID,
				Status:  domain.TagActive,
			}
			if err := tags.Insert(ctx, tag); err != nil {
				log.Fatalf("seq%s tag %s: %v", rec.Seq, tagID, err)
			}

			asset := &domain.Asset{
				TagID:   tagID,
				Name:    rec.Name,
				BuyDate: parseDate(rec.BuyDate),
				Keeper:  rec.Keeper,
				Status:  domain.AssetInUse,
				Unit:    rec.Unit,
			}
			assetID, err := assets.Create(ctx, asset)
			if err != nil {
				log.Fatalf("seq%s asset %s: %v", rec.Seq, tagID, err)
			}

			note := fmt.Sprintf("来源采购行序号 %s（第 %d/%d 件）", rec.Seq, j+1, rec.Qty)
			ev := &domain.AssetEvent{
				AssetID: assetID,
				Type:    domain.EventCreate,
				Actor:   *actor,
				NewValue: map[string]any{
					"name": rec.Name, "keeper": keeperStr(rec.Keeper), "status": "in_use",
				},
				Note: &note,
			}
			if err := events.Create(ctx, ev); err != nil {
				log.Fatalf("seq%s event %s: %v", rec.Seq, tagID, err)
			}
			created++
		}
	}

	fmt.Printf("导入完成：%d 条资产（一物一码），tag 编号 MT-2026-000001 ~ MT-2026-%06d\n",
		created, tagSeq)
}

func keeperStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
