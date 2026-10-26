package httpapi

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"

	"github.com/judy98-s/claw-hub/internal/store"
)

type settlementResponse struct {
	Period string `json:"period"`
	From   string `json:"from"`
	To     string `json:"to"`

	ClaimCount   int `json:"claimCount"`
	PaidCount    int `json:"paidCount"`
	PaidTotalKRW int `json:"paidTotalKrw"`

	PrevClaimCount   int `json:"prevClaimCount"`
	PrevPaidCount    int `json:"prevPaidCount"`
	PrevPaidTotalKRW int `json:"prevPaidTotalKrw"`

	Days     []settlementDay     `json:"days"`
	Machines []settlementMachine `json:"machines"`

	PaidByCash    int `json:"paidByCash"`
	PaidByCard    int `json:"paidByCard"`
	RejectedCount int `json:"rejectedCount"`
	OpenCount     int `json:"openCount"`
}

type settlementDay struct {
	Day          string `json:"day"`
	ClaimCount   int    `json:"claimCount"`
	PaidTotalKRW int    `json:"paidTotalKrw"`
}

type settlementMachine struct {
	MachineID    string `json:"machineId"`
	Label        string `json:"label"`
	ClaimCount   int    `json:"claimCount"`
	PaidTotalKRW int    `json:"paidTotalKrw"`
	NeedsCheck   bool   `json:"needsCheck"`
}

// periodOf는 쿼리에서 기간 이름을 읽는다. 모르는 값은 이번 주다 —
// 400 을 주면 화면이 빈 채로 뜨고, 사장님은 고장인 줄 안다.
func periodOf(r *http.Request) string {
	switch p := r.URL.Query().Get("period"); p {
	case "month", "last_month", "week":
		return p
	default:
		return "week"
	}
}

func (s *Server) handleSettlement(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())
	period := periodOf(r)
	from, to := store.SettlementRange(period, s.now())

	st, err := s.store.SettlementFor(r.Context(), u.StoreID, from, to, s.cfg.Policy.MachineAlertCount)
	if err != nil {
		internalError(w, r, err, "정산 집계 실패")
		return
	}

	res := settlementResponse{
		Period: period,
		From:   st.From.Format("2006-01-02"),
		To:     st.To.Format("2006-01-02"),

		ClaimCount: st.ClaimCount, PaidCount: st.PaidCount, PaidTotalKRW: st.PaidTotalKRW,
		PrevClaimCount: st.PrevClaimCount, PrevPaidCount: st.PrevPaidCount,
		PrevPaidTotalKRW: st.PrevPaidTotalKRW,

		PaidByCash: st.PaidByCash, PaidByCard: st.PaidByCard,
		RejectedCount: st.RejectedCount, OpenCount: st.OpenCount,

		Days:     make([]settlementDay, 0, len(st.Days)),
		Machines: make([]settlementMachine, 0, len(st.Machines)),
	}
	for _, d := range st.Days {
		res.Days = append(res.Days, settlementDay{
			Day: d.Day.Format("2006-01-02"), ClaimCount: d.ClaimCount, PaidTotalKRW: d.PaidTotalKRW,
		})
	}
	for _, m := range st.Machines {
		res.Machines = append(res.Machines, settlementMachine{
			MachineID: m.MachineID, Label: m.Label, ClaimCount: m.ClaimCount,
			PaidTotalKRW: m.PaidTotalKRW, NeedsCheck: m.NeedsCheck,
		})
	}
	writeJSON(w, http.StatusOK, res)
}

// handleSettlementCSV는 기계별 집계를 CSV 로 내려준다.
//
// XLSX 를 만들려면 의존성이 생기고, 엑셀은 CSV 를 연다. 대신 BOM 을
// 붙인다 — 없으면 엑셀이 한글을 깨뜨려 연다. 세무사에게 넘길 파일이라
// 열리자마자 깨져 있으면 쓸 수 없다.
func (s *Server) handleSettlementCSV(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())
	period := periodOf(r)
	from, to := store.SettlementRange(period, s.now())

	st, err := s.store.SettlementFor(r.Context(), u.StoreID, from, to, s.cfg.Policy.MachineAlertCount)
	if err != nil {
		internalError(w, r, err, "정산 집계 실패")
		return
	}

	name := fmt.Sprintf("claw-hub-%s_%s.csv", st.From.Format("20060102"), st.To.Format("20060102"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")

	if _, err := w.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return
	}
	cw := csv.NewWriter(w)
	defer cw.Flush()

	_ = cw.Write([]string{"구분", "이름", "민원 수", "환불액(원)"})
	for _, m := range st.Machines {
		_ = cw.Write([]string{"기계", m.Label, strconv.Itoa(m.ClaimCount), strconv.Itoa(m.PaidTotalKRW)})
	}
	for _, d := range st.Days {
		_ = cw.Write([]string{"날짜", d.Day.Format("2006-01-02"),
			strconv.Itoa(d.ClaimCount), strconv.Itoa(d.PaidTotalKRW)})
	}
	_ = cw.Write([]string{"합계", "", strconv.Itoa(st.ClaimCount), strconv.Itoa(st.PaidTotalKRW)})
	_ = cw.Write([]string{"계좌 송금", "", strconv.Itoa(st.PaidByCash), ""})
	_ = cw.Write([]string{"카드 취소", "", strconv.Itoa(st.PaidByCard), ""})
	_ = cw.Write([]string{"거절", "", strconv.Itoa(st.RejectedCount), ""})
	_ = cw.Write([]string{"대기", "", strconv.Itoa(st.OpenCount), ""})
}
