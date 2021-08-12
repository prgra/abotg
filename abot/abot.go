package abot

import (
	"fmt"
	"os"

	_ "github.com/go-sql-driver/mysql"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
)

type app struct {
	conf   Conf
	bot    *tgbotapi.BotAPI
	states states
	db     *sqlx.DB
	log    *logrus.Logger
}

func (a *app) run() {
	a.msgLoop()

}

type UserInf struct {
	ID      string  `db:"id"`
	UID     int     `db:"uid"`
	FIO     string  `db:"fio"`
	Deposit float64 `db:"deposit"`
	Credit  float64 `db:"credit"`
	TP      string  `db:"tarif"`
}

// Conf config
type Conf struct {
	Abills  Abills `toml:"abills"`
	TgToken string `toml:"tgtoken"`
}

// Abills config part for billing
type Abills struct {
	// DBURL::mysql abills url
	DBURL     string `toml:"dburl"`
	SecretKey string `toml:"secretkey"`
	WebURL    string `toml:"url"`
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
	a.db, err = sqlx.Connect("mysql", c.Abills.DBURL)
	if err != nil {
		return err
	}
	a.db.Exec("SET NAMES latin1")
	a.states.db = make(state)
	a.states.vals = make(map[int]map[string]string)
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

func (a *app) msgLoop() error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	var numericKeyboard = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("ℹ️ информация", "info"),
			tgbotapi.NewInlineKeyboardButtonURL("биллинг", a.conf.Abills.WebURL),
			tgbotapi.NewInlineKeyboardButtonData("🚪 выход", "exit"),
		),
	)
	updates, err := a.bot.GetUpdatesChan(u)
	if err != nil {
		return err
	}
	for update := range updates {
		fromID := 0
		if update.Message != nil {
			fromID = update.Message.From.ID
		}
		if update.CallbackQuery != nil {
			fromID = update.CallbackQuery.From.ID
		}
		uid, _ := a.findAuth(fromID)

		state := a.states.get(fromID)
		if uid == 0 || state == "authlogin" || state == "authpass" {
			uid = a.loginauth(update)
		}
		if uid > 0 {
			if update.CallbackQuery != nil && update.CallbackQuery.Data == "exit" {
				a.logout(uid)
				a.states.set(update.CallbackQuery.From.ID, "authlogin")
				msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Введите логин")
				a.bot.Send(msg)
				continue
			}
			var uinf UserInf
			a.db.Get(&uinf,
				`SELECT u.id, u.uid, pi.fio, b.deposit, u.credit, tp.name as tarif FROM users u 
			JOIN users_pi pi ON pi.uid = u.uid
			JOIN bills b on b.uid = u.uid
			JOIN dv_main dv ON dv.uid = u.uid
			JOIN tarif_plans tp on tp.id = dv.tp_id
			WHERE u.uid = ?`, uid)
			txt := fmt.Sprintf("договор: *%s*\nТариф: *%s*\nбаланс: *%0.2f*\nкредит: *%0.2f*", uinf.ID, uinf.TP, uinf.Deposit, uinf.Credit)
			msg := tgbotapi.NewMessage(int64(fromID), txt)
			msg.ReplyMarkup = numericKeyboard
			msg.ParseMode = "markdown"
			a.bot.Send(msg)
		}
	}
	return err
}

func (a *app) logout(uid int) error {
	if uid == 0 {
		return fmt.Errorf("no such uid")
	}
	_, err := a.db.Exec("DELETE FROM tgauth WHERE uid = ?", uid)
	return err
}

func (a *app) loginauth(update tgbotapi.Update) (uid int) {
	if update.Message == nil {
		return
	}
	switch a.states.get(update.Message.From.ID) {
	case "authlogin":
		a.states.addVal(update.Message.From.ID, "login", update.Message.Text)
		a.states.set(update.Message.From.ID, "authpass")
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Введите пароль")
		a.bot.Send(msg)
	case "authpass":
		a.states.addVal(update.Message.From.ID, "pass", update.Message.Text)
		a.states.set(update.Message.From.ID, "")
		var uid int
		err := a.db.Get(&uid, "SELECT uid FROM users WHERE id = ? AND password = ENCODE(?, ?) AND deleted+disable=0 and company_id=0",
			a.states.getVal(int(update.Message.Chat.ID), "login"),
			a.states.getVal(int(update.Message.Chat.ID), "pass"),
			a.conf.Abills.SecretKey,
		)
		if err != nil {
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "неверный логин или пароль\nВведите логин")
			a.states.set(update.Message.From.ID, "authlogin")

			a.bot.Send(msg)
			break
		}
		if uid > 0 {
			_, err := a.db.Exec("REPLACE into tgauth (uid, tgkey, dt) VALUES (?, ?, now())", uid, update.Message.From.ID)
			if err != nil {
				a.log.WithError(err).Warn("replauth")
			}
			return uid
		}

	default:
		a.states.set(update.Message.From.ID, "authlogin")
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Введите логин")
		_, err := a.bot.Send(msg)
		if err != nil {
			a.log.WithError(err).Warn("tgsend")
		}
	}
	return 0
}
