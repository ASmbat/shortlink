// Command server runs the shortlink HTTP API.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ASmbat/shortlink/internal/handler"
	"github.com/ASmbat/shortlink/internal/middleware"
	"github.com/ASmbat/shortlink/internal/service"
	"github.com/ASmbat/shortlink/internal/store"
)

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	// The in-memory store keeps the demo dependency-free. A durable backend
	// only needs to implement store.Store.
	s := store.NewMemoryStore()
	svc := service.New(s)
	h := handler.New(svc)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Remove expired links in the background so they don't use memory forever.
	go svc.RunCleanup(ctx, time.Minute, log.Printf)

	srv := &http.Server{
		Addr:         addr,
		Handler:      middleware.Recover(middleware.Logging(h.Routes())),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	log.Println("shutdown complete")
}
