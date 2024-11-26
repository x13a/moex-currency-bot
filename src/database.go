package main

import (
	"fmt"
	"strings"
	"sync"
)

type Database struct {
	Data  *Data
	Cache *Cache
}

type Data struct {
	muBooks sync.RWMutex
	muMoex  sync.RWMutex
	books   map[string]OrderBook
	moex    map[string]Instrument
}

func (d *Data) SetOrderBook(ticker string, book OrderBook) {
	d.muBooks.Lock()
	defer d.muBooks.Unlock()
	d.books[ticker] = book
}

func (d *Data) GetOrderBook(ticker string) (OrderBook, bool) {
	d.muBooks.RLock()
	defer d.muBooks.RUnlock()
	v, ok := d.books[ticker]
	if !ok {
		return OrderBook{}, false
	}
	return v.Copy(), true
}

func (d *Data) GetOrderBooks() map[string]OrderBook {
	d.muBooks.RLock()
	defer d.muBooks.RUnlock()
	res := make(map[string]OrderBook, len(d.books))
	for k, v := range d.books {
		res[k] = v.Copy()
	}
	return res
}

func (d *Data) GetTickers() []string {
	d.muBooks.RLock()
	defer d.muBooks.RUnlock()
	res := make([]string, len(d.books))
	i := 0
	for k := range d.books {
		res[i] = k
		i++
	}
	return res
}

func (d *Data) SetInstrument(ticker string, inst Instrument) {
	d.muMoex.Lock()
	defer d.muMoex.Unlock()
	d.moex[ticker] = inst
}

func (d *Data) GetInstruments() map[string]Instrument {
	d.muMoex.RLock()
	defer d.muMoex.RUnlock()
	res := make(map[string]Instrument, len(d.moex))
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

func (c *Cache) GetOrderBook(cmd, ticker string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.data[fmtOrderBookKey(cmd, ticker)]
	return v, ok
}

func (c *Cache) SetOrderBook(cmd, ticker, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[fmtOrderBookKey(cmd, ticker)] = value
}

func fmtOrderBookKey(cmd, ticker string) string {
	return fmt.Sprintf("%s_%s", cmd, ticker)
}

func (c *Cache) ClearRates() {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, CmdRates)
	delete(c.data, CmdRatesConv)
}

func (c *Cache) ClearValToday() {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, CmdValToday)
}

func (c *Cache) ClearOrderBook() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.data {
		if strings.HasPrefix(k, CmdOrderBook) {
			delete(c.data, k)
		}
	}
}

func NewDatabase() *Database {
	return &Database{
		Data: &Data{
			books: make(map[string]OrderBook),
			moex:  make(map[string]Instrument),
		},
		Cache: &Cache{data: make(map[string]string)},
	}
}
