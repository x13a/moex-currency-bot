package main

import (
	"context"
	"errors"
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
	Dunno       = `¯\_(ツ)_/¯`

	CmdRates         = "/rates"
	CmdRatesConv     = "/ratesconv"
	CmdOrderBook     = "/orderbook"
	CmdOrderBookConv = "/orderbookconv"
	CmdValToday      = "/valtoday"
)

var Commands = []tele.Command{
	{
		Text:        CmdRates[1:],
		Description: "rates",
	},
	{
		Text:        CmdRatesConv[1:],
		Description: "rates conv",
	},
	{
		Text:        CmdOrderBook[1:],
		Description: "<TICKER> order book",
	},
	{
		Text:        CmdOrderBookConv[1:],
		Description: "<TICKER> order book conv",
	},
	{
		Text:        CmdValToday[1:],
		Description: "value today",
	},
}

func getEnvBotToken() string {
	token := os.Getenv(EnvBotToken)
	_ = os.Unsetenv(EnvBotToken)
	return token
}

func newBot(cfg *Config) (*tele.Bot, error) {
	token := getEnvBotToken()
	if token == "" {
		return nil, errors.New("bot token is required")
	}
	pref := tele.Settings{
		Token:     token,
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
		return nil, err
	}
	return b, nil
}

func removeWebhook(b *tele.Bot) error {
	wh, err := b.Webhook()
	if err != nil {
		return err
	}
	if wh.Listen != "" {
		return b.RemoveWebhook(false)
	}
	return nil
}

func runBot(
	ctx context.Context,
	wg *sync.WaitGroup,
	db *Database,
	cfg *Config,
) {
	defer wg.Done()
	b, err := newBot(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if cfg.Bot.Polling {
		if err = removeWebhook(b); err != nil {
			log.Fatal(err)
		}
	}
	if err = b.SetMyName(cfg.Bot.Name, ""); err != nil {
		log.Println(err)
	}
	if err = b.SetMyShortDescription(cfg.Bot.About, ""); err != nil {
		log.Println(err)
	}
	if err = b.SetMyDescription(cfg.Bot.Description, ""); err != nil {
		log.Println(err)
	}
	if err = b.SetCommands(Commands); err != nil {
		log.Println(err)
	}
	b.Use(PrivateMiddleware(cfg))
	b.Handle("/start", startHandler(db, cfg))
	b.Handle("/help", helpHandler(Commands))
	b.Handle("/id", idHandler)
	b.Handle(CmdRates, ratesHandler(db, cfg, CmdRates))
	b.Handle(CmdRatesConv, ratesHandler(db, cfg, CmdRatesConv))
	b.Handle(CmdValToday, valTodayHandler(db))
	b.Handle(CmdOrderBook, orderBookHandler(db, cfg, CmdOrderBook))
	b.Handle(CmdOrderBookConv, orderBookHandler(db, cfg, CmdOrderBookConv))
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

func helpHandler(cmds []tele.Command) tele.HandlerFunc {
	return func(c tele.Context) error {
		var buf strings.Builder
		for _, cmd := range cmds {
			fmt.Fprintf(&buf, "/%s - %s\n", cmd.Text, cmd.Description)
		}
		return c.Send(html.EscapeString(buf.String()))
	}
}

func idHandler(c tele.Context) error {
	chatID := int64(0)
	if chat := c.Chat(); chat != nil {
		chatID = chat.ID
	}
	return c.Send(strconv.FormatInt(chatID, 10))
}

func startHandler(db *Database, cfg *Config) tele.HandlerFunc {
	return func(c tele.Context) error {
		if msg := c.Message(); msg != nil {
			switch msg.Payload {
			case CmdRates[1:]:
				return ratesHandler(db, cfg, CmdRates)(c)
			case CmdRatesConv[1:]:
				return ratesHandler(db, cfg, CmdRatesConv)(c)
			case CmdValToday[1:]:
				return valTodayHandler(db)(c)
			}
		}
		return c.Send(html.EscapeString(cfg.Bot.WelcomeMsg))
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
			fmt.Fprintf(&buf, "%-*s | %s\n", width, v.valToday, v.ticker)
		}
		s = strings.TrimSuffix(buf.String(), "\n")
		if s != "" {
			s = codeInline(s)
			db.Cache.Set(CmdValToday, s)
		} else {
			s = Dunno
		}
		return c.Send(s)
	}
}

func ratesHandler(db *Database, cfg *Config, cmd string) tele.HandlerFunc {
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

		books := db.Data.GetOrderBooks()
		arrRes := make([]Result, len(books))
		idx := 0
		bidWidth := 0
		askWidth := 0
		conv := cmd == CmdRatesConv
		for k, v := range books {
			isByn := false
			hasNominal := v.Nominal.GreaterThan(decimal.NewFromFloat(1.0))
			if conv && !hasNominal {
				isByn = strings.HasPrefix(k, "BYN")
				switch {
				case strings.HasPrefix(k, "TRY"):
				case isByn:
					v.Nominal = decimal.NewFromFloat(cfg.ConvNominalBYN)
				default:
					continue
				}
			}
			bid := ""
			if len(v.Bids) != 0 {
				d := v.Bids[0].Price
				if conv {
					d = v.Nominal.Div(d)
				}
				bid = decimalToString(d, cfg.RateDP)
			}
			ask := ""
			if len(v.Asks) != 0 {
				d := v.Asks[0].Price
				if conv {
					d = v.Nominal.Div(d)
				}
				ask = decimalToString(d, cfg.RateDP)
			}
			if bid == "" && ask == "" {
				continue
			}
			bidWidth = max(len(bid), bidWidth)
			askWidth = max(len(ask), askWidth)
			if conv {
				bid, ask = ask, bid
				if isByn {
					k += "*"
				}
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
			fmt.Fprintf(&buf, "%-*s | %-*s | %s\n", bidWidth, v.bid, askWidth, v.ask, v.ticker)
		}
		s = strings.TrimSuffix(buf.String(), "\n")
		if s != "" {
			s = codeInline(s)
			db.Cache.Set(cmd, s)
		} else {
			s = Dunno
		}
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

func decimalToString(d decimal.Decimal, places int32) string {
	s := d.StringFixed(places)
	s = strings.TrimRight(s, "0")
	s = strings.TrimSuffix(s, ".")
	return s
}

func orderBookHandler(db *Database, cfg *Config, cmd string) tele.HandlerFunc {
	return func(c tele.Context) error {
		args := c.Args()
		if len(args) == 0 {
			return c.Send(Dunno)
		}
		ticker := strings.ToUpper(args[0])
		s, ok := db.Cache.GetOrderBook(cmd, ticker)
		if ok {
			return c.Send(s)
		}
		book, ok := db.Data.GetOrderBook(ticker)
		if !ok || (len(book.Asks) == 0 && len(book.Bids) == 0) {
			return c.Send(Dunno)
		}
		conv := cmd == CmdOrderBookConv
		if conv && strings.HasPrefix(ticker, "BYN") {
			book.Nominal = decimal.NewFromFloat(cfg.ConvNominalBYN)
		}

		type Result struct {
			price    string
			quantity string
		}

		res := make([]Result, len(book.Bids)+len(book.Asks)+1)
		i := 0
		width := 1
		for j := len(book.Asks) - 1; j >= 0; j-- {
			v := book.Asks[j]
			quantity := strconv.FormatInt(v.Quantity, 10)
			width = max(len(quantity), width)
			if conv {
				v.Price = book.Nominal.Div(v.Price)
			}
			res[i] = Result{
				price:    decimalToString(v.Price, cfg.RateDP),
				quantity: quantity,
			}
			i++
		}
		res[i] = Result{
			price:    "-",
			quantity: "-",
		}
		i++
		for _, v := range book.Bids {
			quantity := strconv.FormatInt(v.Quantity, 10)
			width = max(len(quantity), width)
			if conv {
				v.Price = book.Nominal.Div(v.Price)
			}
			res[i] = Result{
				price:    decimalToString(v.Price, cfg.RateDP),
				quantity: quantity,
			}
			i++
		}
		var buf strings.Builder
		for _, v := range res {
			fmt.Fprintf(&buf, "%-*s | %s\n", width, v.quantity, v.price)
		}
		s = strings.TrimSuffix(buf.String(), "\n")
		if s != "" {
			s = codeInline(s)
			db.Cache.SetOrderBook(cmd, ticker, s)
		} else {
			s = Dunno
		}
		return c.Send(s)
	}
}
