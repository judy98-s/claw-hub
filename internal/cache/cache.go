// Package cache는 레이트리밋과 멱등성에 쓰는 짧은 수명의 키-값 저장소다.
//
// Redis가 없어도 동작한다. 캐시는 부가 방어선이고, 이중 환불의 실제 방어는
// DB 유니크 제약이 한다. 캐시 장애가 접수 실패로 번지면 안 된다.
package cache

import (
	"context"
	"sync"
	"time"
)

// Cache는 이 시스템이 캐시에 요구하는 전부다.
type Cache interface {
	// Incr는 키를 1 증가시키고 증가 후 값을 반환한다.
	// TTL은 키가 처음 만들어질 때만 설정된다.
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
	// SetNX는 키가 없을 때만 설정하고 설정 여부를 반환한다.
	SetNX(ctx context.Context, key, val string, ttl time.Duration) (bool, error)
	Get(ctx context.Context, key string) (string, bool, error)
	Close() error
}

// New는 URL이 비었으면 인메모리, 아니면 Redis 캐시를 만든다.
func New(url string) (Cache, error) {
	if url == "" {
		return NewMemory(), nil
	}
	return NewRedis(url)
}

type entry struct {
	val       string
	count     int64
	expiresAt time.Time
}

// Memory는 단일 프로세스용 인메모리 캐시다.
//
// 인스턴스를 여러 대 띄우면 레이트리밋이 인스턴스마다 따로 세어진다.
// 그 규모가 되면 REDIS_URL을 설정하면 된다.
type Memory struct {
	mu   sync.Mutex
	data map[string]*entry
	now  func() time.Time
}

func NewMemory() *Memory {
	return &Memory{data: map[string]*entry{}, now: time.Now}
}

// sweep은 만료된 키를 지운다. 호출자가 락을 들고 있어야 한다.
func (m *Memory) sweep(now time.Time) {
	for k, e := range m.data {
		if now.After(e.expiresAt) {
			delete(m.data, k)
		}
	}
}

func (m *Memory) Incr(_ context.Context, key string, ttl time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()

	e, ok := m.data[key]
	if !ok || now.After(e.expiresAt) {
		// TTL은 여기서만 설정한다. 매 증가마다 갱신하면 레이트리밋 창이
		// 영원히 밀려서, 계속 두드리는 클라이언트는 차단이 안 풀린다.
		m.data[key] = &entry{count: 1, expiresAt: now.Add(ttl)}
		return 1, nil
	}
	e.count++
	return e.count, nil
}

func (m *Memory) SetNX(_ context.Context, key, val string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()

	if e, ok := m.data[key]; ok && !now.After(e.expiresAt) {
		return false, nil
	}
	m.data[key] = &entry{val: val, expiresAt: now.Add(ttl)}

	// 쓰기 때 가끔 청소한다. 전용 고루틴을 두면 종료 관리가 붙는데,
	// 이 규모에서는 그만한 가치가 없다.
	if len(m.data) > 1000 {
		m.sweep(now)
	}
	return true, nil
}

func (m *Memory) Get(_ context.Context, key string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	e, ok := m.data[key]
	if !ok || m.now().After(e.expiresAt) {
		return "", false, nil
	}
	return e.val, true, nil
}

func (m *Memory) Close() error { return nil }
