package main

import "testing"

func TestModrinthFileRequiresVerifiedPrimaryFile(t *testing.T) {
	version := modrinthVersion{ID: "version", ProjectID: "project", Name: "Plugin"}
	version.Files = append(version.Files, struct {
		URL      string            `json:"url"`
		Filename string            `json:"filename"`
		Primary  bool              `json:"primary"`
		Hashes   map[string]string `json:"hashes"`
	}{URL: "https://cdn.modrinth.com/data/file.jar", Filename: "file.jar", Primary: true, Hashes: map[string]string{"sha512": "abc"}})
	file, err := modrinthFile(version, "plugins")
	if err != nil || file.Directory != "plugins" || file.HashType != "sha512" {
		t.Fatalf("file=%+v err=%v", file, err)
	}
	version.Files[0].Hashes = nil
	if _, err := modrinthFile(version, "plugins"); err == nil {
		t.Fatal("file without checksum was accepted")
	}
}

func TestAddonRuntimeSupport(t *testing.T) {
	for _, runtime := range []string{"paper", "purpur", "fabric"} {
		if !addonRuntimeSupported(runtime) {
			t.Fatalf("expected runtime %q to support managed add-ons", runtime)
		}
	}
	for _, runtime := range []string{"vanilla", "unknown"} {
		if addonRuntimeSupported(runtime) {
			t.Fatalf("expected runtime %q to reject managed add-ons", runtime)
		}
	}
}
