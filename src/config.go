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
		Polling        bool
		PollingTimeout time.Duration
		Private        bool
		ChatIds        []int64
		UpdateInterval time.Duration
		WelcomeMsg     string
		Webhook        struct {
			Url  string
			Port uint16
			Cert string
		}
	}
}

func loadConfig() *Config {
	config := &Config{}
	_, err := toml.DecodeFile(os.Getenv(EnvBotConfig), config)
	if err != nil {
		log.Fatal(err)
	}
	return config
}
