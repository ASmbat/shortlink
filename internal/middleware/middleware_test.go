package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	return &buf
}

func TestLogging_RecordsStatus(t *testing.T) {
	buf := captureLog(t)
	h := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))

	if !strings.Contains(buf.String(), "GET /x -> 418") {
		t.Fatalf("unexpected log line: %q", buf.String())
	}
}

func TestLogging_DefaultsTo200(t *testing.T) {
	buf := captureLog(t)
	h := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok")) // no explicit WriteHeader
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/y", nil))

	if !strings.Contains(buf.String(), "GET /y -> 200") {
		t.Fatalf("unexpected log line: %q", buf.String())
	}
}

func TestRecover_TurnsPanicInto500(t *testing.T) {
	captureLog(t)
	h := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestRecover_DoesNotWriteTwiceAfterResponseStarted(t *testing.T) {
	captureLog(t)
	h := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("partial"))
		panic("late boom")
	}))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	if rr.Code != http.StatusAccepted || rr.Body.String() != "partial" {
		t.Fatalf("response was changed after it started: %d %q", rr.Code, rr.Body.String())
	}
}

func TestRecover_ReRaisesErrAbortHandler(t *testing.T) {
	h := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))
	defer func() {
		if r := recover(); r != http.ErrAbortHandler {
			t.Fatalf("expected ErrAbortHandler to propagate, got %v", r)
		}
	}()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestRecoverAndLogging_LogTheRealStatus(t *testing.T) {
	buf := captureLog(t)
	h := Recover(Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/z", nil))
	if !strings.Contains(buf.String(), "GET /z -> 404") {
		t.Fatalf("unexpected log: %q", buf.String())
	}
}

func TestRecorder_Unwrap(t *testing.T) {
	rr := httptest.NewRecorder()
	if newRecorder(rr).Unwrap() != rr {
		t.Fatal("Unwrap should return the original writer")
	}
}
