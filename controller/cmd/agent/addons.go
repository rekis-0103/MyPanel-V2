package main

import (
	"context"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxAddonBytes = 128 << 20

type addonFileInput struct {
	URL       string `json:"url"`
	FileName  string `json:"fileName"`
	Directory string `json:"directory"`
	Hash      string `json:"hash"`
	HashType  string `json:"hashType"`
}

func (a *agent) addonOperation(w http.ResponseWriter, r *http.Request, serverID string, parts []string) {
	if len(parts) != 1 || r.Method != http.MethodPost {
		notFound(w)
		return
	}
	switch parts[0] {
	case "install":
		var input struct {
			Files []addonFileInput `json:"files"`
		}
		if decode(w, r, &input) != nil {
			return
		}
		if len(input.Files) == 0 || len(input.Files) > 16 {
			write(w, http.StatusBadRequest, apiError{Error: "invalid add-on files", Code: "invalid_addon"})
			return
		}
		installed := make([]string, 0, len(input.Files))
		for _, file := range input.Files {
			managedPath, err := a.installAddonFile(r.Context(), serverID, file)
			if err != nil {
				internal(w, err)
				return
			}
			installed = append(installed, managedPath)
		}
		write(w, http.StatusOK, map[string]any{"installed": installed})
	case "remove":
		var input struct {
			Path string `json:"path"`
		}
		if decode(w, r, &input) != nil {
			return
		}
		clean, err := safePath(a.serverPath(serverID), input.Path, false)
		if err != nil || !validManagedAddonPath(input.Path) {
			write(w, http.StatusBadRequest, apiError{Error: "invalid managed add-on path", Code: "invalid_addon"})
			return
		}
		root, err := os.OpenRoot(a.serverPath(serverID))
		if err != nil {
			internal(w, err)
			return
		}
		defer root.Close()
		rel, _ := filepath.Rel(a.serverPath(serverID), clean)
		if info, statErr := root.Lstat(rel); statErr == nil && info.Mode().IsRegular() {
			err = root.Remove(rel)
		} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			err = statErr
		}
		if err != nil {
			internal(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		notFound(w)
	}
}

func (a *agent) installAddonFile(ctx context.Context, serverID string, input addonFileInput) (string, error) {
	if !validAddonName(input.FileName) || (input.Directory != "plugins" && input.Directory != "mods") || len(input.Hash) < 32 {
		return "", errors.New("invalid add-on file metadata")
	}
	parsed, err := url.Parse(input.URL)
	if err != nil || parsed.Scheme != "https" || !allowedAddonHost(parsed.Hostname()) || parsed.User != nil {
		return "", errors.New("add-on download host is not allowed")
	}
	client := &http.Client{Timeout: 2 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 4 || req.URL.Scheme != "https" || !allowedAddonHost(req.URL.Hostname()) {
			return errors.New("unsafe add-on redirect")
		}
		return nil
	}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, input.URL, nil)
	if err != nil {
		return "", err
	}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("add-on download returned HTTP %d", response.StatusCode)
	}
	if response.ContentLength > maxAddonBytes {
		return "", errors.New("add-on exceeds 128 MiB limit")
	}
	root, err := os.OpenRoot(a.serverPath(serverID))
	if err != nil {
		return "", err
	}
	defer root.Close()
	if err := root.MkdirAll(input.Directory, 0750); err != nil {
		return "", err
	}
	temporary := filepath.Join(input.Directory, ".mypanel-"+input.FileName+".tmp")
	target := filepath.Join(input.Directory, input.FileName)
	file, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0640)
	if err != nil {
		return "", err
	}
	limited := io.LimitReader(response.Body, maxAddonBytes+1)
	var writer io.Writer = file
	var expected string
	var sum func() string
	if input.HashType == "sha512" {
		h := sha512.New()
		writer = io.MultiWriter(file, h)
		expected = strings.ToLower(input.Hash)
		sum = func() string { return hex.EncodeToString(h.Sum(nil)) }
	} else if input.HashType == "sha1" {
		h := sha1.New()
		writer = io.MultiWriter(file, h)
		expected = strings.ToLower(input.Hash)
		sum = func() string { return hex.EncodeToString(h.Sum(nil)) }
	} else {
		file.Close()
		_ = root.Remove(temporary)
		return "", errors.New("unsupported add-on hash")
	}
	written, copyErr := io.Copy(writer, limited)
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil || written > maxAddonBytes || sum() != expected {
		_ = root.Remove(temporary)
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		return "", errors.New("add-on checksum verification failed")
	}
	if info, statErr := root.Lstat(target); statErr == nil && !info.Mode().IsRegular() {
		_ = root.Remove(temporary)
		return "", errors.New("add-on target is not a regular file")
	} else if statErr == nil {
		_ = root.Remove(target)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		_ = root.Remove(temporary)
		return "", statErr
	}
	if err := root.Rename(temporary, target); err != nil {
		_ = root.Remove(temporary)
		return "", err
	}
	return filepath.ToSlash(target), nil
}

func validAddonName(value string) bool {
	return value == filepath.Base(value) && len(value) > 4 && len(value) <= 180 && (strings.HasSuffix(strings.ToLower(value), ".jar") || strings.HasSuffix(strings.ToLower(value), ".zip")) && !strings.ContainsRune(value, '\x00')
}
func validManagedAddonPath(value string) bool {
	parts := strings.Split(filepath.ToSlash(value), "/")
	return len(parts) == 2 && (parts[0] == "plugins" || parts[0] == "mods") && validAddonName(parts[1])
}
func allowedAddonHost(host string) bool {
	host = strings.ToLower(host)
	return host == "cdn.modrinth.com" || strings.HasSuffix(host, ".forgecdn.net")
}
