package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeAcceptsJSONCharsetAndRejectsTrailingValue(t *testing.T) {
	request := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"one"}`))
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	recorder := httptest.NewRecorder()
	var input struct {
		Name string `json:"name"`
	}
	if err := decode(recorder, request, &input); err != nil || input.Name != "one" {
		t.Fatalf("valid JSON rejected: err=%v name=%q", err, input.Name)
	}

	request = httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"one"}{"name":"two"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	if err := decode(recorder, request, &input); err == nil || recorder.Code != 400 {
		t.Fatalf("trailing JSON accepted: err=%v status=%d", err, recorder.Code)
	}
}
