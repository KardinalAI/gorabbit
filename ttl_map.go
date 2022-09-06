package gorabbit

import (
	"sync"
	"time"
)

type ttlMapValue[V any] struct {
	value     V
	createdAt time.Time
}

type ttlMap[K, V any] struct {
	m map[any]ttlMapValue[V]
	l sync.Mutex
}

func newTTLMap[K, V any](ln uint64, maxTTL time.Duration) *ttlMap[K, V] {
	m := &ttlMap[K, V]{m: make(map[any]ttlMapValue[V], ln)}

	go func() {
		const tickFraction = 3

		for now := range time.Tick(maxTTL / tickFraction) {
			m.l.Lock()
			for k := range m.m {
				issueDate := m.m[k].createdAt
				if now.Sub(issueDate) >= maxTTL {
					delete(m.m, k)
				}
			}
			m.l.Unlock()
		}
	}()

	return m
}

func (m *ttlMap[K, V]) Len() int {
	return len(m.m)
}

func (m *ttlMap[K, V]) Put(k K, v V) {
	m.l.Lock()

	defer m.l.Unlock()

	if _, ok := m.m[k]; !ok {
		m.m[k] = ttlMapValue[V]{value: v, createdAt: time.Now()}
	}
}

func (m *ttlMap[K, V]) Get(k K) (V, bool) {
	m.l.Lock()

	defer m.l.Unlock()

	v, found := m.m[k]

	innerVal := v.value

	return innerVal, found
}

func (m *ttlMap[K, V]) ForEach(process func(k K, v V)) {
	for key, value := range m.m {
		innerVal := value.value

		process(key.(K), innerVal)
	}
}

func (m *ttlMap[K, V]) Delete(k K) {
	delete(m.m, k)
}
