package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"time"
)

type requestIDKey struct{}

func middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := randomToken(12)
		if err != nil {
			id = "unavailable"
		}
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Cache-Control", "no-store")
		start := time.Now()
		next.ServeHTTP(w, r.WithContext(ctx))
		log.Printf("request id=%s method=%s path=%s duration=%s", id, r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

func requestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

func decode(w http.ResponseWriter, r *http.Request, value any) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		write(w, http.StatusUnsupportedMediaType, apiError{Error: "content type must be application/json", Code: "unsupported_media_type", RequestID: requestID(r.Context())})
		return errors.New("content type")
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		write(w, http.StatusBadRequest, apiError{Error: "invalid JSON body", Code: "invalid_json", RequestID: requestID(r.Context())})
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		write(w, http.StatusBadRequest, apiError{Error: "JSON body must contain one value", Code: "invalid_json", RequestID: requestID(r.Context())})
		return errors.New("trailing JSON")
	}
	return nil
}

func write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func method(w http.ResponseWriter) {
	write(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed", Code: "method_not_allowed"})
}

func internal(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("request failed id=%s error=%v", requestID(r.Context()), err)
	write(w, http.StatusInternalServerError, apiError{Error: "internal server error", Code: "internal", RequestID: requestID(r.Context())})
}

func notFound(w http.ResponseWriter, r *http.Request) {
	write(w, http.StatusNotFound, apiError{Error: "resource not found", Code: "not_found", RequestID: requestID(r.Context())})
}
