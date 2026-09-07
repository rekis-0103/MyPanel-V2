package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type addonSearchResult struct {
	Provider      string `json:"provider"`
	ProjectID     string `json:"projectId"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	IconURL       string `json:"iconUrl"`
	Downloads     int64  `json:"downloads"`
	LatestVersion string `json:"latestVersion,omitempty"`
}
type resolvedAddonFile struct {
	URL       string `json:"url"`
	FileName  string `json:"fileName"`
	Directory string `json:"directory"`
	Hash      string `json:"hash"`
	HashType  string `json:"hashType"`
	ProjectID string `json:"-"`
	VersionID string `json:"-"`
	Name      string `json:"-"`
}

func (a *app) addonRoutes(w http.ResponseWriter, r *http.Request, item server, parts []string) {
	if !addonRuntimeSupported(item.Runtime) && ((len(parts) == 1 && parts[0] == "search") || (len(parts) == 0 && r.Method == http.MethodPost)) {
		write(w, http.StatusConflict, apiError{Error: "this server runtime does not load managed add-ons", Code: "runtime_unsupported", RequestID: requestID(r.Context())})
		return
	}
	if len(parts) == 1 && parts[0] == "search" && r.Method == http.MethodGet {
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		provider := strings.ToLower(r.URL.Query().Get("provider"))
		if provider == "" {
			provider = "modrinth"
		}
		if len(query) < 2 || len(query) > 80 {
			write(w, http.StatusBadRequest, apiError{Error: "search query must contain 2-80 characters", Code: "invalid_query", RequestID: requestID(r.Context())})
			return
		}
		results, err := a.searchAddonProvider(r.Context(), provider, query, item)
		if err != nil {
			write(w, http.StatusBadGateway, apiError{Error: err.Error(), Code: "provider_unavailable", RequestID: requestID(r.Context())})
			return
		}
		write(w, http.StatusOK, map[string]any{"provider": provider, "items": results})
		return
	}
	if len(parts) == 0 {
		switch r.Method {
		case http.MethodGet:
			items, err := a.store.listAddons(r.Context(), item.ID)
			if err != nil {
				internal(w, r, err)
				return
			}
			write(w, http.StatusOK, items)
		case http.MethodPost:
			var input struct {
				Provider            string `json:"provider"`
				ProjectID           string `json:"projectId"`
				VersionID           string `json:"versionId"`
				AddonID             string `json:"addonId"`
				ConfirmDependencies bool   `json:"confirmDependencies"`
			}
			if decode(w, r, &input) != nil {
				return
			}
			input.Provider = strings.ToLower(strings.TrimSpace(input.Provider))
			input.ProjectID = strings.TrimSpace(input.ProjectID)
			input.VersionID = strings.TrimSpace(input.VersionID)
			files, err := a.resolveAddon(r.Context(), input.Provider, input.ProjectID, input.VersionID, item)
			if err != nil {
				write(w, http.StatusBadGateway, apiError{Error: err.Error(), Code: "provider_unavailable", RequestID: requestID(r.Context())})
				return
			}
			if len(files) > 1 && !input.ConfirmDependencies {
				write(w, http.StatusConflict, map[string]any{"error": "required dependencies need confirmation", "code": "dependencies_required", "dependencies": files[1:]})
				return
			}
			primary := files[0]
			metadata, _ := json.Marshal(map[string]any{"dependencies": max(0, len(files)-1)})
			addon := serverAddon{Provider: input.Provider, ProjectID: primary.ProjectID, VersionID: primary.VersionID, Name: primary.Name, FileName: primary.FileName, FileHash: primary.Hash, ManagedPath: primary.Directory + "/" + primary.FileName, Metadata: metadata}
			var created serverAddon
			var job job
			if input.AddonID != "" {
				if uuid.Validate(input.AddonID) != nil {
					write(w, http.StatusBadRequest, apiError{Error: "invalid add-on ID", Code: "invalid_addon", RequestID: requestID(r.Context())})
					return
				}
				created, job, err = a.store.updateAddonJob(r.Context(), item.ID, input.AddonID, addon, files)
			} else {
				created, job, err = a.store.createAddonJob(r.Context(), item.ID, addon, files)
			}
			if err != nil {
				write(w, http.StatusConflict, apiError{Error: "add-on install conflicts with an active operation or managed file", Code: "job_conflict", RequestID: requestID(r.Context())})
				return
			}
			session, _ := currentSession(r.Context())
			_ = a.store.audit(r.Context(), &session.UserID, "addon.install", "server", item.ID, clientIP(r), map[string]any{"addonId": created.ID, "provider": created.Provider, "projectId": created.ProjectID})
			write(w, http.StatusAccepted, map[string]any{"addon": created, "job": job})
		default:
			method(w)
		}
		return
	}
	if len(parts) == 1 && uuid.Validate(parts[0]) == nil && r.Method == http.MethodDelete {
		addon, job, err := a.store.createAddonRemoveJob(r.Context(), item.ID, parts[0])
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				notFound(w, r)
			} else {
				write(w, http.StatusConflict, apiError{Error: "another operation is active", Code: "job_conflict", RequestID: requestID(r.Context())})
			}
			return
		}
		session, _ := currentSession(r.Context())
		_ = a.store.audit(r.Context(), &session.UserID, "addon.remove", "server", item.ID, clientIP(r), map[string]any{"addonId": addon.ID})
		write(w, http.StatusAccepted, map[string]any{"addon": addon, "job": job})
		return
	}
	method(w)
}

func (a *app) searchAddonProvider(ctx context.Context, provider, query string, item server) ([]addonSearchResult, error) {
	if provider == "modrinth" {
		return a.searchModrinth(ctx, query, item)
	}
	if provider == "curseforge" {
		return a.searchCurseForge(ctx, query, item)
	}
	return nil, errors.New("unsupported add-on provider")
}
func (a *app) searchModrinth(ctx context.Context, query string, item server) ([]addonSearchResult, error) {
	projectType := "mod"
	loader := item.Runtime
	if item.Runtime == "paper" || item.Runtime == "purpur" {
		projectType = "plugin"
		loader = "paper"
	}
	facets, _ := json.Marshal([][]string{{"project_type:" + projectType}, {"versions:" + item.Version}, {"categories:" + loader}})
	values := url.Values{"query": {query}, "limit": {"20"}, "facets": {string(facets)}}
	var response struct {
		Hits []struct {
			ProjectID     string `json:"project_id"`
			Title         string `json:"title"`
			Description   string `json:"description"`
			IconURL       string `json:"icon_url"`
			Downloads     int64  `json:"downloads"`
			LatestVersion string `json:"latest_version"`
		} `json:"hits"`
	}
	if err := a.providerJSON(ctx, "https://api.modrinth.com/v2/search?"+values.Encode(), "", &response); err != nil {
		return nil, err
	}
	out := make([]addonSearchResult, 0, len(response.Hits))
	for _, hit := range response.Hits {
		out = append(out, addonSearchResult{Provider: "modrinth", ProjectID: hit.ProjectID, Name: hit.Title, Description: hit.Description, IconURL: hit.IconURL, Downloads: hit.Downloads, LatestVersion: hit.LatestVersion})
	}
	return out, nil
}

type modrinthVersion struct {
	ID           string `json:"id"`
	ProjectID    string `json:"project_id"`
	Name         string `json:"name"`
	Dependencies []struct {
		VersionID      *string `json:"version_id"`
		ProjectID      *string `json:"project_id"`
		DependencyType string  `json:"dependency_type"`
	} `json:"dependencies"`
	Files []struct {
		URL      string            `json:"url"`
		Filename string            `json:"filename"`
		Primary  bool              `json:"primary"`
		Hashes   map[string]string `json:"hashes"`
	} `json:"files"`
}

func (a *app) resolveModrinth(ctx context.Context, projectID, versionID string, item server) ([]resolvedAddonFile, error) {
	return a.resolveModrinthDepth(ctx, projectID, versionID, item, 0, map[string]bool{})
}

func (a *app) resolveModrinthDepth(ctx context.Context, projectID, versionID string, item server, depth int, visited map[string]bool) ([]resolvedAddonFile, error) {
	if depth > 4 {
		return nil, errors.New("Modrinth dependency graph is too deep")
	}
	key := projectID + ":" + versionID
	if visited[key] {
		return nil, errors.New("Modrinth dependency cycle detected")
	}
	visited[key] = true
	defer delete(visited, key)
	var versions []modrinthVersion
	if versionID != "" {
		var version modrinthVersion
		if err := a.providerJSON(ctx, "https://api.modrinth.com/v2/version/"+url.PathEscape(versionID), "", &version); err != nil {
			return nil, err
		}
		versions = []modrinthVersion{version}
	} else {
		loader := item.Runtime
		if loader == "purpur" {
			loader = "paper"
		}
		loaders, _ := json.Marshal([]string{loader})
		games, _ := json.Marshal([]string{item.Version})
		endpoint := "https://api.modrinth.com/v2/project/" + url.PathEscape(projectID) + "/version?" + url.Values{"loaders": {string(loaders)}, "game_versions": {string(games)}}.Encode()
		if err := a.providerJSON(ctx, endpoint, "", &versions); err != nil {
			return nil, err
		}
	}
	if len(versions) == 0 {
		return nil, errors.New("no compatible Modrinth version found")
	}
	primary, err := modrinthFile(versions[0], addonDirectory(item.Runtime))
	if err != nil {
		return nil, err
	}
	files := []resolvedAddonFile{primary}
	for _, dependency := range versions[0].Dependencies {
		if dependency.DependencyType != "required" || len(files) >= 16 {
			continue
		}
		id := ""
		if dependency.VersionID != nil {
			id = *dependency.VersionID
		}
		project := ""
		if dependency.ProjectID != nil {
			project = *dependency.ProjectID
		}
		resolved, depErr := a.resolveModrinthDepth(ctx, project, id, item, depth+1, visited)
		if depErr != nil {
			return nil, fmt.Errorf("resolve required dependency: %w", depErr)
		}
		files = append(files, resolved[0])
	}
	return files, nil
}
func modrinthFile(version modrinthVersion, directory string) (resolvedAddonFile, error) {
	if len(version.Files) == 0 {
		return resolvedAddonFile{}, errors.New("Modrinth version has no downloadable file")
	}
	file := version.Files[0]
	for _, candidate := range version.Files {
		if candidate.Primary {
			file = candidate
			break
		}
	}
	hash := file.Hashes["sha512"]
	if hash == "" {
		return resolvedAddonFile{}, errors.New("Modrinth file has no SHA-512 hash")
	}
	return resolvedAddonFile{URL: file.URL, FileName: file.Filename, Directory: directory, Hash: hash, HashType: "sha512", ProjectID: version.ProjectID, VersionID: version.ID, Name: version.Name}, nil
}

func (a *app) searchCurseForge(ctx context.Context, query string, item server) ([]addonSearchResult, error) {
	if a.cfg.CurseForgeAPIKey == "" {
		return nil, errors.New("CurseForge provider is not configured")
	}
	classID := "6"
	if item.Runtime == "paper" || item.Runtime == "purpur" {
		classID = "5"
	}
	values := url.Values{"gameId": {"432"}, "classId": {classID}, "searchFilter": {query}, "gameVersion": {item.Version}, "pageSize": {"20"}}
	if item.Runtime == "fabric" {
		values.Set("modLoaderType", "4")
	}
	var response struct {
		Data []struct {
			ID            int64  `json:"id"`
			Name          string `json:"name"`
			Summary       string `json:"summary"`
			DownloadCount int64  `json:"downloadCount"`
			Logo          *struct {
				ThumbnailURL string `json:"thumbnailUrl"`
			} `json:"logo"`
		} `json:"data"`
	}
	if err := a.providerJSON(ctx, "https://api.curseforge.com/v1/mods/search?"+values.Encode(), a.cfg.CurseForgeAPIKey, &response); err != nil {
		return nil, err
	}
	out := make([]addonSearchResult, 0, len(response.Data))
	for _, hit := range response.Data {
		icon := ""
		if hit.Logo != nil {
			icon = hit.Logo.ThumbnailURL
		}
		out = append(out, addonSearchResult{Provider: "curseforge", ProjectID: strconv.FormatInt(hit.ID, 10), Name: hit.Name, Description: hit.Summary, IconURL: icon, Downloads: hit.DownloadCount})
	}
	return out, nil
}
func (a *app) resolveCurseForge(ctx context.Context, projectID, versionID string, item server) ([]resolvedAddonFile, error) {
	if a.cfg.CurseForgeAPIKey == "" {
		return nil, errors.New("CurseForge provider is not configured")
	}
	id, err := strconv.ParseInt(projectID, 10, 64)
	if err != nil {
		return nil, errors.New("invalid CurseForge project")
	}
	fileQuery := url.Values{"gameVersion": {item.Version}, "pageSize": {"50"}}
	if item.Runtime == "fabric" {
		fileQuery.Set("modLoaderType", "4")
	}
	endpoint := fmt.Sprintf("https://api.curseforge.com/v1/mods/%d/files?", id) + fileQuery.Encode()
	var response struct {
		Data []struct {
			ID          int64  `json:"id"`
			DisplayName string `json:"displayName"`
			FileName    string `json:"fileName"`
			DownloadURL string `json:"downloadUrl"`
			Hashes      []struct {
				Value string `json:"value"`
				Algo  int    `json:"algo"`
			} `json:"hashes"`
		} `json:"data"`
	}
	if err := a.providerJSON(ctx, endpoint, a.cfg.CurseForgeAPIKey, &response); err != nil {
		return nil, err
	}
	if len(response.Data) == 0 {
		return nil, errors.New("no compatible CurseForge file found")
	}
	file := response.Data[0]
	if versionID != "" {
		for _, candidate := range response.Data {
			if strconv.FormatInt(candidate.ID, 10) == versionID {
				file = candidate
				break
			}
		}
	}
	hash := ""
	for _, candidate := range file.Hashes {
		if candidate.Algo == 1 {
			hash = candidate.Value
			break
		}
	}
	if file.DownloadURL == "" || hash == "" {
		return nil, errors.New("CurseForge file cannot be distributed or has no SHA-1 hash")
	}
	return []resolvedAddonFile{{URL: file.DownloadURL, FileName: file.FileName, Directory: addonDirectory(item.Runtime), Hash: hash, HashType: "sha1", ProjectID: projectID, VersionID: strconv.FormatInt(file.ID, 10), Name: file.DisplayName}}, nil
}
func (a *app) resolveAddon(ctx context.Context, provider, projectID, versionID string, item server) ([]resolvedAddonFile, error) {
	if projectID == "" || len(projectID) > 80 {
		return nil, errors.New("invalid project ID")
	}
	if provider == "modrinth" {
		return a.resolveModrinth(ctx, projectID, versionID, item)
	}
	if provider == "curseforge" {
		return a.resolveCurseForge(ctx, projectID, versionID, item)
	}
	return nil, errors.New("unsupported add-on provider")
}
func addonDirectory(runtime string) string {
	if runtime == "paper" || runtime == "purpur" {
		return "plugins"
	}
	return "mods"
}
func addonRuntimeSupported(runtime string) bool {
	return runtime == "paper" || runtime == "purpur" || runtime == "fabric"
}
func (a *app) providerJSON(ctx context.Context, endpoint, key string, out any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "MyPanel-V2/0.1 (self-hosted Minecraft panel)")
	if key != "" {
		request.Header.Set("x-api-key", key)
	}
	client := http.Client{Timeout: 12 * time.Second, CheckRedirect: func(next *http.Request, via []*http.Request) error {
		if len(via) > 3 || (next.URL.Hostname() != "api.modrinth.com" && next.URL.Hostname() != "api.curseforge.com") {
			return errors.New("unsafe provider redirect")
		}
		return nil
	}}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("provider returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}
