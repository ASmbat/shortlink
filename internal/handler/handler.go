package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ASmbat/shortlink/internal/models"
	"github.com/ASmbat/shortlink/internal/service"
	"github.com/ASmbat/shortlink/internal/store"
)

type Handler struct {
	svc *service.Service
}

func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

// Routes wires up the HTTP handlers onto a ServeMux.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /links", h.create)
	mux.HandleFunc("GET /{code}", h.redirect)
	mux.HandleFunc("GET /healthz", h.health)
	return mux
}

// maxRequestBody bounds the POST /links request body. Without this,
// json.Decoder reads the body to completion regardless of size, so a
// client could send an arbitrarily large body and have it held in memory
// (the long_url is also stored indefinitely absent a TTL) — a cheap
// memory-exhaustion vector.
const maxRequestBody = 1 << 20 // 1MB

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)

	var req models.CreateLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	link, err := h.svc.Shorten(r.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidURL) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "could not create link")
		return
	}

	writeJSON(w, http.StatusCreated, link)
}

func (h *Handler) redirect(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	link, err := h.svc.Resolve(r.Context(), code)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "link not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not resolve link")
		return
	}

	http.Redirect(w, r, link.LongURL, http.StatusFound)
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
