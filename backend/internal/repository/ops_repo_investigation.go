package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// GetInvestigationErrorGroups returns bounded, non-sensitive error aggregates.
// A group is deliberately kept to routing dimensions that can be acted on from
// the dashboard; request contents and error messages are not selected.
func (r *opsRepository) GetInvestigationErrorGroups(ctx context.Context, filter *service.OpsDashboardFilter) ([]*service.OpsInvestigationErrorGroup, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	if filter == nil || filter.StartTime.IsZero() || filter.EndTime.IsZero() {
		return nil, fmt.Errorf("start_time/end_time required")
	}

	start := filter.StartTime.UTC()
	end := filter.EndTime.UTC()
	where, args, _ := buildErrorWhere(filter, start, end, 1)
	q := `
SELECT
  COALESCE(error_phase, ''),
  COALESCE(error_owner, ''),
  COALESCE(error_type, ''),
  COALESCE(upstream_status_code, status_code, 0),
  COALESCE(platform, ''),
  group_id,
  COUNT(*),
  SUM(COUNT(*)) OVER ()
FROM ops_error_logs
` + where + `
  AND COALESCE(status_code, 0) >= 400
GROUP BY 1, 2, 3, 4, 5, 6
ORDER BY COUNT(*) DESC
LIMIT 200`

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	groups := make([]*service.OpsInvestigationErrorGroup, 0, 32)
	for rows.Next() {
		item := &service.OpsInvestigationErrorGroup{}
		var groupID sql.NullInt64
		if err := rows.Scan(&item.Phase, &item.Owner, &item.Type, &item.StatusCode, &item.Platform, &groupID, &item.Count, &item.Total); err != nil {
			return nil, err
		}
		if groupID.Valid {
			value := groupID.Int64
			item.GroupID = &value
		}
		groups = append(groups, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return groups, nil
}
