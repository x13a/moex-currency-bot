package main

import (
	"context"
	"os/signal"
	"syscall"
)

func main() {
	cfg := loadConfig()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	defer cancel()
	db := NewDatabase()
	go botRun(ctx, db, cfg)
	go collectLoop(ctx, db, cfg)
	<-ctx.Done()
}
