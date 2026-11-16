package repo

import (
	"context"
	"database/sql"
	"encoding/json"

	"mutag/backend/internal/domain"
)

// EventRepo 资产履历数据访问。
type EventRepo struct {
	db *sql.DB
}

// ListByAsset 按资产取履历（新→旧）。
func (r *EventRepo) ListByAsset(ctx context.Context, assetID int64, limit int) ([]domain.AssetEvent, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT event_id, asset_id, type, actor, old_value, new_value, note, created_at
		 FROM asset_events WHERE asset_id = ? ORDER BY created_at DESC LIMIT ?`,
		assetID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.AssetEvent
	for rows.Next() {
		var e domain.AssetEvent
		var oldValue, newValue, createdAt sql.NullString
		if err := rows.Scan(&e.EventID, &e.AssetID, &e.Type, &e.Actor,
			&oldValue, &newValue, &e.Note, &createdAt); err != nil {
			return nil, err
		}
		if e.CreatedAt, err = timeFromText(createdAt.String); err != nil {
			return nil, err
		}
		if e.OldValue, err = jsonFromNull(oldValue); err != nil {
			return nil, err
		}
		if e.NewValue, err = jsonFromNull(newValue); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// Create 写入履历（JSONB 在 SQLite 形态为 TEXT，手动序列化）。
func (r *EventRepo) Create(ctx context.Context, e *domain.AssetEvent) error {
	oldJSON, err := jsonToNull(e.OldValue)
	if err != nil {
		return err
	}
	newJSON, err := jsonToNull(e.NewValue)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO asset_events (asset_id, type, actor, old_value, new_value, note)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		e.AssetID, e.Type, e.Actor, oldJSON, newJSON, e.Note)
	return err
}

// jsonFromNull TEXT → map（空值为 nil map）。
func jsonFromNull(s sql.NullString) (map[string]any, error) {
	if !s.Valid || s.String == "" {
		return nil, nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s.String), &m); err != nil {
		return nil, err
	}
	return m, nil
}

// jsonToNull map → TEXT（nil 为 NULL）。
func jsonToNull(m map[string]any) (any, error) {
	if m == nil {
		return nil, nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}
