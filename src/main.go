package main

import (
	"context"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	cfg := loadConfig()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	defer cancel()
	db := NewDatabase()
	var wg sync.WaitGroup
	wg.Add(2)
	go botRun(ctx, &wg, db, cfg)
	go collectLoop(ctx, &wg, db, cfg)
	wg.Wait()
}
