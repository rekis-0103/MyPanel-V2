package main

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *store) putMetric(ctx context.Context, serverID string, state agentState) error {
	bucket := time.Now().UTC().Truncate(time.Minute)
	var players, playersMax, latency any
	if state.State == "running" {
		players, playersMax, latency = state.Players, state.PlayersMax, state.LatencyMS
	}
	_, err := s.db.Exec(ctx, `INSERT INTO server_metric_samples
(server_id,bucket_at,resolution,state,cpu_percent,memory_bytes,disk_bytes,players_online,players_max,latency_ms)
VALUES ($1,$2,'1m',$3,$4,$5,$6,$7,$8,$9)
ON CONFLICT (server_id,resolution,bucket_at) DO UPDATE SET state=EXCLUDED.state,
cpu_percent=EXCLUDED.cpu_percent,memory_bytes=EXCLUDED.memory_bytes,disk_bytes=EXCLUDED.disk_bytes,
players_online=EXCLUDED.players_online,players_max=EXCLUDED.players_max,latency_ms=EXCLUDED.latency_ms`,
		serverID, bucket, state.State, state.CPUPercent, state.MemoryBytes, state.DiskBytes, players, playersMax, latency)
	return err
}

func (s *store) maintainMetrics(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `INSERT INTO server_metric_samples
(server_id,bucket_at,resolution,state,cpu_percent,memory_bytes,disk_bytes,players_online,players_max,latency_ms)
SELECT server_id,date_bin('15 minutes',bucket_at,TIMESTAMPTZ '2001-01-01'),'15m',(array_agg(state ORDER BY bucket_at DESC))[1],
avg(cpu_percent),avg(memory_bytes)::bigint,max(disk_bytes),avg(players_online)::int,max(players_max),avg(latency_ms)::int
FROM server_metric_samples WHERE resolution='1m' AND bucket_at>=now()-interval '30 minutes'
GROUP BY server_id,date_bin('15 minutes',bucket_at,TIMESTAMPTZ '2001-01-01')
ON CONFLICT (server_id,resolution,bucket_at) DO UPDATE SET state=EXCLUDED.state,cpu_percent=EXCLUDED.cpu_percent,
memory_bytes=EXCLUDED.memory_bytes,disk_bytes=EXCLUDED.disk_bytes,players_online=EXCLUDED.players_online,
players_max=EXCLUDED.players_max,latency_ms=EXCLUDED.latency_ms`)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `DELETE FROM server_metric_samples
WHERE (resolution='1m' AND bucket_at<now()-interval '24 hours')
   OR (resolution='15m' AND bucket_at<now()-interval '7 days')`)
	return err
}

func (s *store) metricHistory(ctx context.Context, serverID, resolution string, since time.Time) ([]metricSample, error) {
	rows, err := s.db.Query(ctx, `SELECT server_id,bucket_at,resolution,state,cpu_percent,memory_bytes,
disk_bytes,players_online,players_max,latency_ms FROM server_metric_samples
WHERE server_id=$1 AND resolution=$2 AND bucket_at>=$3 ORDER BY bucket_at`, serverID, resolution, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]metricSample, 0)
	for rows.Next() {
		var item metricSample
		if err := rows.Scan(&item.ServerID, &item.BucketAt, &item.Resolution, &item.State, &item.CPUPercent,
			&item.MemoryBytes, &item.DiskBytes, &item.Players, &item.PlayersMax, &item.LatencyMS); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *store) notificationRecipients(ctx context.Context, serverID string) ([]string, error) {
	rows, err := s.db.Query(ctx, `SELECT id FROM users WHERE role='owner' AND status='active'
UNION SELECT owner_user_id FROM servers WHERE id=$1 AND deleted_at IS NULL`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0, 2)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *store) setAlert(ctx context.Context, serverID, kind, severity, title, message string, active bool) error {
	recipients, err := s.notificationRecipients(ctx, serverID)
	if err != nil {
		return err
	}
	key := serverID + ":" + kind
	for _, userID := range recipients {
		if active {
			_, err = s.db.Exec(ctx, `INSERT INTO notifications
(id,user_id,server_id,severity,kind,title,message,dedupe_key)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT (user_id,dedupe_key) WHERE active DO UPDATE SET severity=EXCLUDED.severity,
title=EXCLUDED.title,message=EXCLUDED.message`, uuid.NewString(), userID, serverID, severity, kind, title, message, key)
		} else {
			_, err = s.db.Exec(ctx, `UPDATE notifications SET active=false,resolved_at=now()
WHERE user_id=$1 AND dedupe_key=$2 AND active`, userID, key)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *store) listNotifications(ctx context.Context, userID string, limit int) ([]notification, error) {
	rows, err := s.db.Query(ctx, `SELECT id,server_id,severity,kind,title,message,active,read_at,created_at,resolved_at
FROM notifications WHERE user_id=$1 ORDER BY active DESC,created_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]notification, 0)
	for rows.Next() {
		var item notification
		if err := rows.Scan(&item.ID, &item.ServerID, &item.Severity, &item.Kind, &item.Title, &item.Message, &item.Active, &item.ReadAt, &item.CreatedAt, &item.ResolvedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *store) readNotification(ctx context.Context, userID, id string) error {
	var result pgx.Row = s.db.QueryRow(ctx, `UPDATE notifications SET read_at=COALESCE(read_at,now())
WHERE user_id=$1 AND id=$2 RETURNING id`, userID, id)
	var found string
	return result.Scan(&found)
}

func (s *store) readAllNotifications(ctx context.Context, userID string) error {
	_, err := s.db.Exec(ctx, `UPDATE notifications SET read_at=COALESCE(read_at,now()) WHERE user_id=$1`, userID)
	return err
}
