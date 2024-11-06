# moex-currency-bot

MoEx currency exchange rates telegram bot.

## Setup

```shell
make fix-perm
```

You have to set `BOT_TOKEN` env to your telegram bot token in `config/bot.env`.
You have to set `TINKOFF_TOKEN` env to your tinkoff invest api token in `config/bot.env`.

## Run

```shell
docker compose -f ./compose.polling.yaml up -d
```

## License

[![GNU GPLv3 Image](https://www.gnu.org/graphics/gplv3-127x51.png)](https://www.gnu.org/licenses/gpl-3.0.en.html)
