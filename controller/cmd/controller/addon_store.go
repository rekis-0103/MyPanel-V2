package main

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

func (s *store) listAddons(ctx context.Context, serverID string) ([]serverAddon, error) {
	rows, err := s.db.Query(ctx, `SELECT id,server_id,provider,project_id,version_id,name,file_name,file_hash,managed_path,status,metadata,created_at,updated_at FROM server_addons WHERE server_id=$1 ORDER BY name`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]serverAddon, 0)
	for rows.Next() {
		var item serverAddon
		if err := rows.Scan(&item.ID, &item.ServerID, &item.Provider, &item.ProjectID, &item.VersionID, &item.Name, &item.FileName, &item.FileHash, &item.ManagedPath, &item.Status, &item.Metadata, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *store) createAddonJob(ctx context.Context, serverID string, item serverAddon, files any) (serverAddon, job, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return serverAddon{}, job{}, err
	}
	defer tx.Rollback(ctx)
	item.ID = uuid.NewString()
	item.Status = "installing"
	if len(item.Metadata) == 0 {
		item.Metadata = json.RawMessage(`{}`)
	}
	err = tx.QueryRow(ctx, `INSERT INTO server_addons(id,server_id,provider,project_id,version_id,name,file_name,file_hash,managed_path,status,metadata) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'installing',$10) RETURNING created_at,updated_at`, item.ID, serverID, item.Provider, item.ProjectID, item.VersionID, item.Name, item.FileName, item.FileHash, item.ManagedPath, item.Metadata).Scan(&item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return serverAddon{}, job{}, err
	}
	created, err := insertJob(ctx, tx, serverID, "addon_install", map[string]any{"addonId": item.ID, "files": files})
	if err != nil {
		return serverAddon{}, job{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return serverAddon{}, job{}, err
	}
	return item, created, nil
}

func (s *store) updateAddonJob(ctx context.Context, serverID, addonID string, item serverAddon, files any) (serverAddon, job, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return serverAddon{}, job{}, err
	}
	defer tx.Rollback(ctx)
	var oldPath string
	if err := tx.QueryRow(ctx, `SELECT managed_path FROM server_addons WHERE server_id=$1 AND id=$2 FOR UPDATE`, serverID, addonID).Scan(&oldPath); err != nil {
		return serverAddon{}, job{}, err
	}
	item.ID, item.ServerID, item.Status = addonID, serverID, "installing"
	if len(item.Metadata) == 0 {
		item.Metadata = json.RawMessage(`{}`)
	}
	err = tx.QueryRow(ctx, `UPDATE server_addons SET provider=$3,project_id=$4,version_id=$5,name=$6,file_name=$7,file_hash=$8,managed_path=$9,status='installing',metadata=$10,updated_at=now() WHERE server_id=$1 AND id=$2 RETURNING created_at,updated_at`, serverID, addonID, item.Provider, item.ProjectID, item.VersionID, item.Name, item.FileName, item.FileHash, item.ManagedPath, item.Metadata).Scan(&item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return serverAddon{}, job{}, err
	}
	created, err := insertJob(ctx, tx, serverID, "addon_install", map[string]any{"addonId": addonID, "files": files, "oldPath": oldPath})
	if err != nil {
		return serverAddon{}, job{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return serverAddon{}, job{}, err
	}
	return item, created, nil
}

func (s *store) createAddonRemoveJob(ctx context.Context, serverID, addonID string) (serverAddon, job, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return serverAddon{}, job{}, err
	}
	defer tx.Rollback(ctx)
	var item serverAddon
	err = tx.QueryRow(ctx, `SELECT id,server_id,provider,project_id,version_id,name,file_name,file_hash,managed_path,status,metadata,created_at,updated_at FROM server_addons WHERE server_id=$1 AND id=$2`, serverID, addonID).Scan(&item.ID, &item.ServerID, &item.Provider, &item.ProjectID, &item.VersionID, &item.Name, &item.FileName, &item.FileHash, &item.ManagedPath, &item.Status, &item.Metadata, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return serverAddon{}, job{}, err
	}
	created, err := insertJob(ctx, tx, serverID, "addon_remove", map[string]any{"addonId": addonID, "path": item.ManagedPath})
	if err != nil {
		return serverAddon{}, job{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return serverAddon{}, job{}, err
	}
	return item, created, nil
}

func (s *store) finishAddon(ctx context.Context, addonID string, install bool, jobErr error) error {
	if jobErr != nil {
		_, err := s.db.Exec(ctx, `UPDATE server_addons SET status='failed',updated_at=now() WHERE id=$1`, addonID)
		return err
	}
	if !install {
		_, err := s.db.Exec(ctx, `DELETE FROM server_addons WHERE id=$1`, addonID)
		return err
	}
	_, err := s.db.Exec(ctx, `WITH updated AS (
  UPDATE server_addons SET status='installed',updated_at=now() WHERE id=$1 RETURNING server_id
) UPDATE servers SET restart_required=true,updated_at=now() WHERE id IN (SELECT server_id FROM updated)`, addonID)
	return err
}

func (s *store) clearRestartRequired(ctx context.Context, serverID string) error {
	_, err := s.db.Exec(ctx, `UPDATE servers SET restart_required=false WHERE id=$1`, serverID)
	return err
}
