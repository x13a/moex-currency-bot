package main

import (
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

const EnvBotConfig = "BOT_CONFIG"

type Config struct {
	Bot struct {
		Polling                 bool
		PollingTimeout          time.Duration
		Private                 bool
		ChatIDs                 []int64
		HttpTimeout             time.Duration

		OrderBookUpdateInterval time.Duration
		MoexIssUpdateInterval   time.Duration

		WelcomeMsg  string
		Name        string
		About       string
		Description string

		Webhook struct {
			URL  string
			Port uint16
			Cert string
		}
	}

	MoexIssURL     string
	OrderBookDepth int32
	RateDP         int32
	ConvNominalBYN float64
}

func LoadConfig() (*Config, error) {
	config := &Config{}
	if _, err := toml.DecodeFile(os.Getenv(EnvBotConfig), config); err != nil {
		return nil, err
	}
	return config, nil
}
