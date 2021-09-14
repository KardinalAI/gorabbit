package gorabbit

import (
	"sync"
	"time"
)

type TTLMap struct {
	m map[uint64]time.Time
	l sync.Mutex
}

func NewTTLMap(ln int, maxTTL time.Duration) (m *TTLMap) {
	m = &TTLMap{m: make(map[uint64]time.Time, ln)}
	go func() {
		for now := range time.Tick(maxTTL / 3) {
			m.l.Lock()
			for k, v := range m.m {
				if now.Sub(v) >= maxTTL {
					delete(m.m, k)
				}
			}
			m.l.Unlock()
		}
	}()
	return
}

func (m *TTLMap) Len() int {
	return len(m.m)
}

func (m *TTLMap) Put(k uint64) {
	m.l.Lock()
	_, ok := m.m[k]
	if !ok {
		m.m[k] = time.Now()
	}
	m.l.Unlock()
}

func (m *TTLMap) Get(k uint64) (v time.Time, found bool) {
	m.l.Lock()
	if it, ok := m.m[k]; ok {
		v = it
		found = true
	}
	found = false
	m.l.Unlock()
	return
}
