package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"
)

const EnvBotToken = "BOT_TOKEN"
const RateDP = 4
const Dunno = "¯\\_(ツ)_/¯"

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
	b.Handle("/start", func(c tele.Context) error {
		return c.Send("Meow!")
	})
	b.Handle("/get", func(c tele.Context) error {
		type Result struct {
			ticker string
			bid    string
			ask    string
		}

		data := db.GetAll()
		arrRes := make([]Result, len(data))
		idx := 0
		bidWidth := 0
		askWidth := 0
		for k, v := range data {
			var bid string
			if v.Bid != nil {
				bid = v.Bid.StringFixed(RateDP)
				bid = strings.TrimRight(bid, ".0")
				bidWidth = max(bidWidth, len(bid))
			}
			var ask string
			if v.Ask != nil {
				ask = v.Ask.StringFixed(RateDP)
				ask = strings.TrimRight(ask, ".0")
				askWidth = max(askWidth, len(ask))
			}
			if bid == "" && ask == "" {
				continue
			}
			arrRes[idx] = Result{
				ticker: k,
				bid:    bid,
				ask:    ask,
			}
			idx++
		}
		arrRes = arrRes[:idx]
		sort.Slice(arrRes, func(i, j int) bool {
			return arrRes[i].ticker < arrRes[j].ticker
		})
		arr := make([]string, len(arrRes))
		for i, v := range arrRes {
			arr[i] = fmt.Sprintf("%-*s | %-*s | %s", bidWidth, v.bid, askWidth, v.ask, v.ticker)
		}
		s := strings.Join(arr, "\n")
		if len(s) == 0 {
			return c.Send(Dunno)
		}
		return c.Send(codeInline(s))
	})
	b.Start()
}

func codeInline(s string) string {
	return fmt.Sprintf("<code>%s</code>", s)
}
