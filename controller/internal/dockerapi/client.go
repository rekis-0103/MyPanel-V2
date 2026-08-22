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
	ID       string
	Runtime  string
	Version  string
	Image    string
	MemoryMB int
	CPU      int
	BindIP   string
	Port     int
	DataPath string
	Config   map[string]any
}

type State struct {
	State       string  `json:"state"`
	CPUPercent  float64 `json:"cpuPercent"`
	MemoryBytes int64   `json:"memoryBytes"`
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
		State struct {
			Running bool   `json:"Running"`
			Status  string `json:"Status"`
		} `json:"State"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return State{}, err
	}
	state := "offline"
	if response.State.Running {
		state = "running"
	} else if response.State.Status == "restarting" {
		state = "starting"
	} else if response.State.Status == "dead" {
		state = "error"
	}
	out := State{State: state}
	if response.State.Running {
		metrics, err := c.stats(ctx, id)
		if err == nil {
			out.CPUPercent = metrics.CPUPercent
			out.MemoryBytes = metrics.MemoryBytes
		}
	}
	return out, nil
}

func (c *Client) stats(ctx context.Context, id string) (State, error) {
	status, body, err := c.request(ctx, http.MethodGet, "/containers/"+Name(id)+"/stats?stream=false&one-shot=true", nil)
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

func (c *Client) Logs(ctx context.Context, id string) (string, error) {
	status, body, err := c.request(ctx, http.MethodGet, "/containers/"+Name(id)+"/logs?stdout=1&stderr=1&tail=400&timestamps=1", nil)
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
	environment := []string{
		"EULA=TRUE", "TYPE=" + strings.ToUpper(spec.Runtime), "VERSION=" + spec.Version,
		"MEMORY=" + strconv.Itoa(heapMB) + "M", "ENABLE_RCON=false", "CREATE_CONSOLE_IN_PIPE=true",
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
	if value, ok := spec.Config["jvmOpts"]; ok && strings.TrimSpace(fmt.Sprint(value)) != "" {
		environment = append(environment, "JVM_OPTS="+strings.TrimSpace(fmt.Sprint(value)))
	}
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
			"Binds":         []string{spec.DataPath + ":/data"},
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
