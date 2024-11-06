package main

import (
	"context"
	"fmt"
	"log"
	"os"
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
	if token == "" {
		log.Fatal("empty tinkoff token")
	}
	return token
}

func newClient(ctx context.Context) *investgo.Client {
	tinkoffConfig := investgo.Config{
		Token: getEnvTinkoffToken(),
	}
	logger := &Logger{}
	client, err := investgo.NewClient(ctx, tinkoffConfig, logger)
	if err != nil {
		log.Fatal(err)
	}
	return client
}

type Rate struct {
	Bid *decimal.Decimal
	Ask *decimal.Decimal
}

func collectLoop(
	ctx context.Context,
	wg *sync.WaitGroup,
	db *Database,
	cfg *Config,
) {
	defer wg.Done()
	client := newClient(ctx)
	defer client.Stop()
	instClient := client.NewInstrumentsServiceClient()
	mdClient := client.NewMarketDataServiceClient()
loop:
	for {
		collect(instClient, mdClient, db)
		select {
		case <-time.After(time.Duration(cfg.Bot.UpdateInterval) * time.Second):
		case <-ctx.Done():
			break loop
		}
	}
}

func collect(
	instClient *investgo.InstrumentsServiceClient,
	mdClient *investgo.MarketDataServiceClient,
	db *Database,
) {
	currencies, err := instClient.Currencies(pb.InstrumentStatus_INSTRUMENT_STATUS_UNSPECIFIED)
	if err != nil {
		log.Println(err)
		return
	}
	for _, currency := range currencies.Instruments {
		status, err := mdClient.GetTradingStatus(currency.Figi)
		if err != nil {
			log.Println(err)
			continue
		}
		if status.TradingStatus != pb.SecurityTradingStatus_SECURITY_TRADING_STATUS_NORMAL_TRADING {
			continue
		}
		orderBook, err := mdClient.GetOrderBook(currency.Figi, 1)
		if err != nil {
			log.Println(err)
			continue
		}
		rate := &Rate{}
		nominal := toDecimal(currency.Nominal.Units, currency.Nominal.Nano)
		useNominal := nominal.GreaterThan(decimal.NewFromFloat(1.0))
		if len(orderBook.Bids) != 0 {
			bid := orderBook.Bids[0]
			bidDec := toDecimal(bid.Price.Units, bid.Price.Nano)
			if useNominal {
				bidDec = nominal.Div(bidDec)
			}
			rate.Bid = &bidDec
		}
		if len(orderBook.Asks) != 0 {
			ask := orderBook.Asks[0]
			askDec := toDecimal(ask.Price.Units, ask.Price.Nano)
			if useNominal {
				askDec = nominal.Div(askDec)
			}
			rate.Ask = &askDec
		}
		db.Set(currency.Ticker, *rate)
	}
}
