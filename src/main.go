package main

import (
	"context"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	cfg := mustLoadConfig()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	defer cancel()
	db := NewDatabase()
	var wg sync.WaitGroup
	wg.Add(3)
	go runBot(ctx, &wg, db, cfg)
	waitChan := make(chan struct{}, 1)
	go collectRatesLoop(ctx, &wg, db, cfg, waitChan)
	go collectMoexIssLoop(ctx, &wg, db, cfg, waitChan)
	wg.Wait()
}
