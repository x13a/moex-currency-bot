package main

import (
	"context"
	"fmt"
	"html"
	"log"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	tele "gopkg.in/telebot.v4"
)

const (
	EnvBotToken = "BOT_TOKEN"
	RateDP      = 4
	Dunno       = `¯\_(ツ)_/¯`

	CmdGet     = "/get"
	CmdGetConv = "/getconv"
)

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
	wg *sync.WaitGroup,
	db *Database,
	cfg *Config,
) {
	defer wg.Done()
	// TODO webhook
	pref := tele.Settings{
		Token:     getEnvBotToken(),
		Poller:    &tele.LongPoller{Timeout: 10 * time.Second},
		ParseMode: tele.ModeHTML,
	}
	b, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)
	}
	err = b.SetCommands([]tele.Command{
		{
			Text:        CmdGet[1:],
			Description: "Get Rates",
		},
		{
			Text:        CmdGetConv[1:],
			Description: "Get Rates Conv",
		},
	})
	if err != nil {
		log.Println(err)
	}
	b.Use(PrivateMiddleware(cfg))
	b.Handle("/start", func(c tele.Context) error {
		return c.Send(cfg.Bot.WelcomeMsg)
	})
	b.Handle("/id", func(c tele.Context) error {
		chatId := int64(0)
		chat := c.Chat()
		if chat != nil {
			chatId = chat.ID
		}
		return c.Send(strconv.FormatInt(chatId, 10))
	})
	b.Handle(CmdGet, getHandler(db, false))
	b.Handle(CmdGetConv, getHandler(db, true))
	go b.Start()
	defer b.Stop()
	<-ctx.Done()
}

func codeInline(s string) string {
	return fmt.Sprintf("<code>%s</code>", html.EscapeString(s))
}

func PrivateMiddleware(cfg *Config) tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			if !cfg.Bot.Private {
				return next(c)
			}
			chat := c.Chat()
			if chat == nil {
				return nil
			}
			chatId := chat.ID
			if chatId == 0 {
				return nil
			}
			if slices.Contains(cfg.Bot.ChatIds, chatId) {
				return next(c)
			}
			return nil
		}
	}
}

func getHandler(db *Database, conv bool) tele.HandlerFunc {
	return func(c tele.Context) error {
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
			hasNominal := v.Nominal.GreaterThan(decimal.NewFromFloat(1.0))
			if conv && !hasNominal {
				continue
			}
			bid := ""
			if v.Bid != nil {
				if conv {
					bid := v.Nominal.Div(*v.Bid)
					v.Bid = &bid
				}
				bid = v.Bid.StringFixed(RateDP)
				bid = strings.TrimRight(bid, "0")
				bid = strings.TrimSuffix(bid, ".")
			}
			ask := ""
			if v.Ask != nil {
				if conv {
					ask := v.Nominal.Div(*v.Ask)
					v.Ask = &ask
				}
				ask = v.Ask.StringFixed(RateDP)
				ask = strings.TrimRight(ask, "0")
				ask = strings.TrimSuffix(ask, ".")
			}
			if bid == "" && ask == "" {
				continue
			}
			bidWidth = max(len(bid), bidWidth)
			askWidth = max(len(ask), askWidth)
			if conv {
				bid, ask = ask, bid
			} else if hasNominal {
				k += "*"
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
		if conv {
			bidWidth, askWidth = askWidth, bidWidth
		}
		var buf strings.Builder
		for _, v := range arrRes {
			buf.WriteString(fmt.Sprintf("%-*s | %-*s | %s\n", bidWidth, v.bid, askWidth, v.ask, v.ticker))
		}
		s := strings.TrimSuffix(buf.String(), "\n")
		if len(s) == 0 {
			return c.Send(Dunno)
		}
		return c.Send(codeInline(s))
	}
}
