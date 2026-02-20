package mcpserver

import (
	"sync"
	"time"
)

// Cache는 TTL 기반 인메모리 캐시입니다.
// 백엔드 미연결 시 리소스 핸들러의 폴백 데이터를 제공합니다.
type Cache struct {
	mu    sync.RWMutex
	items map[string]*cacheItem
	ttl   time.Duration
}

// cacheItem은 캐시에 저장되는 개별 항목입니다.
type cacheItem struct {
	data      interface{}
	storedAt  time.Time
	expiredAt time.Time
}

// NewCache는 지정된 TTL로 새 캐시를 생성합니다.
func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		items: make(map[string]*cacheItem),
		ttl:   ttl,
	}
}

// Get은 캐시에서 값을 조회합니다.
// 키가 존재하고 만료되지 않았으면 (data, storedAt, true)를 반환합니다.
// 키가 없거나 만료되었으면 (nil, zero, false)를 반환합니다.
func (c *Cache) Get(key string) (interface{}, time.Time, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, time.Time{}, false
	}

	if time.Now().After(item.expiredAt) {
		return nil, time.Time{}, false
	}

	return item.data, item.storedAt, true
}

// GetStale은 만료 여부와 관계없이 캐시에서 값을 조회합니다.
// 백엔드 장애 시 만료된 캐시라도 폴백으로 반환하기 위해 사용합니다.
// 키가 존재하면 (data, storedAt, true)를 반환합니다.
func (c *Cache) GetStale(key string) (interface{}, time.Time, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, time.Time{}, false
	}

	return item.data, item.storedAt, true
}

// Set은 캐시에 값을 저장합니다.
func (c *Cache) Set(key string, data interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	c.items[key] = &cacheItem{
		data:      data,
		storedAt:  now,
		expiredAt: now.Add(c.ttl),
	}
}

// Delete는 캐시에서 키를 삭제합니다.
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
}

// Len은 캐시에 저장된 항목 수를 반환합니다 (만료된 항목 포함).
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.items)
}
