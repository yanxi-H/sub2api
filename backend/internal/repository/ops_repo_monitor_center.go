package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *opsRepository) UpsertMonitorCenterOpenAIHistory(ctx context.Context, point *service.MonitorCenterOpenAIHistoryPoint) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("nil ops repository")
	}
	if point == nil {
		return fmt.Errorf("nil monitor center history point")
	}
	bucket := point.Timestamp.UTC().Truncate(time.Minute)
	_, err := r.db.ExecContext(ctx, `
INSERT INTO monitor_center_openai_history (
  bucket_start, overall_status, api_status, chatgpt_status, codex_status,
  active_incident_count, fetch_status, latency_ms
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT (bucket_start) DO UPDATE SET
  overall_status = EXCLUDED.overall_status,
  api_status = EXCLUDED.api_status,
  chatgpt_status = EXCLUDED.chatgpt_status,
  codex_status = EXCLUDED.codex_status,
  active_incident_count = EXCLUDED.active_incident_count,
  fetch_status = EXCLUDED.fetch_status,
  latency_ms = EXCLUDED.latency_ms`,
		bucket,
		point.OverallStatus,
		point.APIStatus,
		point.ChatGPTStatus,
		point.CodexStatus,
		point.ActiveIncidentCount,
		point.FetchStatus,
		point.LatencyMs,
	)
	return err
}

func (r *opsRepository) InsertMonitorCenterOpenAIEvent(
	ctx context.Context,
	observedAt time.Time,
	contentHash string,
	status *service.MonitorCenterOpenAIStatus,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("nil ops repository")
	}
	payload, err := json.Marshal(status)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
INSERT INTO monitor_center_openai_events (observed_at, content_hash, normalized_state)
VALUES ($1,$2,$3::jsonb)`, observedAt.UTC(), contentHash, string(payload))
	return err
}

func (r *opsRepository) LoadLatestMonitorCenterOpenAIEventHash(ctx context.Context) (string, error) {
	if r == nil || r.db == nil {
		return "", fmt.Errorf("nil ops repository")
	}
	var hash string
	err := r.db.QueryRowContext(ctx, `
SELECT content_hash
FROM monitor_center_openai_events
ORDER BY observed_at DESC, id DESC
LIMIT 1`).Scan(&hash)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return hash, nil
}

func (r *opsRepository) UpsertMonitorCenterIncidentUpdates(
	ctx context.Context,
	incidents []service.MonitorCenterIncident,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("nil ops repository")
	}
	for _, incident := range incidents {
		for _, update := range incident.Updates {
			_, err := r.db.ExecContext(ctx, `
INSERT INTO monitor_center_openai_incident_updates (
  incident_id, incident_name, incident_status, impact, update_status, update_body, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7)
ON CONFLICT (incident_id, updated_at) DO NOTHING`,
				incident.ID,
				incident.Name,
				incident.Status,
				incident.Impact,
				update.Status,
				update.Body,
				update.UpdatedAt.UTC(),
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *opsRepository) ListMonitorCenterOpenAIHistory(
	ctx context.Context,
	start, end time.Time,
) ([]service.MonitorCenterOpenAIHistoryPoint, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT bucket_start, overall_status, api_status, chatgpt_status, codex_status,
       active_incident_count, fetch_status, latency_ms
FROM monitor_center_openai_history
WHERE bucket_start >= $1 AND bucket_start <= $2
ORDER BY bucket_start ASC`, start.UTC(), end.UTC())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	points := make([]service.MonitorCenterOpenAIHistoryPoint, 0, 256)
	for rows.Next() {
		var point service.MonitorCenterOpenAIHistoryPoint
		if err := rows.Scan(
			&point.Timestamp,
			&point.OverallStatus,
			&point.APIStatus,
			&point.ChatGPTStatus,
			&point.CodexStatus,
			&point.ActiveIncidentCount,
			&point.FetchStatus,
			&point.LatencyMs,
		); err != nil {
			return nil, err
		}
		point.Timestamp = point.Timestamp.UTC()
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return points, nil
}

func (r *opsRepository) DeleteMonitorCenterOpenAIBefore(ctx context.Context, before time.Time) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("nil ops repository")
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM monitor_center_openai_history WHERE bucket_start < $1`, before.UTC()); err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM monitor_center_openai_events WHERE observed_at < $1`, before.UTC()); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM monitor_center_openai_incident_updates WHERE updated_at < $1`, before.UTC())
	return err
}
