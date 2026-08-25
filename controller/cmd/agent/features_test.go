package main

import (
	"encoding/base64"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testServerID = "2d86389b-f055-4b80-9d8b-daeb83f5fa15"

func TestSafePathRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	for _, value := range []string{"../escape", "folder/../../escape", "/absolute", `folder\escape`} {
		if _, err := safePath(root, value, true); err == nil {
			t.Fatalf("unsafe path %q was accepted", value)
		}
	}
}

func TestFileWriteStaysWithinServerRoot(t *testing.T) {
	base := t.TempDir()
	a := &agent{cfg: config{DataRoot: filepath.Join(base, "servers"), MetaRoot: filepath.Join(base, "meta")}}
	if err := os.MkdirAll(a.serverPath(testServerID), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(a.cfg.MetaRoot, 0750); err != nil {
		t.Fatal(err)
	}
	if err := a.writeMetadata(serverSpec{ID: testServerID, DiskMB: 1024}); err != nil {
		t.Fatal(err)
	}
	body := `{"path":"config/server.properties","content":"` + base64.StdEncoding.EncodeToString([]byte("motd=hello")) + `","encoding":"base64"}`
	request := httptest.NewRequest(http.MethodPut, "/v1/servers/"+testServerID+"/files", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	a.files(recorder, request, testServerID)
	if recorder.Code != http.StatusOK {
		t.Fatalf("write status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	data, err := os.ReadFile(filepath.Join(a.serverPath(testServerID), "config", "server.properties"))
	if err != nil || string(data) != "motd=hello" {
		t.Fatalf("written data=%q err=%v", data, err)
	}

	request = httptest.NewRequest(http.MethodPut, "/v1/servers/"+testServerID+"/files", strings.NewReader(`{"path":"../escape","content":"eA==","encoding":"base64"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	a.files(recorder, request, testServerID)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("traversal status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestValidateAgentConfig(t *testing.T) {
	if err := validateAgentConfig(map[string]any{"maxPlayers": float64(20), "onlineMode": true, "jvmOpts": "-XX:+UseG1GC", "extraArgs": "nogui"}); err != nil {
		t.Fatalf("valid configuration rejected: %v", err)
	}
	if err := validateAgentConfig(map[string]any{"unsupported": "value"}); err == nil {
		t.Fatal("unsupported configuration was accepted")
	}
}

func TestStartupOptionsRejectShellControlCharacters(t *testing.T) {
	for _, value := range []string{"$(touch /tmp/x)", "nogui; stop", "@args.txt", "line\nbreak"} {
		if validStartupOption(value) {
			t.Fatalf("unsafe startup option %q was accepted", value)
		}
	}
}

func TestValidateSpecRejectsUnsupportedJava(t *testing.T) {
	valid := serverSpec{ID: testServerID, Runtime: "paper", Version: "1.21.11", JavaVersion: 25, MemoryMB: 2048, CPU: 2, DiskMB: 10240, BindIP: "0.0.0.0", Port: 25565, Config: map[string]any{}}
	if err := validateSpec(testServerID, valid); err != nil {
		t.Fatalf("Java 25 specification rejected: %v", err)
	}
	valid.JavaVersion = 24
	if err := validateSpec(testServerID, valid); err == nil {
		t.Fatal("unsupported Java version was accepted")
	}
}

func TestBoundedCPUPercentUsesConfiguredVCPULimit(t *testing.T) {
	if got := boundedCPUPercent(612.5, 2); got != 200 {
		t.Fatalf("bounded CPU = %v, want 200", got)
	}
	if got := boundedCPUPercent(87.25, 2); got != 87.25 {
		t.Fatalf("valid CPU = %v, want 87.25", got)
	}
	if got := boundedCPUPercent(-1, 2); got != 0 {
		t.Fatalf("negative CPU = %v, want 0", got)
	}
	if got := boundedCPUPercent(math.NaN(), 2); got != 0 {
		t.Fatalf("NaN CPU = %v, want 0", got)
	}
}
