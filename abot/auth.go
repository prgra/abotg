package abot

func (a *app) findAuth(id string) (uid int, err error) {
	err = a.db.Select(&uid, "SELECT uid FROM tgauth WHERE tgkey=?", id)
	return
}
