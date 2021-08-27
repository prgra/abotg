package abot

import "sync"

type state map[int]string

type states struct {
	mut  sync.RWMutex
	db   state
	vals map[int]map[string]string
}

func (s *states) get(k int) string {
	s.mut.RLock()
	r := s.db[k]
	s.mut.RUnlock()
	return r
}
func (s *states) getVals(k int) map[string]string {
	s.mut.RLock()
	r := s.vals[k]
	s.mut.RUnlock()
	return r
}

func (s *states) setVals(k int, m map[string]string) {
	s.mut.Lock()
	s.vals[k] = m
	s.mut.Unlock()
}

func (s *states) addVal(id int, k, v string) {
	s.mut.Lock()
	m, ok := s.vals[id]
	if !ok {
		m = make(map[string]string)
	}
	m[k] = v
	s.vals[id] = m
	s.mut.Unlock()
}

func (s *states) getValEx(id int, k string) (v string, ok bool) {
	s.mut.RLock()
	m, ok := s.vals[id]
	s.mut.RUnlock()
	if !ok {
		return
	}
	v, ok = m[k]
	return v, ok
}
func (s *states) getVal(id int, k string) (v string) {
	v, _ = s.getValEx(id, k)
	return v
}

func (s *states) set(k int, v string) bool {
	s.mut.Lock()
	_, ok := s.db[k]
	s.db[k] = v
	s.mut.Unlock()
	return ok
}

func (s *states) Clear(id int) {
	s.mut.Lock()
	delete(s.db, id)
	delete(s.vals, id)
	s.mut.Unlock()
}
