package abot

import (
	"log"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
)

type app struct {
	conf Conf
	bot  *tgbotapi.BotAPI
	db   *sqlx.DB
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

func ConfigFromEnv(c *Conf) {
	if c.Abills.DBURL == "" {
		c.Abills.DBURL = os.Getenv("AB_ABILLS_DB")
	}
	if c.TgToken == "" {
		c.TgToken = os.Getenv("AB_TOKEN")
	}
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
