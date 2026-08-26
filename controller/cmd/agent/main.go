package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"mypanel/controller/internal/dockerapi"
)

type config struct {
	Addr       string
	TLSCert    string
	TLSKey     string
	TLSCA      string
	DataRoot   string
	BackupRoot string
	MetaRoot   string
	JavaImages map[int]string
	DockerSock string
}

type agent struct {
	cfg    config
	docker *dockerapi.Client
}

type serverSpec struct {
	ID          string         `json:"id"`
	Runtime     string         `json:"runtime"`
	Version     string         `json:"version"`
	JavaVersion int            `json:"javaVersion"`
	MemoryMB    int            `json:"memoryMb"`
	CPU         int            `json:"cpu"`
	DiskMB      int            `json:"diskMb"`
	BindIP      string         `json:"bindIp"`
	Port        int            `json:"port"`
	Config      map[string]any `json:"config"`
}

type apiError struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

var versionPattern = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z._+\-]{0,31}$`)
var errDiskLimit = errors.New("server disk limit exceeded")

const (
	managedConsolePipe = ".mypanel-console-in"
	managedRuntimeDir  = ".mypanel-runtime"
)

var paperPerformancePatch = []byte(`{
  "file": "/data/config/paper-world-defaults.yml",
  "ops": [
    {
      "$set": {
        "path": "$.environment.optimize-explosions",
        "value": true
      }
    }
  ]
}
`)

func main() {
	cfg := config{
		Addr: env("AGENT_ADDR", ":8081"), TLSCert: env("AGENT_TLS_CERT_FILE", "/run/mypanel-certs/agent.crt"),
		TLSKey: env("AGENT_TLS_KEY_FILE", "/run/mypanel-certs/agent.key"), TLSCA: env("AGENT_TLS_CA_FILE", "/run/mypanel-certs/ca.crt"),
		DataRoot: env("AGENT_DATA_ROOT", "/var/lib/mypanel/servers"), BackupRoot: env("AGENT_BACKUP_ROOT", "/var/lib/mypanel/backups"),
		MetaRoot: env("AGENT_META_ROOT", "/var/lib/mypanel/meta"), DockerSock: env("DOCKER_SOCKET", "/var/run/docker.sock"),
		JavaImages: map[int]string{
			21: env("MINECRAFT_IMAGE_JAVA_21", env("MINECRAFT_IMAGE", "itzg/minecraft-server:java21")),
			25: env("MINECRAFT_IMAGE_JAVA_25", "itzg/minecraft-server:java25"),
		},
	}
	if err := os.MkdirAll(cfg.DataRoot, 0750); err != nil {
		log.Fatalf("create data root: %v", err)
	}
	if err := os.MkdirAll(cfg.BackupRoot, 0750); err != nil {
		log.Fatalf("create backup root: %v", err)
	}
	if err := os.MkdirAll(cfg.MetaRoot, 0750); err != nil {
		log.Fatalf("create metadata root: %v", err)
	}
	tlsConfig, err := loadTLS(cfg)
	if err != nil {
		log.Fatalf("configure mTLS: %v", err)
	}
	a := &agent{cfg: cfg, docker: dockerapi.New(cfg.DockerSock)}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", a.health)
	mux.HandleFunc("/v1/servers/", a.server)
	server := &http.Server{Addr: cfg.Addr, Handler: security(mux), TLSConfig: tlsConfig,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second,
		WriteTimeout: 15 * time.Minute, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	log.Printf("agent listening addr=%s", cfg.Addr)
	log.Fatal(server.ListenAndServeTLS("", ""))
}

func loadTLS(cfg config) (*tls.Config, error) {
	certificate, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
	if err != nil {
		return nil, err
	}
	caPEM, err := os.ReadFile(cfg.TLSCA)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("CA file contains no certificate")
	}
	return &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate},
		ClientCAs: pool, ClientAuth: tls.RequireAndVerifyClientCert}, nil
}

func (a *agent) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	write(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *agent) server(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/servers/"), "/"), "/")
	if len(parts) < 2 || uuid.Validate(parts[0]) != nil {
		notFound(w)
		return
	}
	id, action := parts[0], parts[1]
	ctx := r.Context()
	switch action {
	case "provision":
		if r.Method != http.MethodPost {
			method(w)
			return
		}
		var input serverSpec
		if decode(w, r, &input) != nil {
			return
		}
		if err := validateSpec(id, input); err != nil {
			write(w, http.StatusBadRequest, apiError{Error: err.Error(), Code: "invalid_spec"})
			return
		}
		dataPath := a.serverPath(id)
		if err := os.MkdirAll(dataPath, 0750); err != nil {
			internal(w, err)
			return
		}
		if err := a.docker.Stop(ctx, id); err != nil {
			internal(w, err)
			return
		}
		performancePatchPath, err := a.prepareRuntimeFiles(input)
		if err != nil {
			internal(w, err)
			return
		}
		if err := a.writeMetadata(input); err != nil {
			internal(w, err)
			return
		}
		err = a.docker.Provision(ctx, dockerapi.Spec{ID: id, Runtime: input.Runtime, Version: input.Version,
			Image:    a.cfg.JavaImages[input.JavaVersion],
			MemoryMB: input.MemoryMB, CPU: input.CPU, BindIP: input.BindIP, Port: input.Port,
			DataPath: dataPath, PerformancePatchPath: performancePatchPath, Config: input.Config})
		if err != nil {
			internal(w, err)
			return
		}
		write(w, http.StatusOK, map[string]string{"state": "offline"})
	case "start", "stop", "restart":
		if r.Method != http.MethodPost {
			method(w)
			return
		}
		var err error
		if action != "stop" {
			err = a.checkDiskLimit(id)
		}
		if errors.Is(err, errDiskLimit) {
			write(w, http.StatusInsufficientStorage, apiError{Error: "server disk limit exceeded", Code: "disk_limit"})
			return
		} else if err != nil {
			internal(w, err)
			return
		} else if action == "start" {
			var spec serverSpec
			if spec, err = a.readMetadata(id); err == nil {
				_, err = a.prepareRuntimeFiles(spec)
			}
			if err == nil {
				if err = a.setServerCPU(ctx, id, true); err == nil {
					err = a.docker.Start(ctx, id)
				}
			}
		} else if action == "stop" {
			err = a.docker.Stop(ctx, id)
		} else {
			if err = a.docker.Stop(ctx, id); err == nil {
				var spec serverSpec
				if spec, err = a.readMetadata(id); err == nil {
					_, err = a.prepareRuntimeFiles(spec)
				}
				if err == nil {
					if err = a.setServerCPU(ctx, id, true); err == nil {
						err = a.docker.Start(ctx, id)
					}
				}
			}
		}
		if err != nil {
			internal(w, err)
			return
		}
		write(w, http.StatusOK, map[string]string{"state": map[bool]string{true: "offline", false: "starting"}[action == "stop"]})
	case "delete":
		if r.Method != http.MethodPost {
			method(w)
			return
		}
		var input struct {
			PurgeData bool `json:"purgeData"`
		}
		if decode(w, r, &input) != nil {
			return
		}
		if err := a.docker.Stop(ctx, id); err != nil {
			internal(w, err)
			return
		}
		if err := a.docker.Remove(ctx, id); err != nil {
			internal(w, err)
			return
		}
		if input.PurgeData {
			if err := os.RemoveAll(a.serverPath(id)); err != nil {
				internal(w, err)
				return
			}
		}
		_ = os.Remove(a.metadataPath(id))
		write(w, http.StatusOK, map[string]bool{"purged": input.PurgeData})
	case "state":
		if r.Method != http.MethodGet {
			method(w)
			return
		}
		includeMetrics := r.URL.Query().Get("metrics") != "false"
		var state dockerapi.State
		var err error
		if includeMetrics {
			state, err = a.docker.State(ctx, id)
		} else {
			state, err = a.docker.Readiness(ctx, id)
		}
		if err != nil {
			internal(w, err)
			return
		}
		var disk int64
		if includeMetrics {
			disk, _ = directorySize(a.serverPath(id))
		}
		if spec, metadataErr := a.readMetadata(id); metadataErr == nil {
			if runtimeLimit := cpuNanoLimit(spec.CPU, false); state.State == "running" && state.NanoCPUs != runtimeLimit {
				if err := a.docker.SetCPU(ctx, id, runtimeLimit); err != nil {
					internal(w, err)
					return
				}
			}
			if includeMetrics {
				state.CPUPercent = boundedCPUPercent(state.CPUPercent, spec.CPU)
			}
			if includeMetrics && disk > int64(spec.DiskMB)*1024*1024 {
				_ = a.docker.Stop(ctx, id)
				state.State = "error"
				state.Reason = "server disk limit exceeded"
			}
		}
		write(w, http.StatusOK, map[string]any{"state": state.State, "cpuPercent": state.CPUPercent,
			"reason": state.Reason, "memoryBytes": state.MemoryBytes, "diskBytes": disk, "players": 0})
	case "logs":
		if r.Method != http.MethodGet {
			method(w)
			return
		}
		since := strings.TrimSpace(r.URL.Query().Get("since"))
		if since != "" {
			if _, err := time.Parse(time.RFC3339Nano, since); err != nil {
				write(w, http.StatusBadRequest, apiError{Error: "invalid log cursor", Code: "invalid_cursor"})
				return
			}
		}
		tail := 1000
		if rawTail := r.URL.Query().Get("tail"); rawTail != "" {
			value, err := strconv.Atoi(rawTail)
			if err != nil || value < 0 || value > 2000 {
				write(w, http.StatusBadRequest, apiError{Error: "invalid log tail", Code: "invalid_tail"})
				return
			}
			tail = value
		}
		if r.URL.Query().Get("follow") == "true" {
			stream, err := a.docker.FollowLogs(ctx, id, since)
			if err != nil {
				internal(w, err)
				return
			}
			defer stream.Close()
			_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.WriteHeader(http.StatusOK)
			flusher, _ := w.(http.Flusher)
			scanner := bufio.NewScanner(stream)
			scanner.Buffer(make([]byte, 64*1024), 1024*1024)
			for scanner.Scan() {
				if _, err := io.WriteString(w, scanner.Text()+"\n"); err != nil {
					return
				}
				if flusher != nil {
					flusher.Flush()
				}
			}
			return
		}
		logs, err := a.docker.Logs(ctx, id, since, tail)
		if err != nil {
			internal(w, err)
			return
		}
		write(w, http.StatusOK, map[string]string{"logs": logs})
	case "command":
		if r.Method != http.MethodPost {
			method(w)
			return
		}
		var input struct {
			Command string `json:"command"`
		}
		if decode(w, r, &input) != nil {
			return
		}
		input.Command = strings.TrimSpace(input.Command)
		if input.Command == "" || len(input.Command) > 512 || strings.ContainsAny(input.Command, "\r\n\x00") {
			write(w, http.StatusBadRequest, apiError{Error: "invalid command", Code: "invalid_command"})
			return
		}
		output, err := a.sendConsoleCommand(ctx, id, input.Command)
		if err != nil {
			internal(w, err)
			return
		}
		write(w, http.StatusOK, map[string]string{"output": output})
	default:
		a.feature(w, r, id, parts[1:])
	}
}

func (a *agent) sendConsoleCommand(ctx context.Context, id, command string) (string, error) {
	err := writeConsolePipe(filepath.Join(a.serverPath(id), managedConsolePipe), command)
	if errors.Is(err, os.ErrNotExist) {
		// Existing containers created before the persistent pipe setting remain
		// operable until their next configuration update recreates them.
		return a.docker.Command(ctx, id, command)
	}
	return "", err
}

func (a *agent) setServerCPU(ctx context.Context, id string, starting bool) error {
	spec, err := a.readMetadata(id)
	if err != nil {
		return err
	}
	return a.docker.SetCPU(ctx, id, cpuNanoLimit(spec.CPU, starting))
}

func (a *agent) prepareRuntimeFiles(spec serverSpec) (string, error) {
	if spec.Runtime != "paper" && spec.Runtime != "purpur" {
		return "", nil
	}
	root, err := os.OpenRoot(a.serverPath(spec.ID))
	if err != nil {
		return "", err
	}
	defer root.Close()
	if info, statErr := root.Lstat(managedRuntimeDir); statErr == nil && !info.IsDir() {
		if err := root.RemoveAll(managedRuntimeDir); err != nil {
			return "", err
		}
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return "", statErr
	}
	if err := root.MkdirAll(managedRuntimeDir, 0750); err != nil {
		return "", err
	}
	path := filepath.Join(managedRuntimeDir, "paper-performance.json")
	temporary := path + ".tmp"
	if err := root.WriteFile(temporary, paperPerformancePatch, 0640); err != nil {
		return "", err
	}
	if err := root.Rename(temporary, path); err != nil {
		_ = root.Remove(temporary)
		return "", err
	}
	return filepath.Join(a.serverPath(spec.ID), managedRuntimeDir), nil
}

func cpuNanoLimit(cpu int, starting bool) int64 {
	limit := int64(cpu) * 1_000_000_000
	if starting {
		return limit * 125 / 100
	}
	return limit
}

func boundedCPUPercent(value float64, cpu int) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || cpu <= 0 {
		return 0
	}
	limit := float64(cpu * 100)
	if value > limit {
		return limit
	}
	return value
}

func validateSpec(pathID string, input serverSpec) error {
	if input.ID != pathID || !map[string]bool{"vanilla": true, "paper": true, "purpur": true, "fabric": true, "forge": true, "neoforge": true}[input.Runtime] {
		return errors.New("server identity or runtime is invalid")
	}
	if !versionPattern.MatchString(input.Version) || (input.JavaVersion != 21 && input.JavaVersion != 25) || input.MemoryMB < 1024 || input.MemoryMB > 8192 || input.CPU < 1 || input.CPU > 64 || input.DiskMB < 1024 || input.DiskMB > 102400 || input.Port < 1 || input.Port > 65535 {
		return errors.New("server resource specification is invalid")
	}
	if input.BindIP == "" || strings.ContainsAny(input.BindIP, "/\\\x00") {
		return errors.New("bind IP is invalid")
	}
	if err := validateAgentConfig(input.Config); err != nil {
		return err
	}
	return nil
}

func validateAgentConfig(input map[string]any) error {
	for key, value := range input {
		switch key {
		case "motd":
			text, ok := value.(string)
			if !ok || len(text) > 160 || strings.ContainsAny(text, "\r\n\x00") {
				return errors.New("motd is invalid")
			}
		case "difficulty":
			text, ok := value.(string)
			if !ok || !map[string]bool{"peaceful": true, "easy": true, "normal": true, "hard": true}[text] {
				return errors.New("difficulty is invalid")
			}
		case "gamemode":
			text, ok := value.(string)
			if !ok || !map[string]bool{"survival": true, "creative": true, "adventure": true, "spectator": true}[text] {
				return errors.New("gamemode is invalid")
			}
		case "maxPlayers":
			if !numericInRange(value, 1, 1000) {
				return errors.New("maxPlayers is invalid")
			}
		case "viewDistance", "simulationDistance":
			if !numericInRange(value, 2, 32) {
				return errors.New(key + " is invalid")
			}
		case "onlineMode", "whiteList":
			if _, ok := value.(bool); !ok {
				return errors.New(key + " is invalid")
			}
		case "whiteListPlayers":
			text, ok := value.(string)
			if !ok || len(text) > 1024 || strings.ContainsAny(text, "\r\n\x00") {
				return errors.New("whiteListPlayers is invalid")
			}
		case "jvmOpts", "extraArgs":
			text, ok := value.(string)
			if !ok || !validStartupOption(text) {
				return errors.New(key + " contains unsupported startup characters")
			}
		default:
			return errors.New("unsupported server configuration")
		}
	}
	return nil
}

var startupOptionPattern = regexp.MustCompile(`^[0-9A-Za-z._:/=,+%\- ]*$`)

func validStartupOption(value string) bool {
	return len(value) <= 512 && startupOptionPattern.MatchString(value)
}

func numericInRange(value any, minimum, maximum int) bool {
	number, ok := value.(float64)
	return ok && number == float64(int(number)) && number >= float64(minimum) && number <= float64(maximum)
}

func (a *agent) checkDiskLimit(id string) error {
	spec, err := a.readMetadata(id)
	if err != nil {
		return err
	}
	used, err := directorySize(a.serverPath(id))
	if err != nil {
		return err
	}
	if used > int64(spec.DiskMB)*1024*1024 {
		return errDiskLimit
	}
	return nil
}

func (a *agent) serverPath(id string) string {
	return filepath.Join(a.cfg.DataRoot, id)
}

func (a *agent) metadataPath(id string) string {
	return filepath.Join(a.cfg.MetaRoot, id+".json")
}

func (a *agent) writeMetadata(spec serverSpec) error {
	data, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	temporary := a.metadataPath(spec.ID) + ".tmp"
	if err := os.WriteFile(temporary, data, 0600); err != nil {
		return err
	}
	return os.Rename(temporary, a.metadataPath(spec.ID))
}

func (a *agent) readMetadata(id string) (serverSpec, error) {
	data, err := os.ReadFile(a.metadataPath(id))
	if err != nil {
		return serverSpec{}, err
	}
	var spec serverSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return serverSpec{}, err
	}
	if spec.ID != id {
		return serverSpec{}, errors.New("metadata identity mismatch")
	}
	return spec, nil
}

func directorySize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		relative, relativeErr := filepath.Rel(root, path)
		if relativeErr != nil {
			return relativeErr
		}
		if entry.IsDir() && relative == managedRuntimeDir {
			return filepath.SkipDir
		}
		if entry.Type().IsRegular() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			total += info.Size()
		}
		return nil
	})
	return total, err
}

func decode(w http.ResponseWriter, r *http.Request, output any) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		write(w, http.StatusUnsupportedMediaType, apiError{Error: "content type must be application/json", Code: "unsupported_media_type"})
		return errors.New("content type")
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		write(w, http.StatusBadRequest, apiError{Error: "invalid JSON body", Code: "invalid_json"})
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		write(w, http.StatusBadRequest, apiError{Error: "JSON body must contain one value", Code: "invalid_json"})
		return errors.New("trailing JSON")
	}
	return nil
}

func write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func method(w http.ResponseWriter) {
	write(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed", Code: "method_not_allowed"})
}
func notFound(w http.ResponseWriter) {
	write(w, http.StatusNotFound, apiError{Error: "resource not found", Code: "not_found"})
}
func internal(w http.ResponseWriter, err error) {
	log.Printf("agent operation failed error=%v", err)
	write(w, http.StatusBadGateway, apiError{Error: "node operation failed", Code: "node_operation_failed"})
}
func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
