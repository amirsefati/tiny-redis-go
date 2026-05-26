package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tiny-redis-go/internal/command"
	"tiny-redis-go/internal/server"
	"tiny-redis-go/internal/store"
)

func main() {
	logger := log.New(os.Stdout, "[tiny-redis] ", log.LstdFlags|log.Lmsgprefix)

	db := store.New(store.WithActiveExpiration(100*time.Millisecond, 32))
	defer db.Close()
	srv := server.New("0.0.0.0:6379", command.NewRegistry(db), logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Printf("listening on %s", srv.Addr())

	if err := srv.ListenAndServe(ctx); err != nil {
		logger.Fatalf("server stopped with error: %v", err)
	}
}
