package main

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const modpackColumns = `server_id,provider,project_id,slug,file_id,name,version_name,icon_url,runtime,minecraft_version,java_version,status,installed_at,created_at,updated_at`

type modpackInstallPayload struct {
	BackupID     string          `json:"backupId"`
	DesiredState string          `json:"desiredState"`
	OldSpec      agentServerSpec `json:"oldSpec"`
	OldModpack   *serverModpack  `json:"oldModpack,omitempty"`
}

func scanModpack(row rowScanner) (serverModpack, error) {
	var item serverModpack
	err := row.Scan(&item.ServerID, &item.Provider, &item.ProjectID, &item.Slug, &item.FileID, &item.Name,
		&item.VersionName, &item.IconURL, &item.Runtime, &item.MinecraftVersion, &item.JavaVersion,
		&item.Status, &item.InstalledAt, &item.CreatedAt, &item.UpdatedAt)
	return item, err
}

func (s *store) getModpack(ctx context.Context, serverID string) (serverModpack, error) {
	return scanModpack(s.db.QueryRow(ctx, `SELECT `+modpackColumns+` FROM server_modpacks WHERE server_id=$1`, serverID))
}

func getModpackTx(ctx context.Context, tx pgx.Tx, serverID string) (serverModpack, error) {
	return scanModpack(tx.QueryRow(ctx, `SELECT `+modpackColumns+` FROM server_modpacks WHERE server_id=$1`, serverID))
}

func (s *store) createModpackInstallJob(ctx context.Context, serverID string, target serverModpack) (server, serverModpack, backup, job, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return server{}, serverModpack{}, backup{}, job{}, err
	}
	defer tx.Rollback(ctx)

	oldSpec := agentServerSpec{ID: serverID}
	var desiredState string
	if err := tx.QueryRow(ctx, `SELECT runtime,version,java_version,memory_mb,cpu,disk_mb,bind_ip,port,config,desired_state
FROM servers WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, serverID).Scan(&oldSpec.Runtime, &oldSpec.Version,
		&oldSpec.JavaVersion, &oldSpec.MemoryMB, &oldSpec.CPU, &oldSpec.DiskMB, &oldSpec.BindIP,
		&oldSpec.Port, &oldSpec.Config, &desiredState); err != nil {
		return server{}, serverModpack{}, backup{}, job{}, err
	}
	var oldModpack *serverModpack
	if current, currentErr := getModpackTx(ctx, tx, serverID); currentErr == nil {
		oldModpack = &current
		oldSpec.Modpack = &agentModpackSpec{Provider: current.Provider, Slug: current.Slug, FileID: current.FileID}
	} else if !errors.Is(currentErr, pgx.ErrNoRows) {
		return server{}, serverModpack{}, backup{}, job{}, currentErr
	}

	backupItem := backup{ID: uuid.NewString(), ServerID: serverID, Name: "Pre-modpack change", Status: "queued", Kind: "pre_modpack"}
	if err := tx.QueryRow(ctx, `INSERT INTO backups(id,server_id,name,status,kind) VALUES($1,$2,$3,'queued','pre_modpack') RETURNING created_at`,
		backupItem.ID, serverID, backupItem.Name).Scan(&backupItem.CreatedAt); err != nil {
		return server{}, serverModpack{}, backup{}, job{}, err
	}

	target.ServerID = serverID
	target.Provider = "curseforge"
	target.Status = "installing"
	err = tx.QueryRow(ctx, `INSERT INTO server_modpacks(server_id,provider,project_id,slug,file_id,name,version_name,icon_url,runtime,minecraft_version,java_version,status)
VALUES($1,'curseforge',$2,$3,$4,$5,$6,$7,$8,$9,$10,'installing')
ON CONFLICT(server_id) DO UPDATE SET provider='curseforge',project_id=EXCLUDED.project_id,slug=EXCLUDED.slug,file_id=EXCLUDED.file_id,
name=EXCLUDED.name,version_name=EXCLUDED.version_name,icon_url=EXCLUDED.icon_url,runtime=EXCLUDED.runtime,
minecraft_version=EXCLUDED.minecraft_version,java_version=EXCLUDED.java_version,status='installing',installed_at=NULL,updated_at=now()
RETURNING installed_at,created_at,updated_at`, serverID, target.ProjectID, target.Slug, target.FileID, target.Name,
		target.VersionName, target.IconURL, target.Runtime, target.MinecraftVersion, target.JavaVersion).
		Scan(&target.InstalledAt, &target.CreatedAt, &target.UpdatedAt)
	if err != nil {
		return server{}, serverModpack{}, backup{}, job{}, err
	}
	commandTag, err := tx.Exec(ctx, `UPDATE servers SET runtime=$2,version=$3,java_version=$4,observed_state='installing',last_error=NULL,restart_required=false,updated_at=now()
WHERE id=$1 AND deleted_at IS NULL`, serverID, target.Runtime, target.MinecraftVersion, target.JavaVersion)
	if err != nil || commandTag.RowsAffected() == 0 {
		if err == nil {
			err = pgx.ErrNoRows
		}
		return server{}, serverModpack{}, backup{}, job{}, err
	}
	payload := modpackInstallPayload{BackupID: backupItem.ID, DesiredState: desiredState, OldSpec: oldSpec, OldModpack: oldModpack}
	createdJob, err := insertJob(ctx, tx, serverID, "modpack_install", payload)
	if err != nil {
		return server{}, serverModpack{}, backup{}, job{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return server{}, serverModpack{}, backup{}, job{}, err
	}
	updated, err := s.get(ctx, serverID)
	if err != nil {
		return server{}, serverModpack{}, backup{}, job{}, err
	}
	return updated, target, backupItem, createdJob, nil
}

func (s *store) finishModpackInstall(ctx context.Context, serverID, fileID string) error {
	commandTag, err := s.db.Exec(ctx, `UPDATE server_modpacks SET status='installed',installed_at=now(),updated_at=now()
WHERE server_id=$1 AND file_id=$2 AND status='installing'`, serverID, fileID)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *store) rollbackModpackInstall(ctx context.Context, payload modpackInstallPayload) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE servers SET runtime=$2,version=$3,java_version=$4,config=$5,desired_state=$6,restart_required=false,updated_at=now()
WHERE id=$1 AND deleted_at IS NULL`, payload.OldSpec.ID, payload.OldSpec.Runtime, payload.OldSpec.Version,
		payload.OldSpec.JavaVersion, json.RawMessage(payload.OldSpec.Config), payload.DesiredState); err != nil {
		return err
	}
	if payload.OldModpack == nil {
		if _, err := tx.Exec(ctx, `DELETE FROM server_modpacks WHERE server_id=$1`, payload.OldSpec.ID); err != nil {
			return err
		}
	} else {
		old := payload.OldModpack
		if _, err := tx.Exec(ctx, `INSERT INTO server_modpacks(server_id,provider,project_id,slug,file_id,name,version_name,icon_url,runtime,minecraft_version,java_version,status,installed_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'installed',$12)
ON CONFLICT(server_id) DO UPDATE SET provider=EXCLUDED.provider,project_id=EXCLUDED.project_id,slug=EXCLUDED.slug,file_id=EXCLUDED.file_id,
name=EXCLUDED.name,version_name=EXCLUDED.version_name,icon_url=EXCLUDED.icon_url,runtime=EXCLUDED.runtime,
minecraft_version=EXCLUDED.minecraft_version,java_version=EXCLUDED.java_version,status='installed',installed_at=EXCLUDED.installed_at,updated_at=now()`,
			old.ServerID, old.Provider, old.ProjectID, old.Slug, old.FileID, old.Name, old.VersionName,
			old.IconURL, old.Runtime, old.MinecraftVersion, old.JavaVersion, old.InstalledAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
