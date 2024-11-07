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
	Nominal decimal.Decimal
	Bid     *decimal.Decimal
	Ask     *decimal.Decimal
}

func collectLoop(
	ctx context.Context,
	wg *sync.WaitGroup,
	db *Database,
	cfg *Config,
) {
	defer wg.Done()
	client := newClient(ctx)
	defer func() {
		err := client.Stop()
		if err != nil {
			log.Println(err)
		}
	}()
	instClient := client.NewInstrumentsServiceClient()
	mdClient := client.NewMarketDataServiceClient()
	for {
		collect(instClient, mdClient, db)
		select {
		case <-time.After(cfg.Bot.UpdateInterval * time.Second):
		case <-ctx.Done():
			return
		}
	}
}

func collect(
	instClient *investgo.InstrumentsServiceClient,
	mdClient *investgo.MarketDataServiceClient,
	db *Database,
) {
	currencies, err := instClient.Currencies(pb.InstrumentStatus_INSTRUMENT_STATUS_ALL)
	if err != nil {
		log.Println(err)
		return
	}
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
		db.Set(currency.Ticker, *rate)
	}
}
