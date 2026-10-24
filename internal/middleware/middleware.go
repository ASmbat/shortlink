package middleware

import (
	"log"
	"net/http"
	"time"
)

// responseRecorder remembers the status code written by the next handler,
// so it can be logged and so Recover knows whether a response has started.
type responseRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func newRecorder(w http.ResponseWriter) *responseRecorder {
	if rec, ok := w.(*responseRecorder); ok {
		return rec
	}
	return &responseRecorder{ResponseWriter: w, status: http.StatusOK}
}

func (r *responseRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.wroteHeader = true // an implicit 200
	return r.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the underlying writer
// (for Flush, deadlines and so on).
func (r *responseRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// Logging logs method, path, status code and latency for every request.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := newRecorder(w)
		next.ServeHTTP(rec, r)
		log.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, rec.status, time.Since(start))
	})
}

// Recover turns a panic in the next handler into a 500 response instead of
// crashing the server. If the response has already started, it can't send
// a 500, so it only logs. http.ErrAbortHandler is re-raised: it is the
// standard way for a handler to abort a response on purpose.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := newRecorder(w)
		defer func() {
			err := recover()
			if err == nil {
				return
			}
			if err == http.ErrAbortHandler {
				panic(err)
			}
			log.Printf("panic recovered: %v", err)
			if !rec.wroteHeader {
				http.Error(rec, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(rec, r)
	})
}
