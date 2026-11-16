package repo

import (
	"context"
	"database/sql"

	"mutag/backend/internal/domain"
)

// UserRepo 微信用户数据访问。
type UserRepo struct {
	db *sql.DB
}

// GetOrCreate openid 幂等建档（首登默认 staff）。
func (r *UserRepo) GetOrCreate(ctx context.Context, openid, nick string) (*domain.User, error) {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO users (openid, nickname, created_at) VALUES (?, NULLIF(?, ''), ?)
		 ON CONFLICT(openid) DO NOTHING`, openid, nick, nowText())
	if err != nil {
		return nil, err
	}
	u := &domain.User{}
	var nickname, createdAt sql.NullString
	err = r.db.QueryRowContext(ctx,
		`SELECT openid, nickname, role, created_at FROM users WHERE openid = ?`, openid).
		Scan(&u.OpenID, &nickname, &u.Role, &createdAt)
	if err != nil {
		return nil, mapErr(err)
	}
	u.Nickname = nickname.String
	u.CreatedAt, err = timeFromText(createdAt.String)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// ListEventsByActor 依操作人查履历（我的上报，新→旧）。
func (r *UserRepo) ListEventsByActor(ctx context.Context, actor string, limit int) ([]domain.AssetEvent, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT event_id, asset_id, type, actor, old_value, new_value, note, created_at
		 FROM asset_events WHERE actor = ? ORDER BY created_at DESC LIMIT ?`,
		actor, limit)
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
