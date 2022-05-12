package abot

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
	"golang.org/x/term"
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
	Names     string `toml:"names"`
}

// Run bot
func Run(c Conf) error {
	var a app
	a.conf = c
	format := "2006-01-02 15:04:05.000"
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		format = "15:04:05.000"
	}
	a.log = &logrus.Logger{
		Out:   os.Stderr,
		Level: logrus.InfoLevel,
		Formatter: &logrus.TextFormatter{
			// DisableColors:   false,
			TimestampFormat: format,
			FullTimestamp:   true,
		},
	}

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
	var infoKb = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("ℹ️ обновить", "info"),
			tgbotapi.NewInlineKeyboardButtonURL("биллинг", a.conf.Abills.WebURL)),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📞 поделиться", "shareph"),
			tgbotapi.NewInlineKeyboardButtonData("🚪 выход", "exit"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("заявка на ремонт", "repair"),
		),
	)

	var contactBut = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButtonContact("\xF0\x9F\x93\x9E Send phone"),
		),
	)

	var validName = regexp.MustCompile(`^[\w]+$`).MatchString
	updates, err := a.bot.GetUpdatesChan(u)
	if err != nil {
		return err
	}
	for update := range updates {
		fromID := 0
		fromStr := ""
		if update.Message != nil {
			fromID = update.Message.From.ID
			fromStr = update.Message.From.String()

		}
		if update.CallbackQuery != nil {
			fromID = update.CallbackQuery.From.ID
			fromStr = update.CallbackQuery.From.String()

		}
		uid, _ := a.findAuth(fromID)

		if uid == 0 && update.Message != nil && update.Message.Contact != nil {
			a.PhoneFirstAuth(update)
		}

		if uid == 0 && update.CallbackQuery != nil &&
			strings.HasPrefix(update.CallbackQuery.Data, "login_") {
			uid = a.PhoneSecondAuth(update)
			if uid == 0 {
				continue
			}
		}

		if uid == 0 && update.CallbackQuery != nil &&
			update.CallbackQuery.Data == "login" {
			a.states.set(update.CallbackQuery.From.ID, "authlogin")
			msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Введите логин")
			a.bot.Send(msg)
			a.log.WithFields(logrus.Fields{"uid": uid, "tg": fromStr}).Info("login")
		}

		state := a.states.get(fromID)
		if update.Message != nil && update.Message.Contact == nil && uid == 0 || state == "authlogin" || state == "authpass" {
			uid = a.loginauth(update)
		}

		if update.CallbackQuery != nil && (update.CallbackQuery.Data == "shareph" ||
			update.CallbackQuery.Data == "phonelogin") {
			msg := tgbotapi.NewMessage(int64(fromID), "📞 нажмите кнопку ниже, чтобы прислать нам свой телефон")
			msg.ReplyMarkup = contactBut
			a.bot.Send(msg)
			continue
		}

		if uid > 0 {
			if update.Message != nil && update.Message.Contact != nil {
				a.log.WithFields(logrus.Fields{"uid": uid, "tg": fromStr, "ph": update.Message.Contact.PhoneNumber}).Info("contact")
				ph := strings.TrimPrefix(update.Message.Contact.PhoneNumber, "+")
				_, err := a.db.Exec("UPDATE users_pi SET _tgphone = ? WHERE uid = ?", ph, uid)
				if err != nil {
					a.log.WithError(err).Warn("update ph")
				}
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Спасибо")
				a.bot.Send(msg)
			}
			if update.CallbackQuery != nil && update.CallbackQuery.Data == "exit" {
				a.logout(uid)
				a.states.set(update.CallbackQuery.From.ID, "authlogin")
				msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Введите логин, либо пришлите контакт")
				msg.ReplyMarkup = contactBut
				a.bot.Send(msg)
				a.log.WithFields(logrus.Fields{"uid": uid, "tg": fromStr}).Info("logout")
				continue
			}

			if update.CallbackQuery != nil && update.CallbackQuery.Data == "login" {
				a.logout(uid)
				a.states.set(update.CallbackQuery.From.ID, "authlogin")
				msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Введите логин")
				a.bot.Send(msg)
				a.log.WithFields(logrus.Fields{"uid": uid, "tg": fromStr}).Info("authbutton")
				continue
			}

			var uinf UserInf
			if a.conf.Abills.Names != "" {
				if validName(a.conf.Abills.Names) {
					a.db.Exec(fmt.Sprintf("SET NAMES %s", a.conf.Abills.Names))
				} else {
					a.log.WithField("names", a.conf.Abills.Names).Warn("wrong names")
				}
			}
			a.db.Get(&uinf,
				`SELECT u.id, u.uid, pi.fio, b.deposit, u.credit, tp.name as tarif FROM users u 
			LEFT JOIN users_pi pi ON pi.uid = u.uid
			LEFT JOIN bills b on b.uid = u.uid
			LEFT JOIN dv_main dv ON dv.uid = u.uid
			LEFT JOIN tarif_plans tp on tp.id = dv.tp_id
			WHERE u.uid = ?`, uid)
			if err != nil {
				a.log.WithError(err).
					WithFields(logrus.Fields{
						"uid": uid,
						"tg":  fromStr,
					}).Warn("db.GetUserInf")
				continue
			}
			txt := fmt.Sprintf("договор: *%s*\nтариф: *%s*\nбаланс: *%0.2f*\nкредит: *%0.2f*", uinf.ID, uinf.TP, uinf.Deposit, uinf.Credit)
			if update.CallbackQuery != nil {
				go func() {
					msg := tgbotapi.NewEditMessageText(
						int64(fromID),
						update.CallbackQuery.Message.MessageID,
						fmt.Sprintf("договор: ⌛\nтариф: *%s*\nбаланс: ⌛\nкредит: ⌛", uinf.TP),
					)
					msg.ReplyMarkup = &infoKb
					msg.ParseMode = "markdown"
					a.bot.Send(msg)
					time.Sleep(time.Second)
					msg = tgbotapi.NewEditMessageText(
						int64(fromID),
						update.CallbackQuery.Message.MessageID,
						txt,
					)
					msg.ReplyMarkup = &infoKb
					msg.ParseMode = "markdown"
					a.bot.Send(msg)
				}()
			}
			if update.Message != nil {
				msg := tgbotapi.NewMessage(
					int64(fromID),
					txt,
				)
				msg.ReplyMarkup = infoKb
				msg.ParseMode = "markdown"
				a.bot.Send(msg)
			}
			a.log.WithFields(logrus.Fields{
				"uid": uid,
				"tg":  fromStr,
			}).Info("db.GetUserInf")
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
	var authKb = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Войти по паролю", "login"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("войти по телефону", "phonelogin"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("заявка на подключение", "connect"),
		),
	)
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
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Неверный логин или пароль.")
			msg.ReplyMarkup = authKb
			a.states.set(update.Message.From.ID, "")
			_, err := a.bot.Send(msg)
			if err != nil {
				a.log.WithError(err).Warn("tgsend")
			}
			a.log.WithFields(logrus.Fields{
				"login": a.states.getVal(int(update.Message.Chat.ID), "login"),
				"tg":    update.Message.From.String(),
			}).Info("auth.wrongpass")
			break
		}
		if uid > 0 {
			_, err := a.db.Exec("REPLACE into tgauth (uid, tgkey, dt) VALUES (?, ?, now())", uid, update.Message.From.ID)
			if err != nil {
				a.log.WithError(err).Warn("replauth")
			}
			a.log.WithFields(logrus.Fields{
				"login": a.states.getVal(int(update.Message.Chat.ID), "login"),
				"uid":   uid,
				"tg":    update.Message.From.String(),
			}).Info("auth.ok")
			return uid
		}

	default:
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Войдите через телефон или логин.")
		msg.ReplyMarkup = authKb
		_, err := a.bot.Send(msg)
		if err != nil {
			a.log.WithError(err).Warn("tgsend")
		}
	}
	return 0
}

func (a *app) PhoneFirstAuth(update tgbotapi.Update) {
	var preauthKb = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Войти по паролю", "login"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("войти по телефону", "phonelogin"),
		),
	)
	users, err := a.findByPhone(update.Message.Contact.PhoneNumber)
	if err != nil {
		a.log.WithError(err).Warn("findByPhone")
	}
	if len(users) > 0 {
		var authbtn []tgbotapi.InlineKeyboardButton
		for i := range users {
			authbtn = append(
				authbtn,
				tgbotapi.NewInlineKeyboardButtonData(
					users[i].ID, fmt.Sprintf("login_%d", users[i].UID)))
			a.states.set(update.Message.From.ID, "phoneauth")
			a.states.addVal(update.Message.From.ID, fmt.Sprintf("login_%d", users[i].UID), update.Message.Contact.PhoneNumber)
		}
		var authkb = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(authbtn...))
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Нашел по телефону")
		msg.ReplyMarkup = authkb
		a.bot.Send(msg)
	}
	if len(users) == 0 {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "В базе нет Вашего телефона")
		msg.ReplyMarkup = preauthKb
		a.bot.Send(msg)
	}
}

func (a *app) PhoneSecondAuth(update tgbotapi.Update) (uid int) {
	preuid, _ := strconv.Atoi(strings.TrimPrefix(update.CallbackQuery.Data, "login_"))
	if preuid == 0 {
		return
	}
	checkphone := a.states.getVal(update.CallbackQuery.From.ID, update.CallbackQuery.Data)
	if checkphone == "" {
		return
	}
	users, err := a.findByPhone(checkphone)
	if err != nil {
		a.log.WithError(err).Warn("findByPhone")
	}
	fnd := false
	for i := range users {
		if users[i].UID == preuid {
			fnd = true
		}
	}
	if fnd {
		uid = preuid
		_, err := a.db.Exec("REPLACE into tgauth (uid, tgkey, dt) VALUES (?, ?, now())", uid, update.CallbackQuery.From.ID)
		if err != nil {
			a.log.WithError(err).Warn("replauth")
		}
		a.log.WithFields(logrus.Fields{
			"uid": uid,
			"tg":  update.CallbackQuery.From.String(),
		}).Info("tel.ok")
		a.states.set(update.CallbackQuery.From.ID, "")
	}
	return
}
