package main

import (
	"log"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

const EnvBotConfig = "BOT_CONFIG"

type Config struct {
	Bot struct {
		Polling               bool
		PollingTimeout        time.Duration
		Private               bool
		ChatIDs               []int64
		HttpTimeout           time.Duration
		RatesUpdateInterval   time.Duration
		MoexIssUpdateInterval time.Duration
		WelcomeMsg            string

		Name        string
		About       string
		Description string

		Webhook struct {
			Url  string
			Port uint16
			Cert string
		}
	}
	Gen struct {
		RateDP int32
	}
}

func mustLoadConfig() *Config {
	config := &Config{}
	if _, err := toml.DecodeFile(os.Getenv(EnvBotConfig), config); err != nil {
		log.Fatal(err)
	}
	return config
}
