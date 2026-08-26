package dockerapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestContainerSpecHasPersistentDataAndHeadroom(t *testing.T) {
	spec := Spec{ID: "2d86389b-f055-4b80-9d8b-daeb83f5fa15", Runtime: "paper", Version: "1.21.4", Image: "itzg/minecraft-server:java25", MemoryMB: 2048, CPU: 2, BindIP: "192.168.56.101", Port: 25600, DataPath: "/var/lib/mypanel/servers/id", PerformancePatchPath: "/var/lib/mypanel/servers/id/.mypanel-runtime", Config: map[string]any{"motd": "Hello"}}
	value := ContainerSpec("itzg/minecraft-server:java21", spec)
	host := value["HostConfig"].(map[string]any)
	if got := host["Memory"]; got != int64(2048*1024*1024) {
		t.Fatalf("memory = %v", got)
	}
	if got := host["Binds"].([]string)[0]; got != "/var/lib/mypanel/servers/id:/data" {
		t.Fatalf("bind = %q", got)
	}
	if got := host["Binds"].([]string)[1]; got != "/var/lib/mypanel/servers/id/.mypanel-runtime:/mypanel/patches:ro" {
		t.Fatalf("performance patch bind = %q", got)
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
	foundPersistentPipe := false
	foundTerminal := false
	foundPerformancePatches := false
	for _, item := range env {
		if item == "MEMORY=1638M" {
			foundHeap = true
		}
		if item == "CREATE_CONSOLE_IN_PIPE=true" {
			foundPipe = true
		}
		if item == "CONSOLE_IN_NAMED_PIPE=/data/.mypanel-console-in" {
			foundPersistentPipe = true
		}
		if item == "TERM=xterm-256color" {
			foundTerminal = true
		}
		if item == "PATCH_DEFINITIONS=/mypanel/patches" {
			foundPerformancePatches = true
		}
	}
	if !foundHeap || !foundPipe || !foundPersistentPipe || !foundTerminal || !foundPerformancePatches {
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

func TestReadinessDoesNotWaitForDockerStats(t *testing.T) {
	requests := 0
	client := &Client{http: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		if strings.Contains(request.URL.Path, "/stats") {
			t.Fatal("readiness requested Docker stats")
		}
		body := `{"HostConfig":{"NanoCpus":2000000000},"State":{"Running":true,"Status":"running","Health":{"Status":"healthy"}}}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}}
	state, err := client.Readiness(t.Context(), "server-id")
	if err != nil {
		t.Fatal(err)
	}
	if state.State != "running" || state.NanoCPUs != 2_000_000_000 || requests != 1 {
		t.Fatalf("readiness = %#v after %d requests", state, requests)
	}
}

func TestReadinessUsesCurrentBootReadyMarkerBeforeHealthInterval(t *testing.T) {
	requests := 0
	client := &Client{http: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		if strings.HasSuffix(request.URL.Path, "/json") {
			body := `{"State":{"Running":true,"Status":"running","StartedAt":"2026-08-25T09:42:00.000000000Z","Health":{"Status":"starting"}}}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}
		if strings.HasSuffix(request.URL.Path, "/logs") {
			body := "2026-08-25T09:41:00.000000000Z [09:41:00 INFO]: Done (old)! For help, type \"help\"\n" +
				"2026-08-25T09:46:52.374374426Z [09:46:52 INFO]: Done (143.338s)! For help, type \"help\"\n"
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}
		t.Fatalf("unexpected request path %q", request.URL.Path)
		return nil, nil
	})}}
	state, err := client.Readiness(t.Context(), "server-id")
	if err != nil {
		t.Fatal(err)
	}
	if state.State != "running" || requests != 2 {
		t.Fatalf("readiness = %#v after %d requests", state, requests)
	}
}

func TestMinecraftReadyMarkerMustBelongToCurrentBoot(t *testing.T) {
	startedAt, _ := time.Parse(time.RFC3339Nano, "2026-08-25T09:42:00Z")
	logs := "2026-08-25T09:41:00Z [09:41:00 INFO]: Done (30s)! For help, type \"help\"\n"
	if minecraftReadySince(logs, startedAt) {
		t.Fatal("ready marker from a previous boot was accepted")
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

func TestLogsUsesTimestampCursorForIncrementalReads(t *testing.T) {
	var path string
	client := &Client{http: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		path = request.URL.RequestURI()
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("[06:59:03 INFO]: Done\n")), Header: make(http.Header)}, nil
	})}}
	since := "2026-08-25T08:46:46.123456789Z"
	logs, err := client.Logs(t.Context(), "server-id", since, 0)
	if err != nil {
		t.Fatal(err)
	}
	cursor, _ := time.Parse(time.RFC3339Nano, since)
	if !strings.Contains(path, "timestamps=1") || !strings.Contains(path, "since="+strconv.FormatInt(cursor.Unix(), 10)) || strings.Contains(path, "tail=") {
		t.Fatalf("logs request path = %q", path)
	}
	if logs != "[06:59:03 INFO]: Done\n" {
		t.Fatalf("logs = %q", logs)
	}
}

func TestLogsRejectsInvalidTimestampCursor(t *testing.T) {
	client := &Client{}
	if _, err := client.Logs(t.Context(), "server-id", "not-a-timestamp", 0); err == nil {
		t.Fatal("invalid timestamp cursor was accepted")
	}
}

func TestFollowLogsUsesPersistentDockerStream(t *testing.T) {
	var path string
	client := &Client{http: &http.Client{Timeout: time.Second, Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		path = request.URL.RequestURI()
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("live line\n")), Header: make(http.Header)}, nil
	})}}
	since := "2026-08-25T08:46:46.123456789Z"
	stream, err := client.FollowLogs(t.Context(), "server-id", since)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	data, err := io.ReadAll(stream)
	if err != nil {
		t.Fatal(err)
	}
	cursor, _ := time.Parse(time.RFC3339Nano, since)
	for _, expected := range []string{"follow=1", "stdout=1", "stderr=1", "timestamps=1", "since=" + strconv.FormatInt(cursor.Unix(), 10)} {
		if !strings.Contains(path, expected) {
			t.Fatalf("follow path %q is missing %q", path, expected)
		}
	}
	if strings.Contains(path, "tail=") {
		t.Fatalf("follow path %q must replay any cursor gap", path)
	}
	if string(data) != "live line\n" {
		t.Fatalf("stream data = %q", data)
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

func TestSetCPUUsesDockerUpdateEndpoint(t *testing.T) {
	var method, path string
	var body map[string]int64
	client := &Client{http: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		method, path = request.Method, request.URL.RequestURI()
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"Warnings":[]}`)), Header: make(http.Header)}, nil
	})}}
	if err := client.SetCPU(t.Context(), "ab-cd", 2_500_000_000); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost || path != "/containers/mypanel-abcd/update" || body["NanoCPUs"] != 2_500_000_000 {
		t.Fatalf("CPU update = %s %s %#v", method, path, body)
	}
}
