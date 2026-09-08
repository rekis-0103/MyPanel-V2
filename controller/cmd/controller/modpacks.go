package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const curseForgeModpackClassID = "4471"

type curseForgeModpackProject struct {
	ID            int64  `json:"id"`
	ClassID       int64  `json:"classId"`
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	Summary       string `json:"summary"`
	DownloadCount int64  `json:"downloadCount"`
	Logo          *struct {
		ThumbnailURL string `json:"thumbnailUrl"`
	} `json:"logo"`
}

type curseForgeModpackFile struct {
	ID           int64     `json:"id"`
	ModID        int64     `json:"modId"`
	IsAvailable  bool      `json:"isAvailable"`
	DisplayName  string    `json:"displayName"`
	FileName     string    `json:"fileName"`
	ReleaseType  int       `json:"releaseType"`
	FileStatus   int       `json:"fileStatus"`
	FileDate     time.Time `json:"fileDate"`
	GameVersions []string  `json:"gameVersions"`
	IsServerPack bool      `json:"isServerPack"`
	ServerPackID int64     `json:"serverPackFileId"`
}

type modpackSearchResult struct {
	ProjectID string `json:"projectId"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	Summary   string `json:"summary"`
	IconURL   string `json:"iconUrl"`
	Downloads int64  `json:"downloads"`
}

type modpackVersion struct {
	FileID           string    `json:"fileId"`
	Name             string    `json:"name"`
	FileName         string    `json:"fileName"`
	Runtime          string    `json:"runtime"`
	MinecraftVersion string    `json:"minecraftVersion"`
	JavaVersion      int       `json:"javaVersion"`
	ReleaseType      string    `json:"releaseType"`
	PublishedAt      time.Time `json:"publishedAt"`
}

func (a *app) modpackRoutes(w http.ResponseWriter, r *http.Request, item server, parts []string) {
	if a.cfg.CurseForgeAPIKey == "" && (r.Method != http.MethodGet || len(parts) != 0) {
		write(w, http.StatusServiceUnavailable, apiError{Error: "CurseForge modpacks are not configured", Code: "provider_not_configured", RequestID: requestID(r.Context())})
		return
	}
	if len(parts) == 0 {
		switch r.Method {
		case http.MethodGet:
			installed, err := a.store.getModpack(r.Context(), item.ID)
			if errors.Is(err, pgx.ErrNoRows) {
				write(w, http.StatusOK, map[string]any{"item": nil})
				return
			}
			if err != nil {
				internal(w, r, err)
				return
			}
			write(w, http.StatusOK, map[string]any{"item": installed})
		case http.MethodPost:
			a.installModpack(w, r, item)
		default:
			method(w)
		}
		return
	}
	if len(parts) == 1 && parts[0] == "search" && r.Method == http.MethodGet {
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		if len(query) < 2 || len(query) > 80 {
			write(w, http.StatusBadRequest, apiError{Error: "search query must contain 2-80 characters", Code: "invalid_query", RequestID: requestID(r.Context())})
			return
		}
		items, err := a.searchCurseForgeModpacks(r.Context(), query)
		if err != nil {
			modpackProviderError(w, r, err)
			return
		}
		write(w, http.StatusOK, map[string]any{"provider": "curseforge", "items": items})
		return
	}
	if len(parts) == 2 && parts[1] == "versions" && r.Method == http.MethodGet {
		items, err := a.curseForgeModpackVersions(r.Context(), parts[0])
		if err != nil {
			modpackProviderError(w, r, err)
			return
		}
		write(w, http.StatusOK, map[string]any{"provider": "curseforge", "items": items})
		return
	}
	method(w)
}

func (a *app) installModpack(w http.ResponseWriter, r *http.Request, item server) {
	var input struct {
		ProjectID    string `json:"projectId"`
		FileID       string `json:"fileId"`
		Confirmation string `json:"confirmation"`
	}
	if decode(w, r, &input) != nil {
		return
	}
	if input.Confirmation != item.Name {
		write(w, http.StatusBadRequest, apiError{Error: "type the exact server name to confirm replacement", Code: "confirmation_mismatch", RequestID: requestID(r.Context())})
		return
	}
	project, version, err := a.resolveCurseForgeModpack(r.Context(), input.ProjectID, input.FileID)
	if err != nil {
		modpackProviderError(w, r, err)
		return
	}
	target := serverModpack{ServerID: item.ID, Provider: "curseforge", ProjectID: strconv.FormatInt(project.ID, 10), Slug: project.Slug,
		FileID: version.FileID, Name: project.Name, VersionName: version.Name, Runtime: version.Runtime,
		MinecraftVersion: version.MinecraftVersion, JavaVersion: version.JavaVersion, Status: "installing"}
	if project.Logo != nil {
		target.IconURL = project.Logo.ThumbnailURL
	}
	updated, installed, backupItem, createdJob, err := a.store.createModpackInstallJob(r.Context(), item.ID, target)
	if err != nil {
		write(w, http.StatusConflict, apiError{Error: "another server operation is active", Code: "job_conflict", RequestID: requestID(r.Context())})
		return
	}
	session, _ := currentSession(r.Context())
	_ = a.store.audit(r.Context(), &session.UserID, "modpack.install", "server", item.ID, clientIP(r), map[string]any{
		"provider": "curseforge", "projectId": target.ProjectID, "fileId": target.FileID,
		"runtime": target.Runtime, "minecraftVersion": target.MinecraftVersion, "backupId": backupItem.ID,
	})
	write(w, http.StatusAccepted, map[string]any{"server": updated, "modpack": installed, "backup": backupItem, "job": createdJob})
}

func (a *app) searchCurseForgeModpacks(ctx context.Context, query string) ([]modpackSearchResult, error) {
	if a.cfg.CurseForgeAPIKey == "" {
		return nil, errors.New("CurseForge provider is not configured")
	}
	values := url.Values{"gameId": {"432"}, "classId": {curseForgeModpackClassID}, "searchFilter": {query}, "pageSize": {"20"}}
	var response struct {
		Data []curseForgeModpackProject `json:"data"`
	}
	if err := a.providerJSON(ctx, "https://api.curseforge.com/v1/mods/search?"+values.Encode(), a.cfg.CurseForgeAPIKey, &response); err != nil {
		return nil, err
	}
	items := make([]modpackSearchResult, 0, len(response.Data))
	for _, project := range response.Data {
		if !validCurseForgeModpackProject(project) {
			continue
		}
		icon := ""
		if project.Logo != nil {
			icon = project.Logo.ThumbnailURL
		}
		items = append(items, modpackSearchResult{ProjectID: strconv.FormatInt(project.ID, 10), Slug: project.Slug, Name: project.Name, Summary: project.Summary, IconURL: icon, Downloads: project.DownloadCount})
	}
	return items, nil
}

func (a *app) curseForgeModpackVersions(ctx context.Context, projectID string) ([]modpackVersion, error) {
	id, err := parseCurseForgeID(projectID)
	if err != nil {
		return nil, err
	}
	var response struct {
		Data []curseForgeModpackFile `json:"data"`
	}
	endpoint := fmt.Sprintf("https://api.curseforge.com/v1/mods/%d/files?pageSize=50", id)
	if err := a.providerJSON(ctx, endpoint, a.cfg.CurseForgeAPIKey, &response); err != nil {
		return nil, err
	}
	items := make([]modpackVersion, 0, len(response.Data))
	for _, file := range response.Data {
		version, compatible := compatibleModpackVersion(file)
		if compatible {
			items = append(items, version)
		}
	}
	return items, nil
}

func (a *app) resolveCurseForgeModpack(ctx context.Context, projectID, fileID string) (curseForgeModpackProject, modpackVersion, error) {
	projectNumber, err := parseCurseForgeID(projectID)
	if err != nil {
		return curseForgeModpackProject{}, modpackVersion{}, err
	}
	fileNumber, err := parseCurseForgeID(fileID)
	if err != nil {
		return curseForgeModpackProject{}, modpackVersion{}, err
	}
	var projectResponse struct {
		Data curseForgeModpackProject `json:"data"`
	}
	if err := a.providerJSON(ctx, fmt.Sprintf("https://api.curseforge.com/v1/mods/%d", projectNumber), a.cfg.CurseForgeAPIKey, &projectResponse); err != nil {
		return curseForgeModpackProject{}, modpackVersion{}, err
	}
	if projectResponse.Data.ID != projectNumber || !validCurseForgeModpackProject(projectResponse.Data) {
		return curseForgeModpackProject{}, modpackVersion{}, errors.New("invalid CurseForge modpack project")
	}
	var fileResponse struct {
		Data curseForgeModpackFile `json:"data"`
	}
	endpoint := fmt.Sprintf("https://api.curseforge.com/v1/mods/%d/files/%d", projectNumber, fileNumber)
	if err := a.providerJSON(ctx, endpoint, a.cfg.CurseForgeAPIKey, &fileResponse); err != nil {
		return curseForgeModpackProject{}, modpackVersion{}, err
	}
	if fileResponse.Data.ID != fileNumber || fileResponse.Data.ModID != projectNumber {
		return curseForgeModpackProject{}, modpackVersion{}, errors.New("CurseForge file does not belong to this modpack")
	}
	version, compatible := compatibleModpackVersion(fileResponse.Data)
	if !compatible {
		return curseForgeModpackProject{}, modpackVersion{}, errors.New("selected CurseForge file is not a supported Forge or NeoForge modpack manifest")
	}
	return projectResponse.Data, version, nil
}

func compatibleModpackVersion(file curseForgeModpackFile) (modpackVersion, bool) {
	if file.ID <= 0 || !file.IsAvailable || file.IsServerPack || file.FileStatus == 6 || file.FileStatus == 7 {
		return modpackVersion{}, false
	}
	runtime := ""
	for _, value := range file.GameVersions {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "neoforge":
			runtime = "neoforge"
		case "forge":
			if runtime == "" {
				runtime = "forge"
			}
		}
	}
	if runtime == "" {
		return modpackVersion{}, false
	}
	available := make(map[string]bool, len(file.GameVersions))
	for _, value := range file.GameVersions {
		available[strings.TrimSpace(value)] = true
	}
	minecraftVersion := ""
	for _, supported := range supportedMinecraftVersions {
		if available[supported.ID] {
			minecraftVersion = supported.ID
			break
		}
	}
	if minecraftVersion == "" {
		return modpackVersion{}, false
	}
	return modpackVersion{FileID: strconv.FormatInt(file.ID, 10), Name: file.DisplayName, FileName: file.FileName,
		Runtime: runtime, MinecraftVersion: minecraftVersion, JavaVersion: requiredJavaVersion(minecraftVersion),
		ReleaseType: curseForgeReleaseType(file.ReleaseType), PublishedAt: file.FileDate}, true
}

func parseCurseForgeID(value string) (int64, error) {
	if len(value) == 0 || len(value) > 19 || value[0] == '0' {
		return 0, errors.New("invalid CurseForge identifier")
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid CurseForge identifier")
	}
	return id, nil
}

func curseForgeReleaseType(value int) string {
	switch value {
	case 1:
		return "release"
	case 2:
		return "beta"
	case 3:
		return "alpha"
	default:
		return "unknown"
	}
}

func validCurseForgeSlug(value string) bool {
	if len(value) < 1 || len(value) > 80 || value[0] == '-' || value[len(value)-1] == '-' {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
			return false
		}
	}
	return true
}

func validCurseForgeModpackProject(project curseForgeModpackProject) bool {
	return project.ID > 0 && project.ClassID == 4471 && validCurseForgeSlug(project.Slug)
}

func modpackProviderError(w http.ResponseWriter, r *http.Request, err error) {
	code := "provider_unavailable"
	status := http.StatusBadGateway
	message := "CurseForge is temporarily unavailable"
	if strings.Contains(strings.ToLower(err.Error()), "not configured") {
		code, status, message = "provider_not_configured", http.StatusServiceUnavailable, "CurseForge modpacks are not configured"
	} else if strings.Contains(strings.ToLower(err.Error()), "invalid curseforge") || strings.Contains(strings.ToLower(err.Error()), "not a supported") || strings.Contains(strings.ToLower(err.Error()), "does not belong") {
		code, status, message = "incompatible_modpack", http.StatusUnprocessableEntity, err.Error()
	}
	write(w, status, apiError{Error: message, Code: code, RequestID: requestID(r.Context())})
}

func (a *app) executeModpackInstall(ctx context.Context, item job, serverItem server, targetSpec agentServerSpec, result *any) error {
	var payload modpackInstallPayload
	if json.Unmarshal(item.Payload, &payload) != nil || payload.OldSpec.ID != item.ServerID || payload.BackupID == "" || targetSpec.Modpack == nil {
		return errors.New("invalid modpack installation payload")
	}
	a.recordConsoleEvent(ctx, item.ServerID, fmt.Sprintf("Installing CurseForge modpack %s (%s)...", targetSpec.Modpack.Slug, targetSpec.Modpack.FileID))
	if err := a.agent.serverAction(ctx, item.ServerID, "stop", map[string]any{}, nil); err != nil {
		_ = a.store.updateBackup(ctx, payload.BackupID, "failed", 0, "", err)
		rollbackErr := a.store.rollbackModpackInstall(ctx, payload)
		return errors.Join(fmt.Errorf("stop server before modpack installation: %w", err), rollbackErr)
	}
	if err := a.store.updateBackup(ctx, payload.BackupID, "creating", 0, "", nil); err != nil {
		rollbackErr := a.rollbackModpackRuntime(ctx, payload, "", false)
		return errors.Join(fmt.Errorf("mark pre-modpack backup as creating: %w", err), rollbackErr)
	}
	var backupOutput struct {
		BackupID       string `json:"backupId"`
		SizeBytes      int64  `json:"sizeBytes"`
		ChecksumSHA256 string `json:"checksumSha256"`
	}
	if err := a.agent.serverAction(ctx, item.ServerID, "backup", map[string]string{"backupId": payload.BackupID}, &backupOutput); err != nil {
		_ = a.store.updateBackup(ctx, payload.BackupID, "failed", 0, "", err)
		rollbackErr := a.rollbackModpackRuntime(ctx, payload, "", false)
		return errors.Join(fmt.Errorf("create pre-modpack backup: %w", err), rollbackErr)
	}
	if err := a.store.updateBackup(ctx, payload.BackupID, "ready", backupOutput.SizeBytes, backupOutput.ChecksumSHA256, nil); err != nil {
		rollbackErr := a.rollbackModpackRuntime(ctx, payload, backupOutput.ChecksumSHA256, false)
		return errors.Join(fmt.Errorf("record pre-modpack backup: %w", err), rollbackErr)
	}

	installErr := a.agent.serverAction(ctx, item.ServerID, "provision", targetSpec, nil)
	if installErr == nil {
		installErr = a.agent.serverAction(ctx, item.ServerID, "start", targetSpec, nil)
	}
	if installErr == nil {
		_, installErr = a.waitForServerRunningWithin(ctx, item.ServerID, 30*time.Minute)
	}
	if installErr != nil {
		rollbackErr := a.rollbackModpackRuntime(ctx, payload, backupOutput.ChecksumSHA256, true)
		if rollbackErr == nil {
			a.recordConsoleEvent(ctx, item.ServerID, "Modpack installation failed; the previous server was restored from backup.")
		}
		return errors.Join(fmt.Errorf("install CurseForge modpack: %w", installErr), rollbackErr)
	}

	if err := a.store.finishModpackInstall(ctx, item.ServerID, targetSpec.Modpack.FileID); err != nil {
		return err
	}
	state := "running"
	warning := ""
	if payload.DesiredState != "running" {
		if err := a.agent.serverAction(ctx, item.ServerID, "stop", map[string]any{}, nil); err != nil {
			// The replacement itself is healthy and committed. Keep the desired state
			// offline so the reconciler can retry without destroying a valid install.
			warning = "modpack installed, but the server could not be stopped; MyPanel will retry"
		} else {
			state = "offline"
		}
	}
	_ = a.store.setObservedState(ctx, item.ServerID, state, nil)
	a.recordConsoleEvent(ctx, item.ServerID, "CurseForge modpack installed successfully.")
	*result = map[string]any{"backupId": payload.BackupID, "modpack": targetSpec.Modpack, "state": state, "warning": warning}
	return nil
}

func (a *app) rollbackModpackRuntime(ctx context.Context, payload modpackInstallPayload, checksum string, restoreData bool) error {
	var rollbackErrors []error
	if restoreData && checksum != "" {
		if err := a.agent.serverAction(ctx, payload.OldSpec.ID, "restore", map[string]string{"backupId": payload.BackupID, "expectedSha256": checksum}, nil); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("restore pre-modpack backup: %w", err))
		}
	}
	if err := a.store.rollbackModpackInstall(ctx, payload); err != nil {
		rollbackErrors = append(rollbackErrors, fmt.Errorf("restore previous server definition: %w", err))
	}
	if restoreData {
		if err := a.agent.serverAction(ctx, payload.OldSpec.ID, "provision", payload.OldSpec, nil); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("recreate previous runtime: %w", err))
		}
	}
	if payload.DesiredState == "running" {
		if err := a.agent.serverAction(ctx, payload.OldSpec.ID, "start", payload.OldSpec, nil); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("restart previous runtime: %w", err))
		} else if _, err := a.waitForServerRunning(ctx, payload.OldSpec.ID); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("wait for previous runtime: %w", err))
		}
	}
	return errors.Join(rollbackErrors...)
}
