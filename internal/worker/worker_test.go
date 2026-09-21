package worker

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/judy98-s/claw-hub/internal/cache"
	"github.com/judy98-s/claw-hub/internal/domain"
	"github.com/judy98-s/claw-hub/internal/notify"
	"github.com/judy98-s/claw-hub/internal/store"
)

type fakeStore struct {
	machines []store.MachineActivity
	digest   []store.DigestRow
	photos   []store.Photo

	deletedRows []string
	deleteErr   error
}

func (f *fakeStore) MachinesOverThreshold(_ context.Context, threshold int, _ time.Time) ([]store.MachineActivity, error) {
	out := []store.MachineActivity{}
	for _, m := range f.machines {
		if m.Count >= threshold {
			out = append(out, m)
		}
	}
	return out, nil
}

func (f *fakeStore) DailyDigest(context.Context, time.Time) ([]store.DigestRow, error) {
	return f.digest, nil
}

func (f *fakeStore) ExpiredPhotos(_ context.Context, now time.Time, limit int) ([]store.Photo, error) {
	out := []store.Photo{}
	for _, p := range f.photos {
		if !p.ExpiresAt.After(now) && len(out) < limit {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeStore) DeletePhotoRow(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deletedRows = append(f.deletedRows, id)
	for i, p := range f.photos {
		if p.ID == id {
			f.photos = append(f.photos[:i], f.photos[i+1:]...)
			break
		}
	}
	return nil
}

type fakeMedia struct {
	mu        sync.Mutex
	deleted   []string
	deleteErr error
}

func (m *fakeMedia) Put(context.Context, string, io.Reader, string, int64) error { return nil }
func (m *fakeMedia) Open(context.Context, string) (io.ReadCloser, string, error) {
	return nil, "", errors.New("없음")
}
func (m *fakeMedia) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.deleted = append(m.deleted, key)
	return nil
}

type fakeNotifier struct {
	mu       sync.Mutex
	machines []notify.MachineNotice
	digests  []notify.DigestNotice
}

func (n *fakeNotifier) ClaimCreated(context.Context, notify.ClaimNotice) error { return nil }
func (n *fakeNotifier) MachineAlert(_ context.Context, m notify.MachineNotice) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.machines = append(n.machines, m)
	return nil
}
func (n *fakeNotifier) DailyDigest(_ context.Context, d notify.DigestNotice) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.digests = append(n.digests, d)
	return nil
}

func newWorker(st *fakeStore, threshold int) (*Worker, *fakeMedia, *fakeNotifier) {
	md := &fakeMedia{}
	nt := &fakeNotifier{}
	return New(st, cache.NewMemory(), md, nt, threshold), md, nt
}

func TestCheckMachineAlerts_임계치_경계(t *testing.T) {
	tests := []struct {
		count     int
		wantAlert bool
	}{{1, false}, {2, false}, {3, true}, {5, true}}

	for _, tc := range tests {
		st := &fakeStore{machines: []store.MachineActivity{{
			MachineID: "m1", Label: "3번 기계", Count: tc.count,
			ByIssue: map[domain.IssueType]int{domain.IssueCashEaten: tc.count},
		}}}
		w, _, nt := newWorker(st, 3)

		if err := w.CheckMachineAlerts(context.Background(), time.Now()); err != nil {
			t.Fatal(err)
		}
		got := len(nt.machines) > 0
		if got != tc.wantAlert {
			t.Errorf("%d건: 알림 = %v, want %v", tc.count, got, tc.wantAlert)
		}
	}
}

func TestCheckMachineAlerts_같은_기계는_하루에_한번만(t *testing.T) {
	// 임계치를 넘은 뒤로는 매 실행마다 조건이 참이다. 디듀프가 없으면
	// 5분마다 같은 알림이 오고, 사장님은 알림을 꺼버린다.
	st := &fakeStore{machines: []store.MachineActivity{{
		MachineID: "m1", Label: "3번 기계", Count: 5,
		ByIssue: map[domain.IssueType]int{domain.IssueCashEaten: 5},
	}}}
	w, _, nt := newWorker(st, 3)
	now := time.Now()

	for i := 0; i < 10; i++ {
		if err := w.CheckMachineAlerts(context.Background(), now); err != nil {
			t.Fatal(err)
		}
	}
	if len(nt.machines) != 1 {
		t.Errorf("알림 %d건, want 1 — 사장님이 알림을 끄게 된다", len(nt.machines))
	}
}

func TestCheckMachineAlerts_날짜가_바뀌면_다시_알린다(t *testing.T) {
	st := &fakeStore{machines: []store.MachineActivity{{
		MachineID: "m1", Label: "3번 기계", Count: 5,
		ByIssue: map[domain.IssueType]int{},
	}}}
	w, _, nt := newWorker(st, 3)

	day1 := time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC)
	w.CheckMachineAlerts(context.Background(), day1)                  //nolint:errcheck
	w.CheckMachineAlerts(context.Background(), day1.Add(time.Hour))   //nolint:errcheck
	w.CheckMachineAlerts(context.Background(), day1.AddDate(0, 0, 1)) //nolint:errcheck

	if len(nt.machines) != 2 {
		t.Errorf("알림 %d건, want 2 (하루에 한 번씩 이틀)", len(nt.machines))
	}
}

func TestCheckMachineAlerts_내용이_충분하다(t *testing.T) {
	st := &fakeStore{machines: []store.MachineActivity{{
		MachineID: "m1", Label: "3번 기계", Count: 3,
		ByIssue: map[domain.IssueType]int{domain.IssueCashEaten: 2, domain.IssueClawBroken: 1},
	}}}
	w, _, nt := newWorker(st, 3)
	w.CheckMachineAlerts(context.Background(), time.Now()) //nolint:errcheck

	if len(nt.machines) != 1 {
		t.Fatalf("알림 %d건", len(nt.machines))
	}
	got := nt.machines[0]
	if got.MachineLabel != "3번 기계" || got.Count != 3 {
		t.Errorf("알림 = %+v", got)
	}
	if got.ByIssue[domain.IssueCashEaten] != 2 {
		t.Errorf("증상별 집계가 빠졌다: %+v — 사장님이 뭘 고칠지 모른다", got.ByIssue)
	}
}

func TestPurgeExpiredPhotos(t *testing.T) {
	now := time.Now()
	st := &fakeStore{photos: []store.Photo{
		{ID: "p1", ObjectKey: "a.jpg", ExpiresAt: now.Add(-time.Hour)},
		{ID: "p2", ObjectKey: "b.jpg", ExpiresAt: now.Add(-time.Minute)},
		{ID: "p3", ObjectKey: "c.jpg", ExpiresAt: now.Add(time.Hour)},
	}}
	w, md, _ := newWorker(st, 3)

	n, err := w.PurgeExpiredPhotos(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("삭제 %d건, want 2", n)
	}
	if len(md.deleted) != 2 {
		t.Errorf("파일 삭제 %d건, want 2", len(md.deleted))
	}
	if len(st.deletedRows) != 2 {
		t.Errorf("DB 행 삭제 %d건, want 2", len(st.deletedRows))
	}
	// 아직 만료되지 않은 사진은 남아야 한다.
	if len(st.photos) != 1 || st.photos[0].ID != "p3" {
		t.Errorf("남은 사진 = %+v", st.photos)
	}
}

func TestPurgeExpiredPhotos_파일이_없어도_DB행은_지운다(t *testing.T) {
	// 행을 남겨두면 워커가 매 실행마다 같은 사진을 다시 시도하며 로그만 채운다.
	now := time.Now()
	st := &fakeStore{photos: []store.Photo{
		{ID: "p1", ObjectKey: "없는파일.jpg", ExpiresAt: now.Add(-time.Hour)},
	}}
	w, md, _ := newWorker(st, 3)
	md.deleteErr = errors.New("파일이 없습니다")

	n, err := w.PurgeExpiredPhotos(context.Background(), now)
	if err != nil {
		t.Fatalf("파일 삭제 실패가 전체를 막았다: %v", err)
	}
	if n != 1 {
		t.Errorf("삭제 %d건, want 1", n)
	}
	if len(st.deletedRows) != 1 {
		t.Error("파일 삭제 실패로 DB 행이 남았다 — 영원히 재시도된다")
	}
}

func TestPurgeExpiredPhotos_만료된게_없으면_0(t *testing.T) {
	st := &fakeStore{photos: []store.Photo{
		{ID: "p1", ObjectKey: "a.jpg", ExpiresAt: time.Now().Add(time.Hour)},
	}}
	w, _, _ := newWorker(st, 3)

	n, err := w.PurgeExpiredPhotos(context.Background(), time.Now())
	if err != nil || n != 0 {
		t.Errorf("n = %d, err = %v", n, err)
	}
}

func TestSendDailyDigest(t *testing.T) {
	st := &fakeStore{digest: []store.DigestRow{{
		StoreID: "store-1", ClaimCount: 12, PaidCount: 9, PaidTotalKRW: 23000,
		TopMachines: []store.MachineCount{{Label: "3번 기계", Count: 5}},
	}}}
	w, _, nt := newWorker(st, 3)

	day := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	if err := w.SendDailyDigest(context.Background(), day); err != nil {
		t.Fatal(err)
	}
	if len(nt.digests) != 1 {
		t.Fatalf("요약 %d건", len(nt.digests))
	}
	got := nt.digests[0]
	if got.Day != "2026-09-20" {
		t.Errorf("Day = %q", got.Day)
	}
	if got.ClaimCount != 12 || got.PaidCount != 9 || got.PaidTotalKRW != 23000 {
		t.Errorf("집계 = %+v", got)
	}
	if len(got.TopMachines) != 1 || got.TopMachines[0].Label != "3번 기계" {
		t.Errorf("상위 기계 = %+v", got.TopMachines)
	}
}

func TestRun_컨텍스트_취소로_종료된다(t *testing.T) {
	w, _, _ := newWorker(&fakeStore{}, 3)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		w.Run(ctx, 10*time.Millisecond)
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("취소 후에도 Run이 끝나지 않는다")
	}
}
