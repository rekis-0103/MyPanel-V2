package main

import "testing"

func TestCompatibleModpackVersionDerivesTrustedRuntime(t *testing.T) {
	file := curseForgeModpackFile{ID: 8171764, ModID: 1148445, IsAvailable: true, DisplayName: "All the Mods 11 0.0.22", FileName: "atm11.zip", ReleaseType: 2, FileStatus: 4, GameVersions: []string{"26.1.1", "NeoForge"}}
	version, ok := compatibleModpackVersion(file)
	if !ok {
		t.Fatal("compatible NeoForge modpack was rejected")
	}
	if version.FileID != "8171764" || version.Runtime != "neoforge" || version.MinecraftVersion != "26.1.1" || version.JavaVersion != 25 || version.ReleaseType != "beta" {
		t.Fatalf("derived version = %+v", version)
	}
}

func TestCompatibleModpackVersionRejectsUnsafeChoices(t *testing.T) {
	base := curseForgeModpackFile{ID: 1, ModID: 2, IsAvailable: true, DisplayName: "Pack", FileName: "pack.zip", ReleaseType: 1, FileStatus: 4, GameVersions: []string{"1.20.1", "Forge"}}
	for _, test := range []struct {
		name   string
		mutate func(*curseForgeModpackFile)
	}{
		{"server pack instead of manifest", func(file *curseForgeModpackFile) { file.IsServerPack = true }},
		{"fabric loader", func(file *curseForgeModpackFile) { file.GameVersions = []string{"1.20.1", "Fabric"} }},
		{"unsupported Minecraft", func(file *curseForgeModpackFile) { file.GameVersions = []string{"1.19.2", "Forge"} }},
		{"unavailable file", func(file *curseForgeModpackFile) { file.IsAvailable = false }},
		{"malware status", func(file *curseForgeModpackFile) { file.FileStatus = 6 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			file := base
			test.mutate(&file)
			if _, ok := compatibleModpackVersion(file); ok {
				t.Fatalf("unsafe file was accepted: %+v", file)
			}
		})
	}
}

func TestCurseForgeIdentifiersAndSlugsAreStrict(t *testing.T) {
	for _, value := range []string{"1", "8171764"} {
		if _, err := parseCurseForgeID(value); err != nil {
			t.Fatalf("valid ID %q rejected: %v", value, err)
		}
	}
	for _, value := range []string{"", "0", "-1", "1/2", "99999999999999999999"} {
		if _, err := parseCurseForgeID(value); err == nil {
			t.Fatalf("invalid ID %q accepted", value)
		}
	}
	for _, value := range []string{"all-the-mods-11", "pack2"} {
		if !validCurseForgeSlug(value) {
			t.Fatalf("valid slug %q rejected", value)
		}
	}
	for _, value := range []string{"", "-pack", "pack-", "Pack", "pack/path"} {
		if validCurseForgeSlug(value) {
			t.Fatalf("invalid slug %q accepted", value)
		}
	}
}

func TestCurseForgeProjectMustBeAModpack(t *testing.T) {
	valid := curseForgeModpackProject{ID: 1148445, ClassID: 4471, Slug: "all-the-mods-11"}
	if !validCurseForgeModpackProject(valid) {
		t.Fatal("valid modpack project was rejected")
	}
	for _, project := range []curseForgeModpackProject{
		{ID: 1148445, ClassID: 6, Slug: "a-mod-project"},
		{ID: 0, ClassID: 4471, Slug: "all-the-mods-11"},
		{ID: 1148445, ClassID: 4471, Slug: "Invalid Slug"},
	} {
		if validCurseForgeModpackProject(project) {
			t.Fatalf("non-modpack project was accepted: %+v", project)
		}
	}
}
