package main

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateServerConfig(t *testing.T) {
	valid := map[string]any{
		"motd": "Hello", "difficulty": "hard", "gamemode": "survival",
		"maxPlayers": float64(20), "viewDistance": float64(10),
		"simulationDistance": float64(8), "onlineMode": true,
		"whiteList": false, "whiteListPlayers": "Steve,Alex",
		"jvmOpts": "-XX:+UseG1GC", "extraArgs": "nogui",
	}
	if err := validateServerConfig(valid); err != nil {
		t.Fatalf("valid configuration rejected: %v", err)
	}
	for name, input := range map[string]map[string]any{
		"unknown":       {"exec": "bad"},
		"fractional":    {"maxPlayers": 1.5},
		"out of range":  {"viewDistance": float64(100)},
		"wrong type":    {"onlineMode": "true"},
		"newline":       {"motd": "hello\nworld"},
		"startup shell": {"jvmOpts": "$(touch /tmp/x)"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateServerConfig(input); err == nil {
				t.Fatal("unsafe configuration was accepted")
			}
		})
	}
}

func TestJavaVersionAllowlist(t *testing.T) {
	if got := normalizeJavaVersion(0); got != 21 {
		t.Fatalf("legacy default = %d, want 21", got)
	}
	for _, value := range []int{21, 25} {
		if !javaVersionOK(value) {
			t.Fatalf("Java %d should be supported", value)
		}
	}
	for _, value := range []int{0, 8, 17, 24, 26} {
		if javaVersionOK(value) {
			t.Fatalf("Java %d should be rejected", value)
		}
	}
}

func TestConsoleDeltaAppendsNewLinesWithoutRedrawing(t *testing.T) {
	previous := "first\nsecond\n"
	for name, test := range map[string]struct {
		current string
		want    string
		reset   bool
	}{
		"growing":  {"first\nsecond\nthird\nfourth\n", "third\nfourth\n", false},
		"rotated":  {"second\nthird\nfourth\n", "third\nfourth\n", false},
		"replaced": {"different\n", "different\n", true},
	} {
		t.Run(name, func(t *testing.T) {
			got, gotReset := consoleDelta(previous, test.current)
			if got != test.want || gotReset != test.reset {
				t.Fatalf("consoleDelta() = (%q, %v), want (%q, %v)", got, gotReset, test.want, test.reset)
			}
		})
	}
}

func TestInitialConsoleEventQueryUsesContiguousParameters(t *testing.T) {
	query, arguments := consoleEventQuery("server-id", 0, 100)
	if len(arguments) != 2 || !strings.Contains(query, "LIMIT $2") || strings.Contains(query, "$3") {
		t.Fatalf("initial console event query = %q args=%v", query, arguments)
	}
	query, arguments = consoleEventQuery("server-id", 41, 100)
	if len(arguments) != 3 || !strings.Contains(query, "id>$2") || !strings.Contains(query, "LIMIT $3") {
		t.Fatalf("incremental console event query = %q args=%v", query, arguments)
	}
}

func TestLifecycleCompletionWaitsForReadiness(t *testing.T) {
	if got := lifecycleCompletionState("start"); got != "running" {
		t.Fatalf("start completion state = %q", got)
	}
	if got := lifecycleCompletionState("restart"); got != "running" {
		t.Fatalf("restart completion state = %q", got)
	}
	if got := lifecycleCompletionState("stop"); got != "offline" {
		t.Fatalf("stop completion state = %q", got)
	}
}

func TestLifecycleMessagesAreActionableWithoutLeakingInternalErrors(t *testing.T) {
	if got := lifecycleActionMessage("start"); got != "Starting server..." {
		t.Fatalf("start message = %q", got)
	}
	if got := lifecycleActionMessage("restart"); got != "Restarting server..." {
		t.Fatalf("restart message = %q", got)
	}
	message := lifecycleFailureMessage("start", errors.New("open /private/docker.sock: permission denied"))
	if strings.Contains(message, "/private/docker.sock") || !strings.Contains(message, "node could not complete") {
		t.Fatalf("unsafe lifecycle message = %q", message)
	}
	if got := lifecycleFailureMessage("runtime", errors.New("process exited with code 137")); !strings.Contains(got, "process exited with code 137") {
		t.Fatalf("exit reason missing from %q", got)
	}
	message = lifecycleFailureMessage("runtime", errors.New("process exited with code 137 at /private/runtime/path"))
	if strings.Contains(message, "/private/runtime/path") {
		t.Fatalf("exit reason leaked internal path: %q", message)
	}
}

func TestConsoleCommandRequiresRunningState(t *testing.T) {
	for _, state := range []string{"", "offline", "starting", "stopping", "error"} {
		if consoleCommandReady(state) {
			t.Fatalf("command allowed while state is %q", state)
		}
	}
	if !consoleCommandReady("running") {
		t.Fatal("command rejected while server is running")
	}
}
