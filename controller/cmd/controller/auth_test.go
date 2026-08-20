package main

import (
	"net/http/httptest"
	"testing"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := hashPassword("a-correct-horse-battery-staple")
	if err != nil {
		t.Fatal(err)
	}
	if !verifyPassword(hash, "a-correct-horse-battery-staple") {
		t.Fatal("correct password was rejected")
	}
	if verifyPassword(hash, "a-wrong-password") {
		t.Fatal("wrong password was accepted")
	}
}

func TestPasswordHashRejectsWeakPassword(t *testing.T) {
	if _, err := hashPassword("short"); err == nil {
		t.Fatal("weak password was accepted")
	}
}

func TestPasswordVerificationRejectsUnsafeParameters(t *testing.T) {
	unsafe := "$argon2id$v=19$m=0,t=0,p=0$MDEyMzQ1Njc4OWFiY2RlZg$MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY"
	if verifyPassword(unsafe, "irrelevant-password") {
		t.Fatal("unsafe Argon2 parameters were accepted")
	}
}

func TestOriginValidation(t *testing.T) {
	a := &app{cfg: config{TrustedOrigin: "https://panel.example.test"}}
	request := httptest.NewRequest("POST", "https://panel.example.test/api", nil)
	request.Header.Set("Origin", "https://panel.example.test")
	if !a.validOrigin(request) {
		t.Fatal("trusted origin was rejected")
	}
	request.Header.Set("Origin", "https://attacker.example.test")
	if a.validOrigin(request) {
		t.Fatal("untrusted origin was accepted")
	}
}
