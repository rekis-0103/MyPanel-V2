package main

import "testing"

func TestCapacityCalculationsDoNotReturnNegativeAvailability(t *testing.T) {
	total := capacitySummary{CPU: 8, MemoryMB: 16000, DiskMB: 100000, Ports: 10}
	used := capacitySummary{CPU: 10, MemoryMB: 4000, DiskMB: 110000, Ports: 2}
	available := capacityAvailable(total, used)
	if available.CPU != 0 || available.MemoryMB != 12000 || available.DiskMB != 0 || available.Ports != 8 {
		t.Fatalf("unexpected availability: %#v", available)
	}
	if capacityFits(available, 1, 1024, 1024) {
		t.Fatal("package fit ignored exhausted CPU or disk capacity")
	}
}

func TestValidateCheckoutNormalizesInput(t *testing.T) {
	in := checkoutInput{
		PackageID:      "11111111-1111-4111-8111-111111111111",
		IdempotencyKey: "22222222-2222-4222-8222-222222222222",
		Name:           "  Survival  ",
		Runtime:        " PAPER ",
		Version:        " 1.21.4 ",
		JavaVersion:    25,
	}
	if err := validateCheckout(&in); err != nil {
		t.Fatalf("valid checkout rejected: %v", err)
	}
	if in.Name != "Survival" || in.Runtime != "paper" || in.Version != "1.21.4" || in.JavaVersion != 21 {
		t.Fatalf("checkout was not normalized: %#v", in)
	}
}

func TestValidateCheckoutDerivesJava25ForMinecraft26(t *testing.T) {
	in := checkoutInput{
		PackageID:      "11111111-1111-4111-8111-111111111111",
		IdempotencyKey: "22222222-2222-4222-8222-222222222222",
		Name:           "Latest",
		Runtime:        "paper",
		Version:        "26.2",
		JavaVersion:    21,
	}
	if err := validateCheckout(&in); err != nil {
		t.Fatalf("valid checkout rejected: %v", err)
	}
	if in.JavaVersion != 25 {
		t.Fatalf("Minecraft 26.x should require Java 25, got Java %d", in.JavaVersion)
	}
}

func TestHostingPackageValidationAndNormalization(t *testing.T) {
	p := hostingPackage{Slug: "  IRON ", Name: "  Iron  ", Description: "  Two GiB  ", PriceIDR: 59000, CPU: 1, MemoryMB: 2048, DiskMB: 20480}
	normalizePackage(&p)
	if err := validatePackage(p); err != nil {
		t.Fatalf("valid package rejected: %v", err)
	}
	if p.Slug != "iron" || p.Name != "Iron" || p.Description != "Two GiB" {
		t.Fatalf("package was not normalized: %#v", p)
	}
	if p.ThemeColor != "#3FB950" || p.Icon != "grass" {
		t.Fatalf("package presentation defaults were not applied: %#v", p)
	}
	p.MemoryMB = 512
	if validatePackage(p) == nil {
		t.Fatal("undersized package accepted")
	}
}

func TestHostingPackagePresentationValidation(t *testing.T) {
	p := hostingPackage{Slug: "custom", Name: "Custom", PriceIDR: 1000, CPU: 1, MemoryMB: 1024, DiskMB: 1024, ThemeColor: " #58c7df ", Icon: " DIAMOND ", IsPopular: true, Recommended: true}
	normalizePackage(&p)
	if err := validatePackage(p); err != nil {
		t.Fatalf("valid presentation rejected: %v", err)
	}
	if p.ThemeColor != "#58C7DF" || p.Icon != "diamond" {
		t.Fatalf("presentation was not normalized: %#v", p)
	}
	p.ThemeColor = "red; background:url(example)"
	if validatePackage(p) == nil {
		t.Fatal("unsafe theme color accepted")
	}
	p.ThemeColor = "#58C7DF"
	p.Icon = "https://example.com/tracker.svg"
	if validatePackage(p) == nil {
		t.Fatal("external icon URL accepted")
	}
}
