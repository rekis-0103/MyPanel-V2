package main

import "testing"

func TestValidateServerConfig(t *testing.T) {
	valid := map[string]any{
		"motd": "Hello", "difficulty": "hard", "gamemode": "survival",
		"maxPlayers": float64(20), "viewDistance": float64(10),
		"simulationDistance": float64(8), "onlineMode": true,
		"whiteList": false, "whiteListPlayers": "Steve,Alex",
	}
	if err := validateServerConfig(valid); err != nil {
		t.Fatalf("valid configuration rejected: %v", err)
	}
	for name, input := range map[string]map[string]any{
		"unknown":      {"exec": "bad"},
		"fractional":   {"maxPlayers": 1.5},
		"out of range": {"viewDistance": float64(100)},
		"wrong type":   {"onlineMode": "true"},
		"newline":      {"motd": "hello\nworld"},
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
