package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ASmbat/shortlink/internal/models"
	"github.com/ASmbat/shortlink/internal/service"
	"github.com/ASmbat/shortlink/internal/store"
)

func newTestHandler() *Handler {
	return New(service.New(store.NewMemoryStore()))
}

func TestCreate_Success(t *testing.T) {
	h := newTestHandler()
	body, _ := json.Marshal(models.CreateLinkRequest{LongURL: "https://example.com"})

	req := httptest.NewRequest(http.MethodPost, "/links", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var link models.Link
	if err := json.Unmarshal(rr.Body.Bytes(), &link); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	if link.Code == "" {
		t.Error("expected non-empty code")
	}
}

func TestCreate_InvalidURL(t *testing.T) {
	h := newTestHandler()
	body, _ := json.Marshal(models.CreateLinkRequest{LongURL: "not-a-url"})

	req := httptest.NewRequest(http.MethodPost, "/links", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCreate_BodyOverLimitIsRejected(t *testing.T) {
	h := newTestHandler()
	body, _ := json.Marshal(models.CreateLinkRequest{
		LongURL: "https://example.com/" + strings.Repeat("a", maxRequestBody),
	})

	req := httptest.NewRequest(http.MethodPost, "/links", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for an oversized body, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestRedirect_NotFound(t *testing.T) {
	h := newTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/doesnotexist", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestRedirect_Success(t *testing.T) {
	h := newTestHandler()

	createBody, _ := json.Marshal(models.CreateLinkRequest{LongURL: "https://example.com/target"})
	createReq := httptest.NewRequest(http.MethodPost, "/links", bytes.NewReader(createBody))
	createRR := httptest.NewRecorder()
	h.Routes().ServeHTTP(createRR, createReq)

	var link models.Link
	json.Unmarshal(createRR.Body.Bytes(), &link)

	req := httptest.NewRequest(http.MethodGet, "/"+link.Code, nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "https://example.com/target" {
		t.Errorf("unexpected redirect location: %s", loc)
	}
}

func TestHealthz(t *testing.T) {
	h := newTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}
