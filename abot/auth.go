package abot

func (a *app) findAuth(id int) (uid int, err error) {
	err = a.db.Get(&uid, "SELECT uid FROM tgauth WHERE tgkey=?", id)
	return
}
