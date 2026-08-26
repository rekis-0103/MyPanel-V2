package dockerapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	http *http.Client
}

type Spec struct {
	ID                   string
	Runtime              string
	Version              string
	Image                string
	MemoryMB             int
	CPU                  int
	BindIP               string
	Port                 int
	DataPath             string
	PerformancePatchPath string
	Config               map[string]any
}

type State struct {
	State       string  `json:"state"`
	Reason      string  `json:"reason,omitempty"`
	CPUPercent  float64 `json:"cpuPercent"`
	MemoryBytes int64   `json:"memoryBytes"`
	NanoCPUs    int64   `json:"-"`
}

type dockerStats struct {
	MemoryStats struct {
		Usage uint64            `json:"usage"`
		Stats map[string]uint64 `json:"stats"`
	} `json:"memory_stats"`
	CPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
		OnlineCPUs     uint32 `json:"online_cpus"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
}

func New(socket string) *Client {
	if socket == "" {
		socket = "/var/run/docker.sock"
	}
	return &Client{http: &http.Client{
		Timeout: 15 * time.Minute,
		Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", socket)
		}},
	}}
}

func (c *Client) Provision(ctx context.Context, spec Spec) error {
	if spec.Image == "" {
		return errors.New("Minecraft image is required")
	}
	status, _, err := c.request(ctx, http.MethodGet, "/containers/"+Name(spec.ID)+"/json", nil)
	if err != nil {
		return err
	}
	if status == http.StatusOK {
		if err := c.Stop(ctx, spec.ID); err != nil {
			return err
		}
		if err := c.Remove(ctx, spec.ID); err != nil {
			return err
		}
	} else if status != http.StatusNotFound {
		return fmt.Errorf("inspect managed container: Docker returned %d", status)
	}
	if err := c.pull(ctx, spec.Image); err != nil {
		return err
	}
	body, err := json.Marshal(ContainerSpec(spec.Image, spec))
	if err != nil {
		return err
	}
	status, response, err := c.request(ctx, http.MethodPost, "/containers/create?name="+url.QueryEscape(Name(spec.ID)), body)
	if err != nil {
		return err
	}
	if status != http.StatusCreated {
		return dockerError("create Minecraft container", status, response)
	}
	return nil
}

func (c *Client) Start(ctx context.Context, id string) error {
	status, body, err := c.request(ctx, http.MethodPost, "/containers/"+Name(id)+"/start", nil)
	if err != nil {
		return err
	}
	if status != http.StatusNoContent && status != http.StatusNotModified {
		return dockerError("start Minecraft container", status, body)
	}
	return nil
}

func (c *Client) Stop(ctx context.Context, id string) error {
	status, body, err := c.request(ctx, http.MethodPost, "/containers/"+Name(id)+"/stop?t=30", nil)
	if err != nil {
		return err
	}
	if status == http.StatusNotFound || status == http.StatusNotModified || status == http.StatusNoContent {
		return nil
	}
	return dockerError("stop Minecraft container", status, body)
}

func (c *Client) Restart(ctx context.Context, id string) error {
	status, body, err := c.request(ctx, http.MethodPost, "/containers/"+Name(id)+"/restart?t=30", nil)
	if err != nil {
		return err
	}
	if status != http.StatusNoContent {
		return dockerError("restart Minecraft container", status, body)
	}
	return nil
}

func (c *Client) SetCPU(ctx context.Context, id string, nanoCPUs int64) error {
	body, err := json.Marshal(map[string]int64{"NanoCPUs": nanoCPUs})
	if err != nil {
		return err
	}
	status, response, err := c.request(ctx, http.MethodPost, "/containers/"+Name(id)+"/update", body)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return dockerError("update Minecraft CPU limit", status, response)
	}
	return nil
}

func (c *Client) Remove(ctx context.Context, id string) error {
	status, body, err := c.request(ctx, http.MethodDelete, "/containers/"+Name(id)+"?force=1&v=0", nil)
	if err != nil {
		return err
	}
	if status == http.StatusNoContent || status == http.StatusNotFound {
		return nil
	}
	return dockerError("remove Minecraft container", status, body)
}

func (c *Client) State(ctx context.Context, id string) (State, error) {
	out, err := c.Readiness(ctx, id)
	if err != nil {
		return State{}, err
	}
	if out.State == "running" || out.State == "starting" {
		metrics, metricsErr := c.stats(ctx, id)
		if metricsErr == nil {
			out.CPUPercent = metrics.CPUPercent
			out.MemoryBytes = metrics.MemoryBytes
		}
	}
	return out, nil
}

// Readiness inspects lifecycle and health without waiting for Docker's CPU
// sampling endpoint. Callers that only need state transitions should use this.
func (c *Client) Readiness(ctx context.Context, id string) (State, error) {
	status, body, err := c.request(ctx, http.MethodGet, "/containers/"+Name(id)+"/json", nil)
	if err != nil {
		return State{}, err
	}
	if status == http.StatusNotFound {
		return State{State: "offline"}, nil
	}
	if status != http.StatusOK {
		return State{}, dockerError("inspect Minecraft container", status, body)
	}
	var response struct {
		HostConfig struct {
			NanoCPUs int64 `json:"NanoCpus"`
		} `json:"HostConfig"`
		State struct {
			Running   bool   `json:"Running"`
			Status    string `json:"Status"`
			StartedAt string `json:"StartedAt"`
			Error     string `json:"Error"`
			ExitCode  int    `json:"ExitCode"`
			OOMKilled bool   `json:"OOMKilled"`
			Health    *struct {
				Status string `json:"Status"`
			} `json:"Health"`
		} `json:"State"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return State{}, err
	}
	health := ""
	if response.State.Health != nil {
		health = response.State.Health.Status
	}
	state := observedContainerState(response.State.Running, response.State.Status, health)
	if state == "starting" {
		startedAt, parseErr := time.Parse(time.RFC3339Nano, response.State.StartedAt)
		if parseErr == nil {
			logs, logsErr := c.Logs(ctx, id, "", 200)
			if logsErr == nil && minecraftReadySince(logs, startedAt) {
				state = "running"
			}
		}
	}
	return State{State: state, Reason: containerFailureReason(response.State.Running, response.State.Status, response.State.OOMKilled, response.State.ExitCode, response.State.Error), NanoCPUs: response.HostConfig.NanoCPUs}, nil
}

func minecraftReadySince(logs string, startedAt time.Time) bool {
	for _, line := range strings.Split(logs, "\n") {
		prefix, output, found := strings.Cut(line, " ")
		if !found || !strings.Contains(output, "Done (") || !strings.Contains(output, "For help, type") {
			continue
		}
		timestamp, err := time.Parse(time.RFC3339Nano, prefix)
		if err == nil && !timestamp.Before(startedAt) {
			return true
		}
	}
	return false
}

func containerFailureReason(running bool, status string, oomKilled bool, exitCode int, runtimeError string) string {
	if running || status == "created" || status == "restarting" {
		return ""
	}
	if oomKilled {
		return "process exceeded the server memory limit"
	}
	if exitCode != 0 {
		return fmt.Sprintf("process exited with code %d", exitCode)
	}
	if status == "dead" || strings.TrimSpace(runtimeError) != "" {
		return "container runtime reported a failure"
	}
	return ""
}

func observedContainerState(running bool, status, health string) string {
	if running {
		if health != "" && health != "healthy" {
			return "starting"
		}
		return "running"
	}
	if status == "restarting" {
		return "starting"
	}
	if status == "dead" {
		return "error"
	}
	return "offline"
}

func (c *Client) stats(ctx context.Context, id string) (State, error) {
	status, body, err := c.request(ctx, http.MethodGet, "/containers/"+Name(id)+"/stats?stream=false", nil)
	if err != nil {
		return State{}, err
	}
	if status != http.StatusOK {
		return State{}, dockerError("read Minecraft stats", status, body)
	}
	var response dockerStats
	if err := json.Unmarshal(body, &response); err != nil {
		return State{}, err
	}
	return calculateStats(response), nil
}

func calculateStats(response dockerStats) State {
	memory := response.MemoryStats.Usage
	cache := response.MemoryStats.Stats["inactive_file"]
	if cache == 0 {
		cache = response.MemoryStats.Stats["cache"]
	}
	if memory > cache {
		memory -= cache
	}
	var percent float64
	var cpuDelta, systemDelta uint64
	if response.CPUStats.CPUUsage.TotalUsage >= response.PreCPUStats.CPUUsage.TotalUsage {
		cpuDelta = response.CPUStats.CPUUsage.TotalUsage - response.PreCPUStats.CPUUsage.TotalUsage
	}
	if response.CPUStats.SystemCPUUsage >= response.PreCPUStats.SystemCPUUsage {
		systemDelta = response.CPUStats.SystemCPUUsage - response.PreCPUStats.SystemCPUUsage
	}
	if systemDelta > 0 && cpuDelta > 0 {
		cpus := response.CPUStats.OnlineCPUs
		if cpus == 0 {
			cpus = 1
		}
		percent = float64(cpuDelta) / float64(systemDelta) * float64(cpus) * 100
	}
	return State{CPUPercent: percent, MemoryBytes: int64(memory)}
}

func (c *Client) Logs(ctx context.Context, id, since string, tail int) (string, error) {
	query := url.Values{"stdout": {"1"}, "stderr": {"1"}, "timestamps": {"1"}}
	if since != "" {
		cursor, err := time.Parse(time.RFC3339Nano, since)
		if err != nil {
			return "", fmt.Errorf("invalid log cursor: %w", err)
		}
		// The Engine API accepts Unix seconds here. Request the cursor's whole
		// second and let the controller filter the nanosecond timestamps so a
		// burst containing multiple lines in one second cannot be skipped.
		query.Set("since", strconv.FormatInt(cursor.Unix(), 10))
	}
	if tail > 0 {
		query.Set("tail", strconv.Itoa(tail))
	}
	status, body, err := c.request(ctx, http.MethodGet, "/containers/"+Name(id)+"/logs?"+query.Encode(), nil)
	if err != nil {
		return "", err
	}
	if status == http.StatusNotFound {
		return "", nil
	}
	if status != http.StatusOK {
		return "", dockerError("read Minecraft logs", status, body)
	}
	return string(body), nil
}

// FollowLogs keeps one Docker Engine connection open and yields log bytes as
// they are produced. The caller owns the returned body and must close it.
func (c *Client) FollowLogs(ctx context.Context, id, since string) (io.ReadCloser, error) {
	query := url.Values{"stdout": {"1"}, "stderr": {"1"}, "timestamps": {"1"}, "follow": {"1"}}
	if since != "" {
		cursor, err := time.Parse(time.RFC3339Nano, since)
		if err != nil {
			return nil, fmt.Errorf("invalid log cursor: %w", err)
		}
		query.Set("since", strconv.FormatInt(cursor.Unix(), 10))
	} else {
		query.Set("tail", "0")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/containers/"+Name(id)+"/logs?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	// Streaming requests are intentionally governed by ctx instead of the
	// regular client's finite timeout.
	streamClient := *c.http
	streamClient.Timeout = 0
	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return io.NopCloser(strings.NewReader("")), nil
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if readErr != nil {
			return nil, readErr
		}
		return nil, dockerError("follow Minecraft logs", resp.StatusCode, body)
	}
	return resp.Body, nil
}

func (c *Client) Command(ctx context.Context, id, command string) (string, error) {
	createBody, _ := json.Marshal(map[string]any{"AttachStdout": true, "AttachStderr": true, "Tty": true, "User": "1000:1000", "Cmd": []string{"mc-send-to-console", command}})
	status, body, err := c.request(ctx, http.MethodPost, "/containers/"+Name(id)+"/exec", createBody)
	if err != nil {
		return "", err
	}
	if status != http.StatusCreated {
		return "", dockerError("create command exec", status, body)
	}
	var created struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(body, &created); err != nil || created.ID == "" {
		return "", errors.New("Docker returned an invalid exec id")
	}
	startBody := []byte(`{"Detach":false,"Tty":true}`)
	status, body, err = c.request(ctx, http.MethodPost, "/exec/"+created.ID+"/start", startBody)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", dockerError("run command", status, body)
	}
	return strings.TrimSpace(string(body)), nil
}

func (c *Client) pull(ctx context.Context, image string) error {
	status, body, err := c.request(ctx, http.MethodPost, "/images/create?fromImage="+url.QueryEscape(image), nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return dockerError("pull Minecraft image", status, body)
	}
	return nil
}

func (c *Client) request(ctx context.Context, method, requestPath string, body []byte) (int, []byte, error) {
	var input io.Reader
	if body != nil {
		input = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://docker"+requestPath, input)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, data, nil
}

func Name(id string) string { return "mypanel-" + strings.ReplaceAll(id, "-", "") }

func ContainerSpec(image string, spec Spec) map[string]any {
	heapMB := max(768, spec.MemoryMB*80/100)
	binds := []string{spec.DataPath + ":/data"}
	environment := []string{
		"EULA=TRUE", "TYPE=" + strings.ToUpper(spec.Runtime), "VERSION=" + spec.Version,
		"MEMORY=" + strconv.Itoa(heapMB) + "M", "ENABLE_RCON=false", "CREATE_CONSOLE_IN_PIPE=true",
		"CONSOLE_IN_NAMED_PIPE=/data/.mypanel-console-in",
		"TERM=xterm-256color", "COLORTERM=truecolor",
	}
	if spec.PerformancePatchPath != "" {
		binds = append(binds, spec.PerformancePatchPath+":/mypanel/patches:ro")
		environment = append(environment, "PATCH_DEFINITIONS=/mypanel/patches")
	}
	configKeys := map[string]string{
		"motd": "MOTD", "difficulty": "DIFFICULTY", "gamemode": "MODE",
		"maxPlayers": "MAX_PLAYERS", "viewDistance": "VIEW_DISTANCE",
		"simulationDistance": "SIMULATION_DISTANCE", "onlineMode": "ONLINE_MODE",
		"whiteList": "ENABLE_WHITELIST", "whiteListPlayers": "WHITELIST",
	}
	for key, envName := range configKeys {
		if value, ok := spec.Config[key]; ok {
			environment = append(environment, envName+"="+fmt.Sprint(value))
		}
	}
	jvmOpts := "-Dterminal.ansi=true -Dnet.kyori.ansi.colorLevel=truecolor"
	if value, ok := spec.Config["jvmOpts"]; ok && strings.TrimSpace(fmt.Sprint(value)) != "" {
		jvmOpts += " " + strings.TrimSpace(fmt.Sprint(value))
	}
	environment = append(environment, "JVM_OPTS="+jvmOpts)
	if value, ok := spec.Config["extraArgs"]; ok && strings.TrimSpace(fmt.Sprint(value)) != "" {
		environment = append(environment, "EXTRA_ARGS="+strings.TrimSpace(fmt.Sprint(value)))
	}
	return map[string]any{
		"Image":        image,
		"Tty":          true,
		"OpenStdin":    true,
		"StdinOnce":    false,
		"Env":          environment,
		"ExposedPorts": map[string]any{"25565/tcp": map[string]any{}},
		"Labels":       map[string]string{"mypanel.managed": "true", "mypanel.server-id": spec.ID},
		"HostConfig": map[string]any{
			"Memory":        int64(spec.MemoryMB) * 1024 * 1024,
			"MemorySwap":    int64(spec.MemoryMB) * 1024 * 1024,
			"NanoCPUs":      int64(spec.CPU) * 1_000_000_000,
			"PidsLimit":     int64(512),
			"Binds":         binds,
			"PortBindings":  map[string]any{"25565/tcp": []map[string]string{{"HostIp": spec.BindIP, "HostPort": strconv.Itoa(spec.Port)}}},
			"RestartPolicy": map[string]string{"Name": "unless-stopped"},
			"SecurityOpt":   []string{"no-new-privileges:true"},
			"CapDrop":       []string{"ALL"},
			"CapAdd":        []string{"CHOWN", "SETGID", "SETUID"},
		},
	}
}

func dockerError(operation string, status int, body []byte) error {
	var response struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &response)
	if response.Message == "" {
		response.Message = http.StatusText(status)
	}
	if len(response.Message) > 300 {
		response.Message = response.Message[:300]
	}
	return fmt.Errorf("%s: %s", operation, response.Message)
}
