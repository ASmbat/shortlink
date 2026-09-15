// Command server runs the shortlink HTTP API.
package main

import (
	"context"
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

	// MemoryStore is used here for simplicity; swap in a store.Store backed
	// by DynamoDB or another durable store for production use — the rest of
	// the app depends only on the interface in internal/store.
	s := store.NewMemoryStore()
	svc := service.New(s)
	h := handler.New(svc)

	srv := &http.Server{
		Addr:         addr,
		Handler:      middleware.Recover(middleware.Logging(h.Routes())),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown on SIGINT/SIGTERM.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("graceful shutdown failed: %v", err)
	}
	log.Println("shutdown complete")
}
