package dockerapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestContainerSpecHasPersistentDataAndHeadroom(t *testing.T) {
	spec := Spec{ID: "2d86389b-f055-4b80-9d8b-daeb83f5fa15", Runtime: "paper", Version: "1.21.4", Image: "itzg/minecraft-server:java25", MemoryMB: 2048, CPU: 2, BindIP: "192.168.56.101", Port: 25600, DataPath: "/var/lib/mypanel/servers/id", Config: map[string]any{"motd": "Hello"}}
	value := ContainerSpec("itzg/minecraft-server:java21", spec)
	host := value["HostConfig"].(map[string]any)
	if got := host["Memory"]; got != int64(2048*1024*1024) {
		t.Fatalf("memory = %v", got)
	}
	if got := host["Binds"].([]string)[0]; got != "/var/lib/mypanel/servers/id:/data" {
		t.Fatalf("bind = %q", got)
	}
	if got := host["CapDrop"].([]string); len(got) != 1 || got[0] != "ALL" {
		t.Fatalf("capability drop = %v", got)
	}
	capabilities := host["CapAdd"].([]string)
	if got, want := strings.Join(capabilities, ","), "CHOWN,SETGID,SETUID"; got != want {
		t.Fatalf("capability add = %q, want %q", got, want)
	}
	env := value["Env"].([]string)
	foundHeap := false
	foundPipe := false
	foundTerminal := false
	for _, item := range env {
		if item == "MEMORY=1638M" {
			foundHeap = true
		}
		if item == "CREATE_CONSOLE_IN_PIPE=true" {
			foundPipe = true
		}
		if item == "TERM=xterm-256color" {
			foundTerminal = true
		}
	}
	if !foundHeap || !foundPipe || !foundTerminal {
		t.Fatalf("required runtime environment missing: %v", env)
	}
	if value["OpenStdin"] != true {
		t.Fatal("interactive console input was not enabled")
	}
}

func TestContainerSpecAppliesValidatedStartupOptions(t *testing.T) {
	spec := Spec{ID: "id", Runtime: "paper", Version: "1.21.11", Image: "itzg/minecraft-server:java25", MemoryMB: 2048, CPU: 2, DataPath: "/data", Config: map[string]any{"jvmOpts": "-XX:+UseG1GC", "extraArgs": "nogui"}}
	env := ContainerSpec(spec.Image, spec)["Env"].([]string)
	joined := strings.Join(env, "\n")
	for _, expected := range []string{"JVM_OPTS=-Dterminal.ansi=true -Dnet.kyori.ansi.colorLevel=truecolor -XX:+UseG1GC", "EXTRA_ARGS=nogui", "ENABLE_RCON=false"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("%s missing from %v", expected, env)
		}
	}
}

func TestCalculateStatsUsesCgroupV2InactiveFileAndHostCPU(t *testing.T) {
	var stats dockerStats
	stats.MemoryStats.Usage = 800
	stats.MemoryStats.Stats = map[string]uint64{"inactive_file": 300, "cache": 100}
	stats.CPUStats.CPUUsage.TotalUsage = 300
	stats.PreCPUStats.CPUUsage.TotalUsage = 100
	stats.CPUStats.SystemCPUUsage = 1100
	stats.PreCPUStats.SystemCPUUsage = 100
	stats.CPUStats.OnlineCPUs = 8
	result := calculateStats(stats)
	if result.MemoryBytes != 500 {
		t.Fatalf("memory = %d, want 500", result.MemoryBytes)
	}
	if result.CPUPercent != 160 {
		t.Fatalf("host CPU percent = %v, want 160", result.CPUPercent)
	}
}

func TestObservedContainerStateWaitsForHealthyMinecraft(t *testing.T) {
	for _, test := range []struct {
		name    string
		running bool
		status  string
		health  string
		want    string
	}{
		{"booting", true, "running", "starting", "starting"},
		{"unhealthy", true, "running", "unhealthy", "starting"},
		{"ready", true, "running", "healthy", "running"},
		{"legacy image", true, "running", "", "running"},
		{"stopped", false, "exited", "", "offline"},
		{"dead", false, "dead", "", "error"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := observedContainerState(test.running, test.status, test.health); got != test.want {
				t.Fatalf("observedContainerState() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestContainerFailureReasonIsActionableAndSafe(t *testing.T) {
	for _, test := range []struct {
		name               string
		running, oomKilled bool
		status             string
		exitCode           int
		runtimeError, want string
	}{
		{"running", true, false, "running", 0, "", ""},
		{"out of memory", false, true, "exited", 137, "", "process exceeded the server memory limit"},
		{"non-zero exit", false, false, "exited", 1, "", "process exited with code 1"},
		{"runtime failure is redacted", false, false, "dead", 0, "/private/docker.sock: permission denied", "container runtime reported a failure"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := containerFailureReason(test.running, test.status, test.oomKilled, test.exitCode, test.runtimeError); got != test.want {
				t.Fatalf("containerFailureReason() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestStatsWaitsForAnAccurateCPUSample(t *testing.T) {
	var path string
	client := &Client{http: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		path = request.URL.RequestURI()
		body := `{"memory_stats":{"usage":800,"stats":{"inactive_file":300}},"cpu_stats":{"cpu_usage":{"total_usage":300},"system_cpu_usage":1100,"online_cpus":8},"precpu_stats":{"cpu_usage":{"total_usage":100},"system_cpu_usage":100}}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}}
	result, err := client.stats(t.Context(), "server-id")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(path, "one-shot") || path != "/containers/mypanel-serverid/stats?stream=false" {
		t.Fatalf("stats request path = %q", path)
	}
	if result.CPUPercent != 160 {
		t.Fatalf("CPU percent = %v, want 160", result.CPUPercent)
	}
}

func TestCommandUsesConsolePipeWithoutRCON(t *testing.T) {
	var create map[string]any
	requests := 0
	client := &Client{http: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			if err := json.NewDecoder(request.Body).Decode(&create); err != nil {
				t.Fatal(err)
			}
			return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(`{"Id":"exec-id"}`)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}}
	if _, err := client.Command(t.Context(), "server-id", "say hello"); err != nil {
		t.Fatal(err)
	}
	command := create["Cmd"].([]any)
	if command[0] != "mc-send-to-console" || command[1] != "say hello" || create["User"] != "1000:1000" {
		t.Fatalf("exec configuration = %#v", create)
	}
}

func TestLogsDoesNotAddDockerTimestamps(t *testing.T) {
	var path string
	client := &Client{http: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		path = request.URL.RequestURI()
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("[06:59:03 INFO]: Done\n")), Header: make(http.Header)}, nil
	})}}
	logs, err := client.Logs(t.Context(), "server-id")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(path, "timestamps=0") {
		t.Fatalf("logs request path = %q", path)
	}
	if logs != "[06:59:03 INFO]: Done\n" {
		t.Fatalf("logs = %q", logs)
	}
}

func TestContainerSpecUsesSelectedJavaImage(t *testing.T) {
	spec := Spec{ID: "2d86389b-f055-4b80-9d8b-daeb83f5fa15", Runtime: "paper", Version: "1.21.11", Image: "itzg/minecraft-server:java25", MemoryMB: 2048, CPU: 2, BindIP: "0.0.0.0", Port: 25565, DataPath: "/data", Config: map[string]any{}}
	value := ContainerSpec(spec.Image, spec)
	if value["Image"] != "itzg/minecraft-server:java25" {
		t.Fatalf("image = %v", value["Image"])
	}
}

func TestProvisionPullsSelectedJavaImage(t *testing.T) {
	requests := make([]string, 0, 3)
	client := &Client{http: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests = append(requests, request.Method+" "+request.URL.RequestURI())
		status := http.StatusOK
		if len(requests) == 1 {
			status = http.StatusNotFound
		} else if len(requests) == 3 {
			status = http.StatusCreated
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}}
	spec := Spec{ID: "2d86389b-f055-4b80-9d8b-daeb83f5fa15", Runtime: "paper", Version: "1.21.11", Image: "itzg/minecraft-server:java25", MemoryMB: 2048, CPU: 2, BindIP: "0.0.0.0", Port: 25565, DataPath: "/data", Config: map[string]any{}}
	if err := client.Provision(t.Context(), spec); err != nil {
		t.Fatal(err)
	}
	if got, want := requests[1], "POST /images/create?fromImage=itzg%2Fminecraft-server%3Ajava25"; got != want {
		t.Fatalf("image pull request = %q, want %q", got, want)
	}
}

func TestNameUsesOnlyServerID(t *testing.T) {
	if got, want := Name("ab-cd"), "mypanel-abcd"; got != want {
		t.Fatalf("Name() = %q, want %q", got, want)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func TestRestartUsesDockerRestartEndpoint(t *testing.T) {
	var method, path string
	client := &Client{http: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		method, path = request.Method, request.URL.RequestURI()
		return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}}
	if err := client.Restart(t.Context(), "ab-cd"); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost || path != "/containers/mypanel-abcd/restart?t=30" {
		t.Fatalf("restart request = %s %s", method, path)
	}
}
