package main

import "sync"

type Database struct {
	mu   sync.RWMutex
	data map[string]Rate
}

func NewDatabase() *Database {
	return &Database{data: make(map[string]Rate)}
}

func (db *Database) Set(key string, rate Rate) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.data[key] = rate
}

func (db *Database) GetAll() map[string]Rate {
	db.mu.RLock()
	defer db.mu.RUnlock()
	data := make(map[string]Rate, len(db.data))
	for k, v := range db.data {
		data[k] = v
	}
	return data
}
