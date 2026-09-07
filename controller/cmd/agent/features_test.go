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

func TestManagedConsolePipePathIsReserved(t *testing.T) {
	if !reservedServerPath("./" + managedConsolePipe) {
		t.Fatal("managed console pipe was exposed to file operations")
	}
	if !reservedServerPath(managedRuntimeDir + "/paper-performance.json") {
		t.Fatal("managed runtime file was exposed to file operations")
	}
	if reservedServerPath("plugins/example.jar") {
		t.Fatal("ordinary server file was treated as reserved")
	}
}

func TestPrepareRuntimeFilesCreatesPaperOptimizationPatch(t *testing.T) {
	base := t.TempDir()
	a := &agent{cfg: config{DataRoot: filepath.Join(base, "servers")}}
	spec := serverSpec{ID: testServerID, Runtime: "purpur"}
	if err := os.MkdirAll(a.serverPath(testServerID), 0750); err != nil {
		t.Fatal(err)
	}
	path, err := a.prepareRuntimeFiles(spec)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(path, "paper-performance.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"$.environment.optimize-explosions"`) || !strings.Contains(string(data), `"value": true`) {
		t.Fatalf("unexpected performance patch: %s", data)
	}
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("untrusted replacement"), 0640); err != nil {
		t.Fatal(err)
	}
	if _, err := a.prepareRuntimeFiles(spec); err != nil {
		t.Fatalf("replace unsafe runtime path: %v", err)
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		t.Fatalf("managed runtime path was not restored as a directory: info=%v err=%v", info, err)
	}
	vanillaPath, err := a.prepareRuntimeFiles(serverSpec{ID: testServerID, Runtime: "vanilla"})
	if err != nil || vanillaPath != "" {
		t.Fatalf("vanilla patch path = %q, err = %v", vanillaPath, err)
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

func TestCreateFolderAndMoveStayWithinServerRoot(t *testing.T) {
	base := t.TempDir()
	a := &agent{cfg: config{DataRoot: filepath.Join(base, "servers")}}
	if err := os.MkdirAll(a.serverPath(testServerID), 0750); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/v1/servers/"+testServerID+"/files/folders", strings.NewReader(`{"path":"plugins/config"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	a.createFolder(recorder, request, testServerID)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create folder status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if info, err := os.Stat(filepath.Join(a.serverPath(testServerID), "plugins", "config")); err != nil || !info.IsDir() {
		t.Fatalf("created folder info=%v err=%v", info, err)
	}

	oldPath := filepath.Join(a.serverPath(testServerID), "plugins", "old.yml")
	if err := os.WriteFile(oldPath, []byte("enabled: true"), 0640); err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodPost, "/v1/servers/"+testServerID+"/files/move", strings.NewReader(`{"from":"plugins/old.yml","to":"plugins/config/new.yml"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	a.moveFile(recorder, request, testServerID)
	if recorder.Code != http.StatusOK {
		t.Fatalf("move status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if data, err := os.ReadFile(filepath.Join(a.serverPath(testServerID), "plugins", "config", "new.yml")); err != nil || string(data) != "enabled: true" {
		t.Fatalf("moved data=%q err=%v", data, err)
	}

	request = httptest.NewRequest(http.MethodPost, "/v1/servers/"+testServerID+"/files/move", strings.NewReader(`{"from":"plugins","to":"plugins/nested"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	a.moveFile(recorder, request, testServerID)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("nested move status=%d body=%s", recorder.Code, recorder.Body.String())
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

func TestCPUNanoLimitAllowsOnlyStartupBurst(t *testing.T) {
	if got := cpuNanoLimit(2, true); got != 2_500_000_000 {
		t.Fatalf("startup CPU = %d, want 2500000000", got)
	}
	if got := cpuNanoLimit(2, false); got != 2_000_000_000 {
		t.Fatalf("runtime CPU = %d, want 2000000000", got)
	}
}

func TestAddonDownloadAllowlistAndNames(t *testing.T) {
	for _, host := range []string{"cdn.modrinth.com", "mediafilez.forgecdn.net", "edge.forgecdn.net"} {
		if !allowedAddonHost(host) {
			t.Fatalf("expected %s to be allowed", host)
		}
	}
	for _, host := range []string{"modrinth.com.evil.test", "forgecdn.net.evil.test", "localhost"} {
		if allowedAddonHost(host) {
			t.Fatalf("unsafe host %s was allowed", host)
		}
	}
	if !validAddonName("SkinRestorer.jar") {
		t.Fatal("valid jar was rejected")
	}
	for _, name := range []string{"../plugin.jar", "plugin.sh", "plugins/plugin.jar", "x.jar\x00"} {
		if validAddonName(name) {
			t.Fatalf("unsafe add-on name %q was allowed", name)
		}
	}
	for _, path := range []string{"plugins/SkinRestorer.jar", "mods/fabric-api.jar"} {
		if !validManagedAddonPath(path) {
			t.Fatalf("expected managed path %q to be accepted", path)
		}
	}
	for _, path := range []string{"plugins/../server.properties", "plugins/nested/addon.jar", "config/addon.jar", "plugins/addon.txt", "/plugins/addon.jar"} {
		if validManagedAddonPath(path) {
			t.Fatalf("unsafe managed path %q was allowed", path)
		}
	}
}
