package abot

import "sync"

type state map[int]string

type states struct {
	mut sync.RWMutex
	db  state
}

func (s *states) get(k int) string {
	s.mut.Lock()
	defer s.mut.Unlock()
	return s.db[k]
}

func (s *states) set(k int, v string) bool {
	s.mut.Lock()
	_, ok := s.db[k]
	s.db[k] = v
	s.mut.Unlock()
	return ok
}
