package abot

import (
	"fmt"
	"strconv"
	"strings"
)

func (a *app) findAuth(id int) (uid int, err error) {
	err = a.db.Get(&uid, "SELECT uid FROM tgauth WHERE tgkey=?", id)
	return
}

type shortuser struct {
	ID  string `db:"id"`
	UID int    `db:"uid"`
}

func (a *app) findByPhone(phone string) (users []shortuser, err error) {
	if len(phone) < 11 {
		return users, fmt.Errorf("phone to short")
	}
	ip, _ := strconv.Atoi(phone)
	if ip == 0 {
		return users, fmt.Errorf("phone not number")
	}
	phone = strings.TrimPrefix(phone, "+")
	phone = strings.TrimPrefix(phone, "7")

	err = a.db.Select(&users, fmt.Sprintf(`SELECT u.id, u.uid 
	FROM users u 
	JOIN users_pi pi ON pi.uid = u.uid 
	WHERE pi.phone like '%%%s%%' OR pi._tgphone like '%%%s%%'`, phone, phone))
	if err != nil {
		return
	}
	return
}
