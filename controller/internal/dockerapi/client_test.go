package dockerapi

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestContainerSpecHasPersistentDataAndHeadroom(t *testing.T) {
	spec := Spec{ID: "2d86389b-f055-4b80-9d8b-daeb83f5fa15", Runtime: "paper", Version: "1.21.4", MemoryMB: 2048, CPU: 2, BindIP: "192.168.56.101", Port: 25600, DataPath: "/var/lib/mypanel/servers/id", Config: map[string]any{"motd": "Hello"}}
	value := ContainerSpec("itzg/minecraft-server:java21", spec)
	host := value["HostConfig"].(map[string]any)
	if got := host["Memory"]; got != int64(2048*1024*1024) {
		t.Fatalf("memory = %v", got)
	}
	if got := host["Binds"].([]string)[0]; got != "/var/lib/mypanel/servers/id:/data" {
		t.Fatalf("bind = %q", got)
	}
	env := value["Env"].([]string)
	foundHeap := false
	for _, item := range env {
		if item == "MEMORY=1638M" {
			foundHeap = true
		}
	}
	if !foundHeap {
		t.Fatalf("heap headroom missing: %v", env)
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
