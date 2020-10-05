package abot

import (
	"database/sql"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/sirupsen/logrus"
)

type app struct {
	conf Conf
	bot  *tgbotapi.BotAPI
	db   *sql.DB
	log  *logrus.Logger
}

func (a *app) run() {
	a.msgLoop()

}

// Conf config
type Conf struct {
	Abills  Abills `toml:"abills"`
	TgToken string `toml:"tgtoken"`
}

// Abills config part for billing
type Abills struct {
	// DBURL::mysql abills url
	DBURL string `toml:"dburl"`
}

// Run bot
func Run(c Conf) error {
	var a app
	a.conf = c
	a.log = logrus.New()
	var err error
	a.log.WithField("token", a.conf.TgToken).Info("connecting")
	a.bot, err = tgbotapi.NewBotAPI(a.conf.TgToken)
	if err != nil {
		return err
	}
	a.run()
	return nil
}

func (a app) msgLoop() error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := a.bot.GetUpdatesChan(u)
	if err != nil {
		return err
	}
	for update := range updates {
		log.Println(update)
	}
	return err
}
