package abot

import (
	"fmt"
	"regexp"

	"github.com/sirupsen/logrus"
)

func (a *app) GetUserInfo(id int, fromStr string) (uinf UserInf, err error) {
	if id <= 0 {
		return uinf, fmt.Errorf("need id > 0")
	}
	var validName = regexp.MustCompile(`^[\w]+$`).MatchString

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
			WHERE u.uid = ?`, id)
	if err != nil {
		a.log.WithError(err).
			WithFields(logrus.Fields{
				"uid": id,
				"tg":  fromStr,
			}).Warn("db.GetUserInf")
		return uinf, err
	}
	return uinf, nil
}
