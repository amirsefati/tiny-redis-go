package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"tiny-redis-go/internal/command"
	"tiny-redis-go/internal/server"
)

func main() {
	logger := log.New(os.Stdout, "[tiny-redis] ", log.LstdFlags|log.Lmsgprefix)

	srv := server.New("0.0.0.0:6379", command.NewRegistry(), logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Printf("listening on %s", srv.Addr())

	if err := srv.ListenAndServe(ctx); err != nil {
		logger.Fatalf("server stopped with error: %v", err)
	}
}
