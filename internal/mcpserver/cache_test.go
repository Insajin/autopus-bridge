package mcpserver

import (
	"sync"
	"testing"
	"time"
)

// TestNewCache는 캐시 생성을 테스트합니다.
func TestNewCache(t *testing.T) {
	c := NewCache(30 * time.Second)
	if c == nil {
		t.Fatal("캐시가 nil입니다")
	}
	if c.ttl != 30*time.Second {
		t.Errorf("예상 TTL 30s, 실제: %v", c.ttl)
	}
	if c.Len() != 0 {
		t.Errorf("초기 캐시 크기는 0이어야 합니다, 실제: %d", c.Len())
	}
}

// TestCache_SetAndGet은 캐시 저장 및 조회를 테스트합니다.
func TestCache_SetAndGet(t *testing.T) {
	c := NewCache(1 * time.Minute)

	c.Set("key1", "value1")
	c.Set("key2", 42)

	// 존재하는 키 조회
	val, storedAt, ok := c.Get("key1")
	if !ok {
		t.Fatal("key1이 캐시에 있어야 합니다")
	}
	if val != "value1" {
		t.Errorf("예상 value1, 실제: %v", val)
	}
	if storedAt.IsZero() {
		t.Error("storedAt이 설정되어야 합니다")
	}

	// 정수 값 조회
	val, _, ok = c.Get("key2")
	if !ok {
		t.Fatal("key2가 캐시에 있어야 합니다")
	}
	if val != 42 {
		t.Errorf("예상 42, 실제: %v", val)
	}

	// 존재하지 않는 키 조회
	_, _, ok = c.Get("nonexistent")
	if ok {
		t.Error("존재하지 않는 키는 false를 반환해야 합니다")
	}
}

// TestCache_Expiration은 TTL 만료를 테스트합니다.
func TestCache_Expiration(t *testing.T) {
	c := NewCache(50 * time.Millisecond)

	c.Set("expires", "soon")

	// 만료 전 조회
	_, _, ok := c.Get("expires")
	if !ok {
		t.Fatal("만료 전에는 캐시에 있어야 합니다")
	}

	// TTL 대기
	time.Sleep(100 * time.Millisecond)

	// 만료 후 조회
	_, _, ok = c.Get("expires")
	if ok {
		t.Error("만료 후에는 Get이 false를 반환해야 합니다")
	}
}

// TestCache_GetStale은 만료된 캐시도 조회할 수 있는지 테스트합니다.
func TestCache_GetStale(t *testing.T) {
	c := NewCache(50 * time.Millisecond)

	c.Set("stale", "data")

	// TTL 대기
	time.Sleep(100 * time.Millisecond)

	// Get은 실패해야 함
	_, _, ok := c.Get("stale")
	if ok {
		t.Error("만료 후 Get은 false를 반환해야 합니다")
	}

	// GetStale은 성공해야 함
	val, storedAt, ok := c.GetStale("stale")
	if !ok {
		t.Fatal("만료 후에도 GetStale은 데이터를 반환해야 합니다")
	}
	if val != "data" {
		t.Errorf("예상 data, 실제: %v", val)
	}
	if storedAt.IsZero() {
		t.Error("storedAt이 설정되어야 합니다")
	}

	// 존재하지 않는 키에 대한 GetStale
	_, _, ok = c.GetStale("nonexistent")
	if ok {
		t.Error("존재하지 않는 키에 대한 GetStale은 false를 반환해야 합니다")
	}
}

// TestCache_Delete는 캐시 삭제를 테스트합니다.
func TestCache_Delete(t *testing.T) {
	c := NewCache(1 * time.Minute)

	c.Set("to_delete", "value")

	// 삭제 전 확인
	_, _, ok := c.Get("to_delete")
	if !ok {
		t.Fatal("삭제 전에는 캐시에 있어야 합니다")
	}

	c.Delete("to_delete")

	// 삭제 후 확인
	_, _, ok = c.Get("to_delete")
	if ok {
		t.Error("삭제 후에는 Get이 false를 반환해야 합니다")
	}

	// GetStale도 실패해야 함
	_, _, ok = c.GetStale("to_delete")
	if ok {
		t.Error("삭제 후에는 GetStale도 false를 반환해야 합니다")
	}
}

// TestCache_Overwrite는 같은 키에 대한 덮어쓰기를 테스트합니다.
func TestCache_Overwrite(t *testing.T) {
	c := NewCache(1 * time.Minute)

	c.Set("key", "original")
	val, _, _ := c.Get("key")
	if val != "original" {
		t.Errorf("예상 original, 실제: %v", val)
	}

	c.Set("key", "updated")
	val, _, _ = c.Get("key")
	if val != "updated" {
		t.Errorf("예상 updated, 실제: %v", val)
	}

	if c.Len() != 1 {
		t.Errorf("덮어쓰기 후에도 캐시 크기는 1이어야 합니다, 실제: %d", c.Len())
	}
}

// TestCache_Len은 캐시 크기를 테스트합니다.
func TestCache_Len(t *testing.T) {
	c := NewCache(1 * time.Minute)

	if c.Len() != 0 {
		t.Errorf("초기 크기 0이어야 합니다, 실제: %d", c.Len())
	}

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	if c.Len() != 3 {
		t.Errorf("예상 크기 3, 실제: %d", c.Len())
	}

	c.Delete("b")

	if c.Len() != 2 {
		t.Errorf("예상 크기 2, 실제: %d", c.Len())
	}
}

// TestCache_ConcurrentAccess는 동시 접근 안전성을 테스트합니다.
func TestCache_ConcurrentAccess(t *testing.T) {
	c := NewCache(1 * time.Second)

	var wg sync.WaitGroup
	const goroutines = 100
	const iterations = 100

	// 동시에 Set 실행
	for i := range goroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range iterations {
				key := "key"
				c.Set(key, id*iterations+j)
			}
		}(i)
	}

	// 동시에 Get 실행
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iterations {
				c.Get("key")
			}
		}()
	}

	// 동시에 GetStale 실행
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iterations {
				c.GetStale("key")
			}
		}()
	}

	wg.Wait()

	// 데드락이나 패닉 없이 완료되면 성공
}

// TestCache_StructValue는 구조체 포인터를 캐시에 저장하고 조회하는 것을 테스트합니다.
func TestCache_StructValue(t *testing.T) {
	c := NewCache(1 * time.Minute)

	status := &PlatformStatus{
		Connected:  true,
		BackendURL: "http://localhost:8080",
		ServerName: "test",
		Version:    "1.0.0",
		Message:    "ok",
	}

	c.Set("status", status)

	val, _, ok := c.Get("status")
	if !ok {
		t.Fatal("캐시에 status가 있어야 합니다")
	}

	retrieved, ok := val.(*PlatformStatus)
	if !ok {
		t.Fatal("PlatformStatus 타입이어야 합니다")
	}
	if retrieved.Connected != true {
		t.Error("Connected가 true이어야 합니다")
	}
	if retrieved.BackendURL != "http://localhost:8080" {
		t.Errorf("예상 BackendURL http://localhost:8080, 실제: %s", retrieved.BackendURL)
	}
}
