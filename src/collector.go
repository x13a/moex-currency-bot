package main

import (
	"context"
	"encoding/json"
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

const (
	EnvTinkoffToken = "TINKOFF_TOKEN"
	MoexIssUrl      = "https://iss.moex.com/iss/engines/currency/markets/selt/boards/CETS/securities.json"
)

func toDecimal(units int64, nano int32) decimal.Decimal {
	return decimal.RequireFromString(fmt.Sprintf("%d.%d", units, nano))
}

func mustGetEnvTinkoffToken() string {
	token := os.Getenv(EnvTinkoffToken)
	if token == "" {
		log.Fatal("empty tinkoff token")
	}
	_ = os.Unsetenv(EnvTinkoffToken)
	return token
}

func newClient(ctx context.Context) *investgo.Client {
	tinkoffConfig := investgo.Config{
		Token: mustGetEnvTinkoffToken(),
	}
	logger := &Logger{}
	client, err := investgo.NewClient(ctx, tinkoffConfig, logger)
	if err != nil {
		log.Fatal(err)
	}
	return client
}

type Rate struct {
	Nominal decimal.Decimal
	Bid     *decimal.Decimal
	Ask     *decimal.Decimal
}

type MoexInstrument struct {
	ValToday float64
}

func collectMoexIssLoop(
	ctx context.Context,
	wg *sync.WaitGroup,
	db *Database,
	cfg *Config,
	waitChan <-chan struct{},
) {
	defer wg.Done()
	client := &http.Client{Timeout: cfg.Bot.HttpTimeout * time.Second}
	<-waitChan
	for {
		collectMoexIss(ctx, client, db)
		select {
		case <-time.After(cfg.Bot.MoexIssUpdateInterval * time.Second):
		case <-ctx.Done():
			return
		}
	}

}

func collectRatesLoop(
	ctx context.Context,
	wg *sync.WaitGroup,
	db *Database,
	cfg *Config,
	waitChan chan<- struct{},
) {
	defer wg.Done()
	client := newClient(ctx)
	defer client.Stop()
	instClient := client.NewInstrumentsServiceClient()
	mdClient := client.NewMarketDataServiceClient()
	collectRates(instClient, mdClient, db)
	close(waitChan)
	for {
		select {
		case <-time.After(cfg.Bot.RatesUpdateInterval * time.Second):
			collectRates(instClient, mdClient, db)
		case <-ctx.Done():
			return
		}
	}
}

func collectRates(
	instClient *investgo.InstrumentsServiceClient,
	mdClient *investgo.MarketDataServiceClient,
	db *Database,
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
		orderBook, err := mdClient.GetOrderBook(currency.Figi, 1)
		if err != nil {
			log.Println(err)
			continue
		}
		rate := &Rate{
			Nominal: toDecimal(currency.Nominal.Units, currency.Nominal.Nano),
		}
		if len(orderBook.Bids) != 0 {
			bid := orderBook.Bids[0]
			bidDec := toDecimal(bid.Price.Units, bid.Price.Nano)
			rate.Bid = &bidDec
		}
		if len(orderBook.Asks) != 0 {
			ask := orderBook.Asks[0]
			askDec := toDecimal(ask.Price.Units, ask.Price.Nano)
			rate.Ask = &askDec
		}
		db.Data.SetRate(currency.Ticker, *rate)
		dbUpdated = true
	}
	if dbUpdated {
		db.Cache.ClearGet()
	}
}

func collectMoexIss(ctx context.Context, client *http.Client, db *Database) {
	const (
		ColSecID    = "SECID"
		ColValToday = "VALTODAY"
	)

	rates := db.Data.GetRates()
	var buf strings.Builder
	for k := range rates {
		buf.WriteString(k)
		buf.WriteString(",")
	}
	securities := buf.String()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, MoexIssUrl, nil)
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
		Columns []string        `json:"columns"`
		Data    [][]interface{} `json:"data"`
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
		mi := MoexInstrument{}
		for idx, col := range mdr.MarketData.Columns {
			switch col {
			case ColSecID:
				secID = data[idx].(string)
			case ColValToday:
				mi.ValToday = data[idx].(float64)
			}
		}
		db.Data.SetInstrument(secID, mi)
	}
	db.Cache.ClearValToday()
}
