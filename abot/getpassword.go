package abot

import "fmt"

func (a *app) GetPassword(id int) (pass string, err error) {
	if id <= 0 {
		return "", fmt.Errorf("need id > 0")
	}
	err = a.db.Get(&pass, "SELECT DECODE(password, ?) FROM users WHERE uid=?", a.conf.Abills.SecretKey, id)
	return pass, err
}
