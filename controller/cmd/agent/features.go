package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type fileEntry struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Type      string    `json:"type"`
	SizeBytes int64     `json:"sizeBytes"`
	Modified  time.Time `json:"modified"`
}

type backupResult struct {
	BackupID       string `json:"backupId"`
	SizeBytes      int64  `json:"sizeBytes"`
	ChecksumSHA256 string `json:"checksumSha256"`
}

func (a *agent) feature(w http.ResponseWriter, r *http.Request, serverID string, parts []string) {
	if len(parts) == 0 {
		notFound(w)
		return
	}
	switch parts[0] {
	case "files":
		a.files(w, r, serverID)
	case "backup":
		a.createBackup(w, r, serverID)
	case "restore":
		a.restoreBackup(w, r, serverID)
	case "backups":
		if len(parts) != 2 || uuid.Validate(parts[1]) != nil || r.Method != http.MethodDelete {
			notFound(w)
			return
		}
		path := filepath.Join(a.cfg.BackupRoot, serverID, parts[1]+".tar.gz")
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			internal(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		notFound(w)
	}
}

func (a *agent) files(w http.ResponseWriter, r *http.Request, serverID string) {
	root := a.serverPath(serverID)
	rootFS, err := os.OpenRoot(root)
	if errors.Is(err, os.ErrNotExist) {
		notFound(w)
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	defer rootFS.Close()
	switch r.Method {
	case http.MethodGet:
		requested := r.URL.Query().Get("path")
		target, err := safePath(root, requested, true)
		if err != nil {
			write(w, http.StatusBadRequest, apiError{Error: err.Error(), Code: "unsafe_path"})
			return
		}
		relative, _ := filepath.Rel(root, target)
		info, err := rootFS.Lstat(relative)
		if errors.Is(err, os.ErrNotExist) {
			notFound(w)
			return
		}
		if err != nil {
			internal(w, err)
			return
		}
		if info.Mode()&os.ModeSymlink != 0 {
			write(w, http.StatusBadRequest, apiError{Error: "symbolic links are not accessible", Code: "unsafe_path"})
			return
		}
		if !info.IsDir() {
			if info.Size() > 10<<20 {
				write(w, http.StatusRequestEntityTooLarge, apiError{Error: "file is too large to open", Code: "file_too_large"})
				return
			}
			data, err := rootFS.ReadFile(relative)
			if err != nil {
				internal(w, err)
				return
			}
			write(w, http.StatusOK, map[string]any{"type": "file", "path": requested,
				"content": base64.StdEncoding.EncodeToString(data), "encoding": "base64", "sizeBytes": len(data)})
			return
		}
		directory, err := rootFS.Open(relative)
		if err != nil {
			internal(w, err)
			return
		}
		defer directory.Close()
		entries, err := directory.ReadDir(-1)
		if err != nil {
			internal(w, err)
			return
		}
		if len(entries) > 2000 {
			write(w, http.StatusRequestEntityTooLarge, apiError{Error: "directory contains too many entries", Code: "directory_too_large"})
			return
		}
		out := make([]fileEntry, 0, len(entries))
		for _, entry := range entries {
			entryInfo, err := entry.Info()
			if err != nil {
				continue
			}
			kind := "file"
			if entry.IsDir() {
				kind = "directory"
			} else if entry.Type()&os.ModeSymlink != 0 {
				kind = "symlink"
			}
			entryPath := filepath.ToSlash(filepath.Join(requested, entry.Name()))
			out = append(out, fileEntry{Name: entry.Name(), Path: strings.TrimPrefix(entryPath, "./"), Type: kind,
				SizeBytes: entryInfo.Size(), Modified: entryInfo.ModTime()})
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].Type != out[j].Type {
				return out[i].Type == "directory"
			}
			return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
		})
		write(w, http.StatusOK, map[string]any{"type": "directory", "path": requested, "entries": out})
	case http.MethodPut:
		var input struct {
			Path     string `json:"path"`
			Content  string `json:"content"`
			Encoding string `json:"encoding"`
		}
		if decode(w, r, &input) != nil {
			return
		}
		if input.Path == "" {
			write(w, http.StatusBadRequest, apiError{Error: "file path is required", Code: "invalid_file"})
			return
		}
		data := []byte(input.Content)
		if input.Encoding == "base64" {
			var err error
			data, err = base64.StdEncoding.DecodeString(input.Content)
			if err != nil {
				write(w, http.StatusBadRequest, apiError{Error: "invalid base64 content", Code: "invalid_file"})
				return
			}
		} else if input.Encoding != "" && input.Encoding != "utf8" {
			write(w, http.StatusBadRequest, apiError{Error: "unsupported file encoding", Code: "invalid_file"})
			return
		}
		if len(data) > 10<<20 {
			write(w, http.StatusRequestEntityTooLarge, apiError{Error: "file exceeds 10 MiB", Code: "file_too_large"})
			return
		}
		target, err := safePath(root, input.Path, true)
		if err != nil || target == root {
			write(w, http.StatusBadRequest, apiError{Error: "unsafe file path", Code: "unsafe_path"})
			return
		}
		spec, err := a.readMetadata(serverID)
		if err != nil {
			internal(w, err)
			return
		}
		used, err := directorySize(root)
		if err != nil {
			internal(w, err)
			return
		}
		relative, _ := filepath.Rel(root, target)
		var previous int64
		if info, err := rootFS.Lstat(relative); err == nil {
			if info.Mode()&os.ModeSymlink != 0 || info.IsDir() {
				write(w, http.StatusBadRequest, apiError{Error: "target is not a regular file", Code: "unsafe_path"})
				return
			}
			previous = info.Size()
		}
		if used-previous+int64(len(data)) > int64(spec.DiskMB)*1024*1024 {
			write(w, http.StatusInsufficientStorage, apiError{Error: "server disk limit exceeded", Code: "disk_limit"})
			return
		}
		if err := rootFS.MkdirAll(filepath.Dir(relative), 0750); err != nil {
			internal(w, err)
			return
		}
		temporary := relative + ".mypanel-upload"
		if err := rootFS.WriteFile(temporary, data, 0640); err != nil {
			internal(w, err)
			return
		}
		if err := rootFS.Rename(temporary, relative); err != nil {
			_ = rootFS.Remove(temporary)
			internal(w, err)
			return
		}
		write(w, http.StatusOK, map[string]any{"path": input.Path, "sizeBytes": len(data)})
	case http.MethodDelete:
		requested := r.URL.Query().Get("path")
		target, err := safePath(root, requested, false)
		if err != nil || target == root {
			write(w, http.StatusBadRequest, apiError{Error: "unsafe file path", Code: "unsafe_path"})
			return
		}
		relative, _ := filepath.Rel(root, target)
		if err := rootFS.RemoveAll(relative); err != nil {
			internal(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		method(w)
	}
}

func safePath(root, requested string, allowMissing bool) (string, error) {
	if strings.ContainsRune(requested, '\x00') || strings.Contains(requested, "\\") || strings.HasPrefix(requested, "/") || filepath.IsAbs(requested) {
		return "", errors.New("path must be relative")
	}
	cleanSlash := path.Clean(requested)
	if cleanSlash == ".." || strings.HasPrefix(cleanSlash, "../") {
		return "", errors.New("path escapes server root")
	}
	clean := filepath.Clean(filepath.FromSlash(cleanSlash))
	if clean == "." {
		clean = ""
	}
	target := filepath.Join(root, clean)
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes server root")
	}
	current := root
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) && allowMissing {
			break
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("symbolic links are not accessible")
		}
	}
	return target, nil
}

func (a *agent) createBackup(w http.ResponseWriter, r *http.Request, serverID string) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var input struct {
		BackupID string `json:"backupId"`
	}
	if decode(w, r, &input) != nil {
		return
	}
	if uuid.Validate(input.BackupID) != nil {
		write(w, http.StatusBadRequest, apiError{Error: "invalid backup id", Code: "invalid_backup"})
		return
	}
	result, err := a.backup(r.Context(), serverID, input.BackupID)
	if err != nil {
		internal(w, err)
		return
	}
	write(w, http.StatusOK, result)
}

func (a *agent) backup(ctx context.Context, serverID, backupID string) (backupResult, error) {
	state, _ := a.docker.State(ctx, serverID)
	if state.State == "running" {
		_, _ = a.docker.Command(ctx, serverID, "save-off")
		_, _ = a.docker.Command(ctx, serverID, "save-all flush")
		defer a.docker.Command(context.Background(), serverID, "save-on")
	}
	backupDir := filepath.Join(a.cfg.BackupRoot, serverID)
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		return backupResult{}, err
	}
	finalPath := filepath.Join(backupDir, backupID+".tar.gz")
	temporary := finalPath + ".tmp"
	if err := os.Remove(temporary); err != nil && !errors.Is(err, os.ErrNotExist) {
		return backupResult{}, err
	}
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return backupResult{}, err
	}
	hasher := sha256.New()
	gz := gzip.NewWriter(io.MultiWriter(file, hasher))
	tarWriter := tar.NewWriter(gz)
	err = filepath.WalkDir(a.serverPath(serverID), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(a.serverPath(serverID), path)
		if err != nil || relative == "." {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relative)
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tarWriter, input)
		closeErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	closeErrors := []error{tarWriter.Close(), gz.Close(), file.Close()}
	if err == nil {
		for _, closeErr := range closeErrors {
			if closeErr != nil {
				err = closeErr
				break
			}
		}
	}
	if err != nil {
		_ = os.Remove(temporary)
		return backupResult{}, err
	}
	if err := os.Rename(temporary, finalPath); err != nil {
		_ = os.Remove(temporary)
		return backupResult{}, err
	}
	info, err := os.Stat(finalPath)
	if err != nil {
		return backupResult{}, err
	}
	return backupResult{BackupID: backupID, SizeBytes: info.Size(), ChecksumSHA256: hex.EncodeToString(hasher.Sum(nil))}, nil
}

func (a *agent) restoreBackup(w http.ResponseWriter, r *http.Request, serverID string) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var input struct {
		BackupID string `json:"backupId"`
	}
	if decode(w, r, &input) != nil {
		return
	}
	if uuid.Validate(input.BackupID) != nil {
		write(w, http.StatusBadRequest, apiError{Error: "invalid backup id", Code: "invalid_backup"})
		return
	}
	if err := a.restore(r.Context(), serverID, input.BackupID); err != nil {
		internal(w, err)
		return
	}
	write(w, http.StatusOK, map[string]bool{"restored": true})
}

func (a *agent) restore(ctx context.Context, serverID, backupID string) error {
	if err := a.docker.Stop(ctx, serverID); err != nil {
		return err
	}
	spec, err := a.readMetadata(serverID)
	if err != nil {
		return err
	}
	archivePath := filepath.Join(a.cfg.BackupRoot, serverID, backupID+".tar.gz")
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()
	temporary := a.serverPath(serverID) + ".restore-" + backupID
	previous := a.serverPath(serverID) + ".previous-" + backupID
	_ = os.RemoveAll(temporary)
	if err := os.MkdirAll(temporary, 0750); err != nil {
		return err
	}
	var extracted int64
	reader := tar.NewReader(gz)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			_ = os.RemoveAll(temporary)
			return err
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeDir {
			_ = os.RemoveAll(temporary)
			return errors.New("backup contains an unsupported entry type")
		}
		target, err := safePath(temporary, header.Name, true)
		if err != nil || target == temporary {
			_ = os.RemoveAll(temporary)
			return errors.New("backup contains an unsafe path")
		}
		if header.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(target, 0750); err != nil {
				_ = os.RemoveAll(temporary)
				return err
			}
			continue
		}
		extracted += header.Size
		if extracted > int64(spec.DiskMB)*1024*1024 {
			_ = os.RemoveAll(temporary)
			return errors.New("backup exceeds server disk limit")
		}
		if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
			_ = os.RemoveAll(temporary)
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0640)
		if err != nil {
			_ = os.RemoveAll(temporary)
			return err
		}
		_, copyErr := io.CopyN(output, reader, header.Size)
		closeErr := output.Close()
		if copyErr != nil || closeErr != nil {
			_ = os.RemoveAll(temporary)
			return errors.Join(copyErr, closeErr)
		}
	}
	_ = os.RemoveAll(previous)
	if err := os.Rename(a.serverPath(serverID), previous); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = os.RemoveAll(temporary)
		return err
	}
	if err := os.Rename(temporary, a.serverPath(serverID)); err != nil {
		_ = os.Rename(previous, a.serverPath(serverID))
		return err
	}
	_ = os.RemoveAll(previous)
	return nil
}
