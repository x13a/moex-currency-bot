package main

import (
	"context"
	"log"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	defer cancel()

	db := NewDatabase()
	syncChan := make(chan struct{}, 1)

	var wg sync.WaitGroup
	wg.Add(3)
	go runBot(ctx, &wg, db, cfg)
	go collectOrderBookLoop(ctx, &wg, db, cfg, syncChan)
	go collectMoexIssLoop(ctx, &wg, db, cfg, syncChan)
	wg.Wait()
}
