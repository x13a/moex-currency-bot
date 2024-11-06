package main

import (
	"log"
	"os"

	"github.com/BurntSushi/toml"
)

const EnvBotConfig = "BOT_CONFIG"

type Config struct {
	Bot struct {
		Polling        bool
		Private        bool
		Users          []string
		UpdateInterval int
		Webhook        struct {
			host string
			port int
			cert string
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
