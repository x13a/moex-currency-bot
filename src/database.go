package main

import "sync"

type Database struct {
	Data  *Data
	Cache *Cache
}

type Data struct {
	muRates sync.RWMutex
	muMoex  sync.RWMutex
	rates   map[string]Rate
	moex    map[string]MoexInstrument
}

func (d *Data) SetRate(key string, rate Rate) {
	d.muRates.Lock()
	defer d.muRates.Unlock()
	d.rates[key] = rate
}

func (d *Data) GetRates() map[string]Rate {
	d.muRates.RLock()
	defer d.muRates.RUnlock()
	rates := make(map[string]Rate, len(d.rates))
	for k, v := range d.rates {
		rates[k] = v
	}
	return rates
}

func (d *Data) SetInstrument(key string, value MoexInstrument) {
	d.muMoex.Lock()
	defer d.muMoex.Unlock()
	d.moex[key] = value
}

func (d *Data) GetInstruments() map[string]MoexInstrument {
	d.muMoex.RLock()
	defer d.muMoex.RUnlock()
	res := make(map[string]MoexInstrument, len(d.moex))
	for k, v := range d.moex {
		res[k] = v
	}
	return res
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

func (c *Cache) ClearGet() {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, CmdGet)
	delete(c.data, CmdGetConv)
}

func (c *Cache) ClearValToday() {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, CmdValToday)
}

func NewDatabase() *Database {
	return &Database{
		Data: &Data{
			rates: make(map[string]Rate),
			moex:  make(map[string]MoexInstrument),
		},
		Cache: &Cache{data: make(map[string]string)},
	}
}
