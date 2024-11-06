package main

import (
	"context"
	"fmt"
	"log"
	"os"
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
	Buy  *decimal.Decimal
	Sell *decimal.Decimal
}

func collectLoop(
	ctx context.Context,
	db *Database,
	cfg *Config,
) {
	client := newClient(ctx)
	defer client.Stop()
	instClient := client.NewInstrumentsServiceClient()
	mdClient := client.NewMarketDataServiceClient()
	for {
		collect(instClient, mdClient, db)
		select {
		case <-time.After(time.Duration(cfg.Bot.UpdateInterval) * time.Second):
		case <-ctx.Done():
			break
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
		orderBook, err := mdClient.GetOrderBook(currency.Figi, 1)
		if err != nil {
			log.Println(err)
			return
		}
		rate := &Rate{}
		nominal := toDecimal(currency.Nominal.Units, currency.Nominal.Nano)
		useNominal := nominal.GreaterThan(decimal.NewFromFloat(1.0))
		if len(orderBook.Bids) != 0 {
			bid := orderBook.Bids[0]
			sell := toDecimal(bid.Price.Units, bid.Price.Nano)
			if useNominal {
				sell = nominal.Div(sell)
			}
			rate.Sell = &sell
		}
		if len(orderBook.Asks) != 0 {
			ask := orderBook.Asks[0]
			buy := toDecimal(ask.Price.Units, ask.Price.Nano)
			if useNominal {
				buy = nominal.Div(buy)
			}
			rate.Buy = &buy
		}
		db.Set(currency.Ticker, *rate)
		time.Sleep(1 * time.Second)
	}
}
