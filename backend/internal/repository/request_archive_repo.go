package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// requestArchiveRepository 请求存档仓储(raw SQL,独立于 ent)。
type requestArchiveRepository struct {
	db *sql.DB
}

// NewRequestArchiveRepository 创建请求存档仓储。
func NewRequestArchiveRepository(db *sql.DB) service.RequestArchiveRepository {
	return &requestArchiveRepository{db: db}
}

func (r *requestArchiveRepository) BatchInsert(ctx context.Context, entries []service.RequestArchiveEntry) error {
	if len(entries) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, e := range entries {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO request_archive_logs
				(request_id, user_id, user_email, api_key_id, api_key_name, group_id, endpoint, protocol, model, ip_address, prompt_text, truncated)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`, e.RequestID, e.UserID, e.UserEmail, e.APIKeyID, e.APIKeyName, e.GroupID, e.Endpoint, e.Protocol, e.Model, e.IPAddress, e.PromptText, e.Truncated)
		if err != nil {
			return fmt.Errorf("insert request archive: %w", err)
		}
	}
	return tx.Commit()
}

func (r *requestArchiveRepository) List(ctx context.Context, page, pageSize int, search string, userID, apiKeyID *int64, startDate, endDate *time.Time) ([]service.RequestArchiveEntry, int64, error) {
	where := "WHERE 1=1"
	args := []any{}
	argIdx := 1

	if search != "" {
		where += fmt.Sprintf(" AND to_tsvector('simple', prompt_text) @@ plainto_tsquery('simple', $%d)", argIdx)
		args = append(args, search)
		argIdx++
	}
	if userID != nil {
		where += fmt.Sprintf(" AND user_id = $%d", argIdx)
		args = append(args, *userID)
		argIdx++
	}
	if apiKeyID != nil {
		where += fmt.Sprintf(" AND api_key_id = $%d", argIdx)
		args = append(args, *apiKeyID)
		argIdx++
	}
	if startDate != nil {
		where += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, *startDate)
		argIdx++
	}
	if endDate != nil {
		where += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, *endDate)
		argIdx++
	}

	// count
	var total int64
	countQuery := "SELECT COUNT(*) FROM request_archive_logs " + where
	countArgs := append([]any(nil), args...)
	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count request archive: %w", err)
	}

	if total == 0 {
		return []service.RequestArchiveEntry{}, 0, nil
	}

	// 分页查询(倒序,最新的在前)
	params := pagination.PaginationParams{Page: page, PageSize: pageSize}
	listQuery := "SELECT id, created_at, request_id, user_id, COALESCE(user_email,''), api_key_id, COALESCE(api_key_name,''), group_id, endpoint, protocol, model, ip_address, LEFT(prompt_text, 500) AS prompt_preview, truncated FROM request_archive_logs " + where + " ORDER BY created_at DESC"
	listQuery += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	listArgs := append(args, params.Limit(), params.Offset())

	rows, err := r.db.QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list request archive: %w", err)
	}
	defer rows.Close()

	entries := make([]service.RequestArchiveEntry, 0, params.Limit())
	for rows.Next() {
		var e service.RequestArchiveEntry
		var groupID *int64
		if err := rows.Scan(&e.ID, &e.CreatedAt, &e.RequestID, &e.UserID, &e.UserEmail, &e.APIKeyID, &e.APIKeyName, &groupID, &e.Endpoint, &e.Protocol, &e.Model, &e.IPAddress, &e.PromptText, &e.Truncated); err != nil {
			return nil, 0, fmt.Errorf("scan request archive: %w", err)
		}
		e.GroupID = groupID
		entries = append(entries, e)
	}
	return entries, total, rows.Err()
}

func (r *requestArchiveRepository) GetByID(ctx context.Context, id int64) (*service.RequestArchiveEntry, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, created_at, request_id, user_id, COALESCE(user_email,''), api_key_id, COALESCE(api_key_name,''), group_id, endpoint, protocol, model, ip_address, prompt_text, truncated
		FROM request_archive_logs WHERE id = $1
	`, id)
	var e service.RequestArchiveEntry
	var groupID *int64
	if err := row.Scan(&e.ID, &e.CreatedAt, &e.RequestID, &e.UserID, &e.UserEmail, &e.APIKeyID, &e.APIKeyName, &groupID, &e.Endpoint, &e.Protocol, &e.Model, &e.IPAddress, &e.PromptText, &e.Truncated); err != nil {
		return nil, fmt.Errorf("get request archive by id: %w", err)
	}
	e.GroupID = groupID
	return &e, nil
}

func (r *requestArchiveRepository) CleanupOlderThan(ctx context.Context, before time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, "DELETE FROM request_archive_logs WHERE created_at < $1", before)
	if err != nil {
		return 0, fmt.Errorf("cleanup request archive: %w", err)
	}
	deleted, _ := res.RowsAffected()
	return deleted, nil
}

// 编译期断言:确保实现了接口
var _ service.RequestArchiveRepository = (*requestArchiveRepository)(nil)
