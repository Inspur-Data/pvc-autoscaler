package utils

import (
	"sync"
	"time"
)

type item struct {
	expiredTime time.Time
	val         interface{}
}

type ExpiredMap struct {
	expiredMap *sync.Map
}

func (m *ExpiredMap) initMap() {
	if m.expiredMap == nil {
		m.expiredMap = &sync.Map{}
	}
}

func (m *ExpiredMap) Load(key interface{}) (value interface{}, ok bool) {
	m.initMap()
	value, ok = m.expiredMap.Load(key)
	//不存在
	if !ok {
		return nil, ok
	}
	item := value.(item)
	//未过期
	if item.expiredTime.After(time.Now()) {
		return item.val, ok
	}
	return nil, false
}

func (m *ExpiredMap) Store(key, value interface{}, duration time.Duration) {
	m.initMap()
	item := item{
		expiredTime: time.Now().Add(duration),
		val:         value,
	}
	m.expiredMap.Store(key, item)
}
