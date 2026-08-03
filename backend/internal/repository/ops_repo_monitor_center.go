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
  active_incident_count, fetch_status, latency_ms, failure_reason, incident_refs
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb)
ON CONFLICT (bucket_start) DO UPDATE SET
  overall_status = EXCLUDED.overall_status,
  api_status = EXCLUDED.api_status,
  chatgpt_status = EXCLUDED.chatgpt_status,
  codex_status = EXCLUDED.codex_status,
  active_incident_count = EXCLUDED.active_incident_count,
  fetch_status = EXCLUDED.fetch_status,
  latency_ms = EXCLUDED.latency_ms,
  failure_reason = EXCLUDED.failure_reason,
  incident_refs = EXCLUDED.incident_refs`,
		bucket,
		point.OverallStatus,
		point.APIStatus,
		point.ChatGPTStatus,
		point.CodexStatus,
		point.ActiveIncidentCount,
		point.FetchStatus,
		point.LatencyMs,
		point.FailureReason,
		marshalMonitorCenterJSON(point.IncidentRefs, `{}`),
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
  incident_id, incident_name, incident_status, impact, update_status, update_body, updated_at,
  affected_components, affected_groups, incident_url
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9::jsonb,$10)
ON CONFLICT (incident_id, updated_at) DO UPDATE SET
  incident_name = EXCLUDED.incident_name,
  incident_status = EXCLUDED.incident_status,
  impact = EXCLUDED.impact,
  update_status = EXCLUDED.update_status,
  update_body = EXCLUDED.update_body,
  affected_components = EXCLUDED.affected_components,
  affected_groups = EXCLUDED.affected_groups,
  incident_url = EXCLUDED.incident_url`,
				incident.ID,
				incident.Name,
				incident.Status,
				incident.Impact,
				update.Status,
				update.Body,
				update.UpdatedAt.UTC(),
				marshalMonitorCenterJSON(incident.AffectedComponents, `[]`),
				marshalMonitorCenterJSON(incident.AffectedGroups, `[]`),
				incident.URL,
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *opsRepository) UpsertMonitorCenterIncidents(
	ctx context.Context,
	observedAt time.Time,
	incidents []service.MonitorCenterIncident,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("nil ops repository")
	}
	for _, incident := range incidents {
		_, err := r.db.ExecContext(ctx, `
INSERT INTO monitor_center_openai_incidents (
  incident_id, incident_name, incident_status, impact, affected_components, affected_groups,
  started_at, created_at, updated_at, resolved_at, last_seen_at, incident_url
) VALUES ($1,$2,$3,$4,$5::jsonb,$6::jsonb,$7,$8,$9,$10,$11,$12)
ON CONFLICT (incident_id) DO UPDATE SET
  incident_name = EXCLUDED.incident_name,
  incident_status = EXCLUDED.incident_status,
  impact = EXCLUDED.impact,
  affected_components = EXCLUDED.affected_components,
  affected_groups = EXCLUDED.affected_groups,
  started_at = COALESCE(EXCLUDED.started_at, monitor_center_openai_incidents.started_at),
  created_at = COALESCE(EXCLUDED.created_at, monitor_center_openai_incidents.created_at),
  updated_at = EXCLUDED.updated_at,
  resolved_at = CASE
    WHEN EXCLUDED.incident_status = 'resolved' THEN COALESCE(EXCLUDED.resolved_at, monitor_center_openai_incidents.resolved_at)
    ELSE NULL
  END,
  last_seen_at = EXCLUDED.last_seen_at,
  incident_url = EXCLUDED.incident_url`,
			incident.ID,
			incident.Name,
			incident.Status,
			incident.Impact,
			marshalMonitorCenterJSON(incident.AffectedComponents, `[]`),
			marshalMonitorCenterJSON(incident.AffectedGroups, `[]`),
			incident.StartedAt,
			incident.CreatedAt,
			incident.UpdatedAt.UTC(),
			incident.ResolvedAt,
			observedAt.UTC(),
			incident.URL,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *opsRepository) ResolveMissingMonitorCenterIncidents(
	ctx context.Context,
	observedAt time.Time,
	activeIncidentIDs []string,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("nil ops repository")
	}
	_, err := r.db.ExecContext(ctx, `
UPDATE monitor_center_openai_incidents AS incident
SET incident_status = 'resolved',
    resolved_at = COALESCE(incident.resolved_at, $1),
    updated_at = GREATEST(incident.updated_at, $1)
WHERE incident.incident_status <> 'resolved'
  AND incident.last_seen_at < $1
  AND NOT EXISTS (
    SELECT 1 FROM jsonb_array_elements_text($2::jsonb) AS active(id)
    WHERE active.id = incident.incident_id
  )`, observedAt.UTC(), marshalMonitorCenterJSON(activeIncidentIDs, `[]`))
	return err
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
       active_incident_count, fetch_status, latency_ms, failure_reason, incident_refs
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
		var incidentRefs []byte
		if err := rows.Scan(
			&point.Timestamp,
			&point.OverallStatus,
			&point.APIStatus,
			&point.ChatGPTStatus,
			&point.CodexStatus,
			&point.ActiveIncidentCount,
			&point.FetchStatus,
			&point.LatencyMs,
			&point.FailureReason,
			&incidentRefs,
		); err != nil {
			return nil, err
		}
		point.Timestamp = point.Timestamp.UTC()
		if len(incidentRefs) > 0 {
			_ = json.Unmarshal(incidentRefs, &point.IncidentRefs)
		}
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return points, nil
}

func (r *opsRepository) ListMonitorCenterIncidents(
	ctx context.Context,
	start, end time.Time,
) ([]service.MonitorCenterIncident, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT incident_id, incident_name, incident_status, impact, affected_components, affected_groups,
       started_at, created_at, updated_at, resolved_at, incident_url
FROM monitor_center_openai_incidents
WHERE COALESCE(started_at, created_at, updated_at) <= $2
  AND COALESCE(resolved_at, last_seen_at) >= $1
ORDER BY COALESCE(started_at, created_at, updated_at) DESC`, start.UTC(), end.UTC())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	incidents := make([]service.MonitorCenterIncident, 0, 8)
	for rows.Next() {
		var incident service.MonitorCenterIncident
		var components, groups []byte
		if err := rows.Scan(
			&incident.ID, &incident.Name, &incident.Status, &incident.Impact, &components, &groups,
			&incident.StartedAt, &incident.CreatedAt, &incident.UpdatedAt, &incident.ResolvedAt, &incident.URL,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(components, &incident.AffectedComponents)
		_ = json.Unmarshal(groups, &incident.AffectedGroups)
		incident.Updates = []service.MonitorCenterIncidentUpdate{}
		incidents = append(incidents, incident)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(incidents) == 0 {
		return incidents, nil
	}
	updates, err := r.db.QueryContext(ctx, `
SELECT incident_id, update_status, update_body, updated_at
FROM monitor_center_openai_incident_updates
WHERE incident_id IN (
  SELECT incident_id FROM monitor_center_openai_incidents
  WHERE COALESCE(started_at, created_at, updated_at) <= $2
    AND COALESCE(resolved_at, last_seen_at) >= $1
)
ORDER BY updated_at DESC`, start.UTC(), end.UTC())
	if err != nil {
		return nil, err
	}
	defer func() { _ = updates.Close() }()
	index := make(map[string]int, len(incidents))
	for i := range incidents {
		index[incidents[i].ID] = i
	}
	for updates.Next() {
		var incidentID string
		var update service.MonitorCenterIncidentUpdate
		if err := updates.Scan(&incidentID, &update.Status, &update.Body, &update.UpdatedAt); err != nil {
			return nil, err
		}
		if i, ok := index[incidentID]; ok {
			incidents[i].Updates = append(incidents[i].Updates, update)
			if incidents[i].LatestUpdate == nil {
				latest := update
				incidents[i].LatestUpdate = &latest
			}
		}
	}
	if err := updates.Err(); err != nil {
		return nil, err
	}
	return incidents, nil
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
	if _, err := r.db.ExecContext(ctx, `DELETE FROM monitor_center_openai_incidents WHERE last_seen_at < $1`, before.UTC()); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM monitor_center_openai_incident_updates WHERE updated_at < $1`, before.UTC())
	return err
}

func marshalMonitorCenterJSON(value any, fallback string) string {
	payload, err := json.Marshal(value)
	if err != nil || string(payload) == "null" {
		return fallback
	}
	return string(payload)
}
