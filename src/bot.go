package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"
)

const EnvBotToken = "BOT_TOKEN"
const RateDP = 4

func getEnvBotToken() string {
	token := os.Getenv(EnvBotToken)
	_ = os.Unsetenv(EnvBotToken)
	if token == "" {
		log.Fatal("empty bot token")
	}
	return token
}

func botRun(
	ctx context.Context,
	db *Database,
	cfg *Config,
) {
	pref := tele.Settings{
		Token:     getEnvBotToken(),
		Poller:    &tele.LongPoller{Timeout: 10 * time.Second},
		ParseMode: tele.ModeHTML,
	}
	b, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)
	}
	b.Handle("/get", func(c tele.Context) error {
		data := db.GetAll()
		arr := make([]string, len(data))
		idx := 0
		for k, v := range data {
			var buy string
			if v.Buy != nil {
				buy = v.Buy.StringFixed(RateDP)
			}
			var sell string
			if v.Sell != nil {
				sell = v.Sell.StringFixed(RateDP)
			}
			if buy == "" && sell == "" {
				continue
			}
			arr[idx] = fmt.Sprintf("%-9s | %-9s | %s", buy, sell, k)
			idx++
		}
		return c.Send(fmt.Sprintf("<code>%s</code>", strings.Join(arr, "\n")))
	})
	b.Start()
}
