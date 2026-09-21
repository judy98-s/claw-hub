package cache

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestMemory_Incr(t *testing.T) {
	ctx := context.Background()
	c := NewMemory()

	for want := int64(1); want <= 3; want++ {
		got, err := c.Incr(ctx, "k", time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("Incr = %d, want %d", got, want)
		}
	}
}

func TestMemory_Incr_TTL이_지나면_리셋(t *testing.T) {
	ctx := context.Background()
	c := NewMemory()

	if _, err := c.Incr(ctx, "k", 30*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Incr(ctx, "k", 30*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(60 * time.Millisecond)

	got, err := c.Incr(ctx, "k", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 {
		t.Errorf("TTL 경과 후 Incr = %d, want 1", got)
	}
}

func TestMemory_Incr_TTL은_첫_증가때만_설정된다(t *testing.T) {
	// 매 요청마다 TTL이 갱신되면 레이트리밋 창이 영원히 밀려서,
	// 계속 두드리는 클라이언트는 절대 차단이 풀리지 않는다.
	ctx := context.Background()
	c := NewMemory()

	for i := 0; i < 3; i++ {
		if _, err := c.Incr(ctx, "k", 60*time.Millisecond); err != nil {
			t.Fatal(err)
		}
		time.Sleep(25 * time.Millisecond)
	}
	// 총 75ms 경과. 첫 증가 기준 TTL 60ms 는 이미 지났어야 한다.
	got, err := c.Incr(ctx, "k", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 {
		t.Errorf("Incr = %d, want 1 — TTL이 매번 갱신되고 있다", got)
	}
}

func TestMemory_SetNX(t *testing.T) {
	ctx := context.Background()
	c := NewMemory()

	ok, err := c.SetNX(ctx, "k", "v1", time.Hour)
	if err != nil || !ok {
		t.Fatalf("첫 SetNX = %v, %v", ok, err)
	}
	ok, err = c.SetNX(ctx, "k", "v2", time.Hour)
	if err != nil || ok {
		t.Fatalf("두 번째 SetNX = %v, %v — 덮어썼다", ok, err)
	}

	v, found, err := c.Get(ctx, "k")
	if err != nil || !found || v != "v1" {
		t.Errorf("Get = %q, %v, %v — 최초 값이 유지되어야 한다", v, found, err)
	}
}

func TestMemory_SetNX_TTL_만료후_다시_설정가능(t *testing.T) {
	ctx := context.Background()
	c := NewMemory()

	if ok, _ := c.SetNX(ctx, "k", "v", 30*time.Millisecond); !ok {
		t.Fatal("첫 SetNX 실패")
	}
	time.Sleep(60 * time.Millisecond)
	if ok, _ := c.SetNX(ctx, "k", "v2", time.Hour); !ok {
		t.Error("TTL 만료 후에도 SetNX가 실패한다")
	}
}

func TestMemory_Get_없는키(t *testing.T) {
	_, found, err := NewMemory().Get(context.Background(), "없음")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("없는 키가 found=true")
	}
}

func TestMemory_동시_Incr(t *testing.T) {
	ctx := context.Background()
	c := NewMemory()

	const n = 1000
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.Incr(ctx, "k", time.Hour); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	got, err := c.Incr(ctx, "k", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if got != n+1 {
		t.Errorf("동시 %d회 후 = %d, want %d", n, got, n+1)
	}
}

func TestNew_빈URL이면_인메모리(t *testing.T) {
	// Redis 미설치나 장애가 접수 실패로 번지면 안 된다.
	c, err := New("")
	if err != nil {
		t.Fatalf("빈 URL에서 에러: %v", err)
	}
	if _, ok := c.(*Memory); !ok {
		t.Errorf("빈 URL인데 %T 가 나왔다", c)
	}
}
