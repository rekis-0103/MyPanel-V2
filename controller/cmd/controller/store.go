package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type store struct{ db *pgxpool.Pool }

const consoleEventRetention = 200

func openStore(ctx context.Context, databaseURL string) (*store, error) {
	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return &store{db: db}, nil
}

const serverColumns = `s.id, s.node_id, s.name, s.runtime, s.version, s.java_version, s.memory_mb,
 s.cpu, s.disk_mb, s.bind_ip, s.port, s.desired_state, s.observed_state,
 s.config, s.last_error, s.created_at, s.updated_at, s.owner_user_id,
 COALESCE((SELECT username FROM users u WHERE u.id=s.owner_user_id),''),
 (SELECT status FROM subscriptions sub WHERE sub.server_id=s.id),
 (SELECT current_period_end FROM subscriptions sub WHERE sub.server_id=s.id)`

type rowScanner interface{ Scan(...any) error }

func scanServer(row rowScanner) (server, error) {
	var item server
	err := row.Scan(&item.ID, &item.NodeID, &item.Name, &item.Runtime, &item.Version, &item.JavaVersion,
		&item.MemoryMB, &item.CPU, &item.DiskMB, &item.BindIP, &item.Port,
		&item.DesiredState, &item.State, &item.Config, &item.LastError,
		&item.CreatedAt, &item.UpdatedAt, &item.OwnerUserID, &item.OwnerUsername,
		&item.SubscriptionStatus, &item.SubscriptionEndsAt)
	return item, err
}

func (s *store) list(ctx context.Context) ([]server, error) {
	rows, err := s.db.Query(ctx, `SELECT `+serverColumns+`
FROM servers s WHERE s.deleted_at IS NULL ORDER BY s.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]server, 0)
	for rows.Next() {
		item, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		active, err := s.activeJob(ctx, item.ID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		item.CurrentJob = active
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) listForSession(ctx context.Context, session sessionRecord) ([]server, error) {
	if session.Role == "owner" {
		return s.list(ctx)
	}
	rows, err := s.db.Query(ctx, `SELECT `+serverColumns+`
FROM servers s WHERE s.deleted_at IS NULL AND s.owner_user_id=$1 ORDER BY s.created_at DESC`, session.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]server, 0)
	for rows.Next() {
		item, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		active, err := s.activeJob(ctx, item.ID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		item.CurrentJob = active
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) get(ctx context.Context, id string) (server, error) {
	item, err := scanServer(s.db.QueryRow(ctx, `SELECT `+serverColumns+`
FROM servers s WHERE s.id=$1 AND s.deleted_at IS NULL`, id))
	if err != nil {
		return server{}, err
	}
	active, err := s.activeJob(ctx, id)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return server{}, err
	}
	item.CurrentJob = active
	return item, nil
}

func (s *store) getForSession(ctx context.Context, id string, session sessionRecord) (server, error) {
	item, err := s.get(ctx, id)
	if err != nil {
		return server{}, err
	}
	if session.Role != "owner" && item.OwnerUserID != session.UserID {
		return server{}, pgx.ErrNoRows
	}
	return item, nil
}

func (s *store) create(ctx context.Context, in createServerInput, cfg config, ownerID string) (server, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return server{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(74201)`); err != nil {
		return server{}, err
	}
	var memoryUsed, cpuUsed, diskUsed int
	if err := tx.QueryRow(ctx, `SELECT
COALESCE(sum(CASE WHEN subscriptions.id IS NULL OR subscriptions.status IN ('provisioning','active','action_required','grace') THEN servers.memory_mb ELSE 0 END),0),
COALESCE(sum(CASE WHEN subscriptions.id IS NULL OR subscriptions.status IN ('provisioning','active','action_required','grace') THEN servers.cpu ELSE 0 END),0),
COALESCE(sum(servers.disk_mb),0)
FROM servers LEFT JOIN subscriptions ON subscriptions.server_id=servers.id
WHERE servers.deleted_at IS NULL`).Scan(&memoryUsed, &cpuUsed, &diskUsed); err != nil {
		return server{}, err
	}
	if memoryUsed+in.MemoryMB > cfg.NodeMemoryMB || cpuUsed+in.CPU > cfg.NodeCPUs || diskUsed+in.DiskMB > cfg.NodeDiskMB {
		return server{}, errCapacity
	}
	var port int
	if err := tx.QueryRow(ctx, `SELECT candidate FROM generate_series($1::integer,$2::integer) candidate
WHERE NOT EXISTS (SELECT 1 FROM allocations WHERE node_id=$3 AND bind_ip=$4 AND port=candidate)
ORDER BY candidate LIMIT 1`, cfg.PortStart, cfg.PortEnd, defaultNodeID, cfg.BindIP).Scan(&port); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return server{}, errNoAllocation
		}
		return server{}, err
	}
	item := server{
		ID: uuid.NewString(), NodeID: defaultNodeID, Name: in.Name, Runtime: in.Runtime,
		Version: in.Version, JavaVersion: in.JavaVersion, MemoryMB: in.MemoryMB, CPU: in.CPU, DiskMB: in.DiskMB,
		BindIP: cfg.BindIP, Port: port, DesiredState: "offline", State: "installing", OwnerUserID: ownerID,
		Config: json.RawMessage(`{}`),
	}
	err = tx.QueryRow(ctx, `INSERT INTO servers
(id,node_id,name,runtime,version,java_version,memory_mb,cpu,disk_mb,bind_ip,port,desired_state,observed_state,owner_user_id)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'offline','installing',$12)
RETURNING created_at,updated_at`, item.ID, item.NodeID, item.Name, item.Runtime,
		item.Version, item.JavaVersion, item.MemoryMB, item.CPU, item.DiskMB, item.BindIP, item.Port, ownerID).
		Scan(&item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return server{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO allocations (id,node_id,server_id,bind_ip,port)
VALUES ($1,$2,$3,$4,$5)`, uuid.NewString(), item.NodeID, item.ID, item.BindIP, item.Port); err != nil {
		return server{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return server{}, err
	}
	return item, nil
}

func (s *store) setDesiredState(ctx context.Context, id, state string) error {
	ct, err := s.db.Exec(ctx, `UPDATE servers SET desired_state=$2,updated_at=now()
WHERE id=$1 AND deleted_at IS NULL`, id, state)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *store) setObservedState(ctx context.Context, id, state string, lastError *string) error {
	_, err := s.db.Exec(ctx, `UPDATE servers SET observed_state=$2,last_error=$3,updated_at=now()
WHERE id=$1 AND deleted_at IS NULL`, id, state, lastError)
	return err
}

func (s *store) updateConfig(ctx context.Context, id string, value json.RawMessage) (server, error) {
	_, err := s.db.Exec(ctx, `UPDATE servers SET config=$2,updated_at=now()
WHERE id=$1 AND deleted_at IS NULL`, id, value)
	if err != nil {
		return server{}, err
	}
	return s.get(ctx, id)
}

func (s *store) updateConfigJob(ctx context.Context, id string, value json.RawMessage) (job, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return job{}, err
	}
	defer tx.Rollback(ctx)
	commandTag, err := tx.Exec(ctx, `UPDATE servers SET config=$2,updated_at=now()
WHERE id=$1 AND deleted_at IS NULL`, id, value)
	if err != nil {
		return job{}, err
	}
	if commandTag.RowsAffected() == 0 {
		return job{}, pgx.ErrNoRows
	}
	out, err := insertJob(ctx, tx, id, "update", map[string]any{})
	if err != nil {
		return job{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return job{}, err
	}
	return out, nil
}

func (s *store) finalizeDelete(ctx context.Context, id string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM allocations WHERE server_id=$1`, id); err != nil {
		return err
	}
	ct, err := tx.Exec(ctx, `UPDATE servers SET desired_state='deleted',deleted_at=now(),updated_at=now()
WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return tx.Commit(ctx)
}

func (s *store) createJob(ctx context.Context, serverID, action string, payload any) (job, error) {
	return insertJob(ctx, s.db, serverID, action, payload)
}

type jobQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func insertJob(ctx context.Context, queryer jobQuerier, serverID, action string, payload any) (job, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return job{}, err
	}
	var out job
	err = queryer.QueryRow(ctx, `INSERT INTO jobs (id,node_id,server_id,action,status,payload)
VALUES ($1,$2,$3,$4,'queued',$5)
RETURNING id,node_id,server_id,action,status,payload,result,error,attempts,created_at,started_at,completed_at`,
		uuid.NewString(), defaultNodeID, serverID, action, data).Scan(&out.ID, &out.NodeID,
		&out.ServerID, &out.Action, &out.Status, &out.Payload, &out.Result, &out.Error,
		&out.Attempts, &out.CreatedAt, &out.StartedAt, &out.CompletedAt)
	return out, err
}

func (s *store) createLifecycleJob(ctx context.Context, serverID, action, desired string, payload any) (job, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return job{}, err
	}
	defer tx.Rollback(ctx)
	commandTag, err := tx.Exec(ctx, `UPDATE servers SET desired_state=$2,updated_at=now()
WHERE id=$1 AND deleted_at IS NULL`, serverID, desired)
	if err != nil {
		return job{}, err
	}
	if commandTag.RowsAffected() == 0 {
		return job{}, pgx.ErrNoRows
	}
	out, err := insertJob(ctx, tx, serverID, action, payload)
	if err != nil {
		return job{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return job{}, err
	}
	return out, nil
}

func scanJob(row rowScanner) (job, error) {
	var out job
	err := row.Scan(&out.ID, &out.NodeID, &out.ServerID, &out.Action, &out.Status,
		&out.Payload, &out.Result, &out.Error, &out.Attempts, &out.CreatedAt,
		&out.StartedAt, &out.CompletedAt)
	return out, err
}

const jobColumns = `id,node_id,server_id,action,status,payload,result,error,attempts,created_at,started_at,completed_at`

func (s *store) getJob(ctx context.Context, id string) (job, error) {
	return scanJob(s.db.QueryRow(ctx, `SELECT `+jobColumns+` FROM jobs WHERE id=$1`, id))
}

func (s *store) activeJob(ctx context.Context, serverID string) (*job, error) {
	out, err := scanJob(s.db.QueryRow(ctx, `SELECT `+jobColumns+` FROM jobs
WHERE server_id=$1 AND status IN ('queued','running') ORDER BY created_at LIMIT 1`, serverID))
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *store) claimJob(ctx context.Context) (job, error) {
	return scanJob(s.db.QueryRow(ctx, `WITH next AS (
  SELECT id FROM jobs WHERE status='queued' ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1
)
UPDATE jobs SET status='running',started_at=COALESCE(started_at,now()),updated_at=now(),attempts=attempts+1
WHERE id=(SELECT id FROM next)
RETURNING `+jobColumns))
}

func (s *store) recoverJobs(ctx context.Context) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE jobs SET status='failed',error='operation interrupted too many times',
completed_at=now(),updated_at=now() WHERE status='running' AND attempts >= 3`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE jobs SET status='queued',started_at=NULL,updated_at=now()
WHERE status='running' AND attempts < 3`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *store) finishJob(ctx context.Context, id string, result any, jobErr error) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	status := "completed"
	var message *string
	if jobErr != nil {
		status = "failed"
		v := jobErr.Error()
		if len(v) > 500 {
			v = v[:500]
		}
		message = &v
	}
	_, err = s.db.Exec(ctx, `UPDATE jobs SET status=$2,result=$3,error=$4,completed_at=now(),updated_at=now()
WHERE id=$1`, id, status, data, message)
	return err
}

func (s *store) addConsoleEvent(ctx context.Context, serverID, message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return nil
	}
	characters := []rune(message)
	if len(characters) > 500 {
		message = string(characters[:500])
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO server_console_events (server_id,message) VALUES ($1,$2)`, serverID, message); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM server_console_events WHERE server_id=$1 AND id NOT IN (
  SELECT id FROM server_console_events WHERE server_id=$1 ORDER BY id DESC LIMIT $2
)`, serverID, consoleEventRetention); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *store) consoleEvents(ctx context.Context, serverID string, afterID int64, limit int) ([]consoleEvent, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	query, arguments := consoleEventQuery(serverID, afterID, limit)
	rows, err := s.db.Query(ctx, query, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]consoleEvent, 0)
	for rows.Next() {
		var item consoleEvent
		if err := rows.Scan(&item.ID, &item.Message, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func consoleEventQuery(serverID string, afterID int64, limit int) (string, []any) {
	query := `SELECT id,message,created_at FROM server_console_events
WHERE server_id=$1 AND id>$2 ORDER BY id LIMIT $3`
	arguments := []any{serverID, afterID, limit}
	if afterID == 0 {
		query = `SELECT id,message,created_at FROM (
  SELECT id,message,created_at FROM server_console_events
  WHERE server_id=$1 ORDER BY id DESC LIMIT $2
) recent ORDER BY id`
		arguments = []any{serverID, limit}
	}
	return query, arguments
}

func (s *store) userCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&count)
	return count, err
}

func (s *store) createOwner(ctx context.Context, username, passwordHash string) error {
	_, err := s.db.Exec(ctx, `INSERT INTO users (id,username,password_hash,role)
VALUES ($1,$2,$3,'owner')`, uuid.NewString(), username, passwordHash)
	return err
}

func (s *store) findUser(ctx context.Context, username string) (userRecord, error) {
	var out userRecord
	err := s.db.QueryRow(ctx, `SELECT id,username,password_hash,role,status,session_version,must_change_password
FROM users WHERE lower(username)=lower($1)`, username).
		Scan(&out.ID, &out.Username, &out.PasswordHash, &out.Role, &out.Status, &out.SessionVersion, &out.MustChangePassword)
	return out, err
}

func (s *store) findUserByID(ctx context.Context, id string) (userRecord, error) {
	var out userRecord
	err := s.db.QueryRow(ctx, `SELECT id,username,password_hash,role,status,session_version,must_change_password FROM users WHERE id=$1`, id).
		Scan(&out.ID, &out.Username, &out.PasswordHash, &out.Role, &out.Status, &out.SessionVersion, &out.MustChangePassword)
	return out, err
}

func (s *store) audit(ctx context.Context, userID *string, action, targetType, targetID, ip string, detail any) error {
	data, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `INSERT INTO audit_events
(user_id,action,target_type,target_id,ip,detail) VALUES ($1,$2,$3,$4,$5,$6)`,
		userID, action, nullable(targetType), nullable(targetID), nullable(ip), data)
	return err
}

func (s *store) listAudit(ctx context.Context, limit int) ([]map[string]any, error) {
	rows, err := s.db.Query(ctx, `SELECT a.id,u.username,a.action,a.target_type,a.target_id,a.ip,a.detail,a.created_at
FROM audit_events a LEFT JOIN users u ON u.id=a.user_id ORDER BY a.created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id int64
		var username, targetType, targetID, ip *string
		var action string
		var detail json.RawMessage
		var createdAt time.Time
		if err := rows.Scan(&id, &username, &action, &targetType, &targetID, &ip, &detail, &createdAt); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "username": username, "action": action,
			"targetType": targetType, "targetId": targetID, "ip": ip, "detail": detail, "createdAt": createdAt})
	}
	return out, rows.Err()
}

func (s *store) createBackupJob(ctx context.Context, serverID, name string) (backup, job, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return backup{}, job{}, err
	}
	defer tx.Rollback(ctx)
	item := backup{ID: uuid.NewString(), ServerID: serverID, Name: name, Status: "queued"}
	if err := tx.QueryRow(ctx, `INSERT INTO backups (id,server_id,name,status)
VALUES ($1,$2,$3,'queued') RETURNING created_at`, item.ID, item.ServerID, item.Name).Scan(&item.CreatedAt); err != nil {
		return backup{}, job{}, err
	}
	payload, _ := json.Marshal(map[string]string{"backupId": item.ID})
	var createdJob job
	if err := tx.QueryRow(ctx, `INSERT INTO jobs (id,node_id,server_id,action,status,payload)
VALUES ($1,$2,$3,'backup','queued',$4)
RETURNING `+jobColumns, uuid.NewString(), defaultNodeID, serverID, payload).Scan(&createdJob.ID,
		&createdJob.NodeID, &createdJob.ServerID, &createdJob.Action, &createdJob.Status,
		&createdJob.Payload, &createdJob.Result, &createdJob.Error, &createdJob.Attempts,
		&createdJob.CreatedAt, &createdJob.StartedAt, &createdJob.CompletedAt); err != nil {
		return backup{}, job{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return backup{}, job{}, err
	}
	return item, createdJob, nil
}

func (s *store) listBackups(ctx context.Context, serverID string) ([]backup, error) {
	rows, err := s.db.Query(ctx, `SELECT id,server_id,name,size_bytes,checksum_sha256,status,error,created_at,completed_at
FROM backups WHERE server_id=$1 ORDER BY created_at DESC`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]backup, 0)
	for rows.Next() {
		var item backup
		if err := rows.Scan(&item.ID, &item.ServerID, &item.Name, &item.SizeBytes, &item.ChecksumSHA256,
			&item.Status, &item.Error, &item.CreatedAt, &item.CompletedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *store) getBackup(ctx context.Context, serverID, backupID string) (backup, error) {
	var item backup
	err := s.db.QueryRow(ctx, `SELECT id,server_id,name,size_bytes,checksum_sha256,status,error,created_at,completed_at
FROM backups WHERE server_id=$1 AND id=$2`, serverID, backupID).Scan(&item.ID, &item.ServerID,
		&item.Name, &item.SizeBytes, &item.ChecksumSHA256, &item.Status, &item.Error,
		&item.CreatedAt, &item.CompletedAt)
	return item, err
}

func (s *store) updateBackup(ctx context.Context, backupID, status string, size int64, checksum string, jobErr error) error {
	var message *string
	if jobErr != nil {
		value := jobErr.Error()
		if len(value) > 500 {
			value = value[:500]
		}
		message = &value
	}
	_, err := s.db.Exec(ctx, `UPDATE backups SET status=$2,size_bytes=$3,checksum_sha256=NULLIF($4,''),error=$5,
completed_at=CASE WHEN $2 IN ('ready','failed') THEN now() ELSE completed_at END WHERE id=$1`,
		backupID, status, size, checksum, message)
	return err
}

func (s *store) deleteBackup(ctx context.Context, serverID, backupID string) error {
	ct, err := s.db.Exec(ctx, `DELETE FROM backups WHERE server_id=$1 AND id=$2`, serverID, backupID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *store) createSchedule(ctx context.Context, serverID, name, action string, payload any, intervalMinutes int) (schedule, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return schedule{}, err
	}
	var item schedule
	err = s.db.QueryRow(ctx, `INSERT INTO schedules
(id,server_id,name,action,payload,interval_minutes,next_run_at)
VALUES ($1,$2,$3,$4,$5,$6,now()+make_interval(mins=>$6))
RETURNING id,server_id,name,action,payload,interval_minutes,enabled,next_run_at,last_run_at,created_at`,
		uuid.NewString(), serverID, name, action, data, intervalMinutes).Scan(&item.ID, &item.ServerID,
		&item.Name, &item.Action, &item.Payload, &item.IntervalMinutes, &item.Enabled,
		&item.NextRunAt, &item.LastRunAt, &item.CreatedAt)
	return item, err
}

func (s *store) listSchedules(ctx context.Context, serverID string) ([]schedule, error) {
	rows, err := s.db.Query(ctx, `SELECT id,server_id,name,action,payload,interval_minutes,enabled,next_run_at,last_run_at,created_at
FROM schedules WHERE server_id=$1 ORDER BY created_at`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]schedule, 0)
	for rows.Next() {
		var item schedule
		if err := rows.Scan(&item.ID, &item.ServerID, &item.Name, &item.Action, &item.Payload,
			&item.IntervalMinutes, &item.Enabled, &item.NextRunAt, &item.LastRunAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *store) deleteSchedule(ctx context.Context, serverID, scheduleID string) error {
	ct, err := s.db.Exec(ctx, `DELETE FROM schedules WHERE server_id=$1 AND id=$2`, serverID, scheduleID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *store) claimDueSchedules(ctx context.Context, limit int) ([]schedule, error) {
	rows, err := s.db.Query(ctx, `WITH due AS (
  SELECT schedules.id FROM schedules
  LEFT JOIN subscriptions ON subscriptions.server_id=schedules.server_id
  WHERE schedules.enabled AND schedules.next_run_at<=now()
    AND (subscriptions.id IS NULL OR subscriptions.status='active')
  ORDER BY next_run_at FOR UPDATE OF schedules SKIP LOCKED LIMIT $1
)
UPDATE schedules s SET last_run_at=now(),next_run_at=now()+make_interval(mins=>s.interval_minutes)
FROM due WHERE s.id=due.id
RETURNING s.id,s.server_id,s.name,s.action,s.payload,s.interval_minutes,s.enabled,s.next_run_at,s.last_run_at,s.created_at`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]schedule, 0)
	for rows.Next() {
		var item schedule
		if err := rows.Scan(&item.ID, &item.ServerID, &item.Name, &item.Action, &item.Payload,
			&item.IntervalMinutes, &item.Enabled, &item.NextRunAt, &item.LastRunAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

var (
	errCapacity     = errors.New("node capacity exceeded")
	errNoAllocation = errors.New("no game port allocation available")
)

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func conflictMessage(err error) error {
	if isUniqueViolation(err) {
		return fmt.Errorf("conflicting active operation: %w", err)
	}
	return err
}
