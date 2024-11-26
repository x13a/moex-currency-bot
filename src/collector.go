package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/tinkoff/invest-api-go-sdk/investgo"
	pb "github.com/tinkoff/invest-api-go-sdk/proto"
)

const EnvTinkoffToken = "TINKOFF_TOKEN"

func toDecimal(units int64, nano int32) decimal.Decimal {
	return decimal.RequireFromString(fmt.Sprintf("%d.%d", units, nano))
}

func getEnvTinkoffToken() string {
	token := os.Getenv(EnvTinkoffToken)
	_ = os.Unsetenv(EnvTinkoffToken)
	return token
}

func newClient(ctx context.Context) (*investgo.Client, error) {
	token := getEnvTinkoffToken()
	if token == "" {
		return nil, errors.New("tinkoff token is required")
	}
	tinkoffConfig := investgo.Config{
		Token: token,
	}
	logger := &Logger{}
	client, err := investgo.NewClient(ctx, tinkoffConfig, logger)
	if err != nil {
		return nil, err
	}
	return client, nil
}

type OrderBook struct {
	Nominal decimal.Decimal
	Bids    []Order
	Asks    []Order
}

func (b *OrderBook) Copy() OrderBook {
	bids := make([]Order, len(b.Bids))
	for i, bid := range b.Bids {
		bids[i] = bid.Copy()
	}
	asks := make([]Order, len(b.Asks))
	for i, ask := range b.Asks {
		asks[i] = ask.Copy()
	}
	return OrderBook{
		Nominal: b.Nominal.Copy(),
		Bids:    bids,
		Asks:    asks,
	}
}

type Order struct {
	Price    decimal.Decimal
	Quantity int64
}

func (o *Order) Copy() Order {
	return Order{
		Price:    o.Price.Copy(),
		Quantity: o.Quantity,
	}
}

type Instrument struct {
	ValToday float64
}

func collectMoexIssLoop(
	ctx context.Context,
	wg *sync.WaitGroup,
	db *Database,
	cfg *Config,
	syncChan <-chan struct{},
) {
	defer wg.Done()
	client := &http.Client{Timeout: cfg.Bot.HttpTimeout * time.Second}
	<-syncChan
	for {
		collectMoexIss(ctx, client, db, cfg)
		select {
		case <-time.After(cfg.Bot.MoexIssUpdateInterval * time.Second):
		case <-ctx.Done():
			return
		}
	}
}

func collectOrderBookLoop(
	ctx context.Context,
	wg *sync.WaitGroup,
	db *Database,
	cfg *Config,
	syncChan chan<- struct{},
) {
	defer wg.Done()
	client, err := newClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Stop()
	instClient := client.NewInstrumentsServiceClient()
	mdClient := client.NewMarketDataServiceClient()
	collectOrderBook(instClient, mdClient, db, cfg)
	close(syncChan)
	for {
		select {
		case <-time.After(cfg.Bot.OrderBookUpdateInterval * time.Second):
			collectOrderBook(instClient, mdClient, db, cfg)
		case <-ctx.Done():
			return
		}
	}
}

func newOrderFromProtoOrder(o *pb.Order) Order {
	return Order{
		Price:    toDecimal(o.Price.Units, o.Price.Nano),
		Quantity: o.Quantity,
	}
}

func collectOrderBook(
	instClient *investgo.InstrumentsServiceClient,
	mdClient *investgo.MarketDataServiceClient,
	db *Database,
	cfg *Config,
) {
	currencies, err := instClient.Currencies(pb.InstrumentStatus_INSTRUMENT_STATUS_ALL)
	if err != nil {
		log.Println(err)
		return
	}
	dbUpdated := false
	for _, currency := range currencies.Instruments {
		if currency.TradingStatus != pb.SecurityTradingStatus_SECURITY_TRADING_STATUS_NORMAL_TRADING {
			continue
		}
		orderBook, err := mdClient.GetOrderBook(currency.Figi, cfg.OrderBookDepth)
		if err != nil {
			log.Println(err)
			continue
		}
		book := OrderBook{
			Nominal: toDecimal(currency.Nominal.Units, currency.Nominal.Nano),
			Bids:    make([]Order, len(orderBook.Bids)),
			Asks:    make([]Order, len(orderBook.Asks)),
		}
		for i, bid := range orderBook.Bids {
			book.Bids[i] = newOrderFromProtoOrder(bid)
		}
		for i, ask := range orderBook.Asks {
			book.Asks[i] = newOrderFromProtoOrder(ask)
		}
		db.Data.SetOrderBook(currency.Ticker, book)
		dbUpdated = true
	}
	if dbUpdated {
		db.Cache.ClearRates()
		db.Cache.ClearOrderBook()
	}
}

func collectMoexIss(ctx context.Context, client *http.Client, db *Database, cfg *Config) {
	const (
		ColSecID    = "SECID"
		ColValToday = "VALTODAY"
	)

	tickers := db.Data.GetTickers()
	if len(tickers) == 0 {
		return
	}
	var buf strings.Builder
	for _, v := range tickers {
		buf.WriteString(v)
		buf.WriteString(",")
	}
	securities := buf.String()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.MoexIssURL, nil)
	if err != nil {
		log.Println(err)
		return
	}
	q := req.URL.Query()
	q.Add("iss.only", "marketdata")
	q.Add("marketdata.columns", strings.Join([]string{ColSecID, ColValToday}, ","))
	q.Add("securities", securities[:len(securities)-1])
	q.Add("iss.meta", "off")
	q.Add("iss.clear_cache", "1")
	q.Add("iss.json", "compact")
	req.URL.RawQuery = q.Encode()
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return
	}

	type MarketData struct {
		Columns []string `json:"columns"`
		Data    [][]any  `json:"data"`
	}

	type MarketDataResponse struct {
		MarketData MarketData `json:"marketdata"`
	}

	var mdr MarketDataResponse
	if err = json.Unmarshal(body, &mdr); err != nil {
		log.Println(err)
		return
	}
	for _, data := range mdr.MarketData.Data {
		secID := ""
		inst := Instrument{}
		for idx, col := range mdr.MarketData.Columns {
			switch col {
			case ColSecID:
				secID = data[idx].(string)
			case ColValToday:
				inst.ValToday = data[idx].(float64)
			}
		}
		db.Data.SetInstrument(secID, inst)
	}
	db.Cache.ClearValToday()
}
