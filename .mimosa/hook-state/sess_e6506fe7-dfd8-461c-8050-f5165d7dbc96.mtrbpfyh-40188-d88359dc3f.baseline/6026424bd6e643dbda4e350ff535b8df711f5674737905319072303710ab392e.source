package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	opsUserErrorLimit     = 20
	opsUserErrorTypeLimit = 6
)

func (r *opsRepository) GetUserErrorDistribution(ctx context.Context, filter *service.OpsDashboardFilter) (*service.OpsUserErrorDistributionResponse, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	if filter == nil {
		return nil, fmt.Errorf("nil filter")
	}
	if filter.StartTime.IsZero() || filter.EndTime.IsZero() {
		return nil, fmt.Errorf("start_time/end_time required")
	}

	start := filter.StartTime.UTC()
	end := filter.EndTime.UTC()
	where, args, _ := buildErrorWhere(filter, start, end, 1)

	query := fmt.Sprintf(`
WITH filtered AS (
  SELECT
    user_id,
    COALESCE(NULLIF(BTRIM(error_type), ''), 'unknown') AS error_type
  FROM ops_error_logs
  %s
    AND NOT is_business_limited
    AND (COALESCE(status_code, 0) >= 400 OR error_type = 'cyber_policy')
), user_totals AS (
  SELECT user_id, COUNT(*) AS total
  FROM filtered
  GROUP BY user_id
), ranked_users AS (
  SELECT
    user_id,
    total,
    COUNT(*) OVER () AS total_users,
    SUM(total) OVER () AS overall_total
  FROM user_totals
  ORDER BY total DESC, user_id ASC NULLS LAST
  LIMIT %d
), top_types AS (
  SELECT f.error_type, COUNT(*) AS total
  FROM filtered f
  JOIN ranked_users ru ON f.user_id IS NOT DISTINCT FROM ru.user_id
  GROUP BY f.error_type
  ORDER BY total DESC, f.error_type ASC
  LIMIT %d
), bucketed AS (
  SELECT
    f.user_id,
    CASE WHEN tt.error_type IS NULL THEN 'other' ELSE f.error_type END AS error_type,
    COUNT(*) AS count
  FROM filtered f
  JOIN ranked_users ru ON f.user_id IS NOT DISTINCT FROM ru.user_id
  LEFT JOIN top_types tt ON tt.error_type = f.error_type
  GROUP BY f.user_id, CASE WHEN tt.error_type IS NULL THEN 'other' ELSE f.error_type END
)
SELECT
  ru.user_id,
  COALESCE(u.username, '') AS username,
  COALESCE(u.email, '') AS email,
  COALESCE(u.deleted_at IS NOT NULL, FALSE) AS deleted,
  ru.total,
  b.error_type,
  b.count,
  ru.total_users,
  ru.overall_total
FROM ranked_users ru
LEFT JOIN users u ON u.id = ru.user_id
JOIN bucketed b ON b.user_id IS NOT DISTINCT FROM ru.user_id
ORDER BY ru.total DESC, ru.user_id ASC NULLS LAST, b.count DESC, b.error_type ASC`, where, opsUserErrorLimit, opsUserErrorTypeLimit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	response := &service.OpsUserErrorDistributionResponse{
		UserLimit: opsUserErrorLimit,
		Items:     []*service.OpsUserErrorDistributionItem{},
	}
	var current *service.OpsUserErrorDistributionItem
	var currentKey string
	for rows.Next() {
		var userID sql.NullInt64
		var username, email, errorType string
		var deleted bool
		var userTotal, errorCount, overallTotal int64
		var totalUsers int
		if err := rows.Scan(
			&userID,
			&username,
			&email,
			&deleted,
			&userTotal,
			&errorType,
			&errorCount,
			&totalUsers,
			&overallTotal,
		); err != nil {
			return nil, err
		}

		key := "unknown"
		var id *int64
		if userID.Valid {
			value := userID.Int64
			id = &value
			key = fmt.Sprintf("user:%d", value)
		}
		if current == nil || currentKey != key {
			current = &service.OpsUserErrorDistributionItem{
				UserID:   id,
				Username: username,
				Email:    email,
				Deleted:  deleted,
				Total:    userTotal,
				Errors:   []*service.OpsUserErrorTypeCount{},
			}
			response.Items = append(response.Items, current)
			currentKey = key
		}
		current.Errors = append(current.Errors, &service.OpsUserErrorTypeCount{ErrorType: errorType, Count: errorCount})
		response.TotalUsers = totalUsers
		response.Total = overallTotal
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return response, nil
}
