package main

import "sync"

type Database struct {
	Data  *Data
	Cache *Cache
}

type Data struct {
	mu    sync.RWMutex
	rates map[string]Rate
}

func (d *Data) SetRate(key string, rate Rate) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.rates[key] = rate
}

func (d *Data) GetRates() map[string]Rate {
	d.mu.RLock()
	defer d.mu.RUnlock()
	rates := make(map[string]Rate, len(d.rates))
	for k, v := range d.rates {
		rates[k] = v
	}
	return rates
}

type Cache struct {
	mu   sync.RWMutex
	data map[string]string
}

func (c *Cache) Set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
}

func (c *Cache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.data[key]
	return v, ok
}

func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]string)
}

func NewDatabase() *Database {
	return &Database{
		Data:  &Data{rates: make(map[string]Rate)},
		Cache: &Cache{data: make(map[string]string)},
	}
}
