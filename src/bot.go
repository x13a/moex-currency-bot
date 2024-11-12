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

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	tele "gopkg.in/telebot.v4"
)

const (
	EnvBotToken = "BOT_TOKEN"
	RateDP      = 4
	Dunno       = `¯\_(ツ)_/¯`

	CmdGet      = "/get"
	CmdGetConv  = "/getconv"
	CmdValToday = "/valtoday"
)

func mustGetEnvBotToken() string {
	token := os.Getenv(EnvBotToken)
	if token == "" {
		log.Fatal("empty bot token")
	}
	_ = os.Unsetenv(EnvBotToken)
	return token
}

func runBot(
	ctx context.Context,
	wg *sync.WaitGroup,
	db *Database,
	cfg *Config,
) {
	defer wg.Done()
	pref := tele.Settings{
		Token:     mustGetEnvBotToken(),
		ParseMode: tele.ModeHTML,
	}
	if cfg.Bot.Polling {
		pref.Poller = &tele.LongPoller{Timeout: cfg.Bot.PollingTimeout * time.Second}
	} else {
		pref.Poller = &tele.Webhook{
			Listen:      fmt.Sprintf("0.0.0.0:%d", cfg.Bot.Webhook.Port),
			SecretToken: uuid.New().String(),
			Endpoint: &tele.WebhookEndpoint{
				PublicURL: cfg.Bot.Webhook.Url,
				Cert:      cfg.Bot.Webhook.Cert,
			},
		}
	}
	b, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)
	}
	cmds := []tele.Command{
		{
			Text:        CmdGet[1:],
			Description: "Rates",
		},
		{
			Text:        CmdGetConv[1:],
			Description: "Rates Conv",
		},
		{
			Text:        CmdValToday[1:],
			Description: "Value Today",
		},
	}
	if err = b.SetCommands(cmds); err != nil {
		log.Println(err)
	}
	b.Use(PrivateMiddleware(cfg))
	b.Handle("/start", func(c tele.Context) error {
		return c.Send(cfg.Bot.WelcomeMsg)
	})
	b.Handle("/help", func(c tele.Context) error {
		var buf strings.Builder
		for _, cmd := range cmds {
			buf.WriteString(fmt.Sprintf("/%s - %s\n", cmd.Text, cmd.Description))
		}
		return c.Send(buf.String())
	})
	b.Handle("/id", func(c tele.Context) error {
		chatID := int64(0)
		if chat := c.Chat(); chat != nil {
			chatID = chat.ID
		}
		return c.Send(strconv.FormatInt(chatID, 10))
	})
	b.Handle(CmdValToday, valTodayHandler(db))
	b.Handle(CmdGet, getHandler(db, CmdGet))
	b.Handle(CmdGetConv, getHandler(db, CmdGetConv))
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
			chatID := chat.ID
			if chatID == 0 {
				return nil
			}
			if slices.Contains(cfg.Bot.ChatIDs, chatID) {
				return next(c)
			}
			return nil
		}
	}
}

func valTodayHandler(db *Database) tele.HandlerFunc {
	return func(c tele.Context) error {
		s, ok := db.Cache.Get(CmdValToday)
		if ok {
			return c.Send(s)
		}

		type Result struct {
			ticker   string
			valToday string
		}

		instruments := db.Data.GetInstruments()
		arrRes := make([]Result, len(instruments))
		idx := 0
		width := 0
		for k, v := range instruments {
			if v.ValToday == 0.0 {
				continue
			}
			res := Result{
				ticker:   k,
				valToday: formatFloat64(v.ValToday),
			}
			width = max(len(res.valToday), width)
			arrRes[idx] = res
			idx++
		}
		arrRes = arrRes[:idx]
		sort.Slice(arrRes, func(i, j int) bool {
			return arrRes[i].ticker < arrRes[j].ticker
		})
		var buf strings.Builder
		for _, v := range arrRes {
			buf.WriteString(fmt.Sprintf("%-*s | %s\n", width, v.valToday, v.ticker))
		}
		s = strings.TrimSuffix(buf.String(), "\n")
		if len(s) == 0 {
			return c.Send(Dunno)
		}
		s = codeInline(s)
		db.Cache.Set(CmdValToday, s)
		return c.Send(s)
	}
}

func getHandler(db *Database, cmd string) tele.HandlerFunc {
	return func(c tele.Context) error {
		s, ok := db.Cache.Get(cmd)
		if ok {
			return c.Send(s)
		}

		type Result struct {
			ticker string
			bid    string
			ask    string
		}

		rates := db.Data.GetRates()
		arrRes := make([]Result, len(rates))
		idx := 0
		bidWidth := 0
		askWidth := 0
		conv := cmd == CmdGetConv
		for k, v := range rates {
			hasNominal := v.Nominal.GreaterThan(decimal.NewFromFloat(1.0))
			if conv && !hasNominal {
				switch {
				case strings.HasPrefix(k, "TRY"):
				case strings.HasPrefix(k, "BYN"):
					v.Nominal = decimal.NewFromFloat(100.0)
				default:
					continue
				}
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
		s = strings.TrimSuffix(buf.String(), "\n")
		if len(s) == 0 {
			return c.Send(Dunno)
		}
		s = codeInline(s)
		db.Cache.Set(cmd, s)
		return c.Send(s)
	}
}

func formatFloat64(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if len(s) <= 3 {
		return s
	}
	numOfComma := (len(s) - 1) / 3
	res := make([]byte, len(s)+numOfComma)
	for i, j, k := len(s)-1, len(res)-1, 0; ; i, j = i-1, j-1 {
		res[j] = s[i]
		if i == 0 {
			break
		}
		if k++; k == 3 {
			j, k = j-1, 0
			res[j] = ','
		}
	}
	return string(res)
}
