package order

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/ordering/internal/httpx"
)

// fakeRepository は試験用の Repository。
//
// 受け取った引数を記録し、あらかじめ決めた結果を返す。
type fakeRepository struct {
	check  Check
	checks []Check
	order  Order
	err    error

	gotTableNo int
	gotStatus  *Status
	gotID      int64
	gotLines   []Line
}

func (f *fakeRepository) Open(_ context.Context, tableNo int) (Check, error) {
	f.gotTableNo = tableNo
	return f.check, f.err
}

func (f *fakeRepository) List(_ context.Context, status *Status) ([]Check, error) {
	f.gotStatus = status
	return f.checks, f.err
}

func (f *fakeRepository) Get(_ context.Context, id int64) (Check, error) {
	f.gotID = id
	return f.check, f.err
}

func (f *fakeRepository) AddOrder(_ context.Context, checkID int64, lines []Line) (Order, error) {
	f.gotID = checkID
	f.gotLines = lines
	return f.order, f.err
}

func (f *fakeRepository) Pay(_ context.Context, id int64) (Check, error) {
	f.gotID = id
	return f.check, f.err
}

func (f *fakeRepository) Close(_ context.Context, id int64) (Check, error) {
	f.gotID = id
	return f.check, f.err
}

// serve は Handler を登録したルータに要求を 1 つ送り、応答を返す。
func serve(t *testing.T, repo Repository, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	mux := http.NewServeMux()
	NewHandler(repo).Register(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, target, strings.NewReader(body)))
	return rec
}

func TestHandlerOpen(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		repoErr    error
		wantStatus int
	}{
		{"伝票を開く", `{"tableNo": 3}`, nil, http.StatusCreated},
		{"テーブル番号なし", `{}`, nil, http.StatusBadRequest},
		{"テーブル番号が 0", `{"tableNo": 0}`, nil, http.StatusBadRequest},
		{"JSON でない", `tableNo=3`, nil, http.StatusBadRequest},
		{"定義にない項目", `{"tableNo": 3, "table": 3}`, nil, http.StatusBadRequest},
		{"JSON の値が 2 つ", `{"tableNo": 3}{"tableNo": 4}`, nil, http.StatusBadRequest},
		{"利用中のテーブル", `{"tableNo": 3}`, ErrTableOccupied, http.StatusConflict},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepository{check: Check{ID: 1, TableNo: 3, Status: StatusOpen}, err: tt.repoErr}
			rec := serve(t, repo, http.MethodPost, "/api/checks", tt.body)

			if rec.Code != tt.wantStatus {
				t.Fatalf("状態コード = %d, 期待値 %d (本体: %s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantStatus == http.StatusCreated && repo.gotTableNo != 3 {
				t.Errorf("Repository に渡したテーブル番号 = %d, 期待値 3", repo.gotTableNo)
			}
		})
	}
}

func TestHandlerList(t *testing.T) {
	t.Run("状態で絞り込む", func(t *testing.T) {
		repo := &fakeRepository{checks: []Check{}}
		rec := serve(t, repo, http.MethodGet, "/api/checks?status=open", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("状態コード = %d, 期待値 %d", rec.Code, http.StatusOK)
		}
		if repo.gotStatus == nil || *repo.gotStatus != StatusOpen {
			t.Errorf("Repository に渡した状態 = %v, 期待値 open", repo.gotStatus)
		}
		if got := rec.Body.String(); got != "[]\n" {
			t.Errorf("本体 = %q, 期待値 %q", got, "[]\n")
		}
	})

	t.Run("未定義の状態", func(t *testing.T) {
		rec := serve(t, &fakeRepository{}, http.MethodGet, "/api/checks?status=cancelled", "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("状態コード = %d, 期待値 %d", rec.Code, http.StatusBadRequest)
		}
	})
}

func TestHandlerGetIncludesTotals(t *testing.T) {
	repo := &fakeRepository{check: Check{
		ID:      7,
		TableNo: 2,
		Status:  StatusOpen,
		Orders: []Order{
			{ID: 1, Items: []Item{{ID: 1, Name: "生ビール", PriceYen: 600, Quantity: 2}}},
			{ID: 2, Items: []Item{{ID: 2, Name: "枝豆", PriceYen: 380, Quantity: 1}}},
		},
	}}
	rec := serve(t, repo, http.MethodGet, "/api/checks/7", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("状態コード = %d, 期待値 %d", rec.Code, http.StatusOK)
	}
	if repo.gotID != 7 {
		t.Errorf("Repository に渡した識別子 = %d, 期待値 7", repo.gotID)
	}

	var got checkResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("本体を読み取れません: %v", err)
	}
	if got.TotalYen != 1580 {
		t.Errorf("totalYen = %d, 期待値 1580", got.TotalYen)
	}
	if got.Orders[0].TotalYen != 1200 || got.Orders[0].Items[0].SubtotalYen != 1200 {
		t.Errorf("1 回目の注文の金額 = %+v", got.Orders[0])
	}
}

func TestHandlerGetWithoutOrdersReturnsEmptyArray(t *testing.T) {
	repo := &fakeRepository{check: Check{ID: 1, TableNo: 1, Status: StatusOpen}}
	rec := serve(t, repo, http.MethodGet, "/api/checks/1", "")

	// 注文のない伝票でも orders は null ではなく [] になる。
	if !strings.Contains(rec.Body.String(), `"orders":[]`) {
		t.Errorf("本体 = %s", rec.Body)
	}
}

func TestHandlerAddOrder(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		body       string
		repoErr    error
		wantStatus int
	}{
		{"注文を追加", "/api/checks/1/orders", `{"items": [{"menuItemId": 1, "quantity": 2}]}`, nil, http.StatusCreated},
		{"品目なし", "/api/checks/1/orders", `{"items": []}`, nil, http.StatusBadRequest},
		{"数量が 0", "/api/checks/1/orders", `{"items": [{"menuItemId": 1, "quantity": 0}]}`, nil, http.StatusBadRequest},
		// 単価は受け取らない。送られてきても定義にない項目として拒否する。
		{"単価を送る", "/api/checks/1/orders", `{"items": [{"menuItemId": 1, "quantity": 1, "priceYen": 1}]}`, nil, http.StatusBadRequest},
		{"不正な識別子", "/api/checks/x/orders", `{"items": [{"menuItemId": 1, "quantity": 1}]}`, nil, http.StatusBadRequest},
		{"存在しない伝票", "/api/checks/9/orders", `{"items": [{"menuItemId": 1, "quantity": 1}]}`, ErrCheckNotFound, http.StatusNotFound},
		{"会計済みの伝票", "/api/checks/1/orders", `{"items": [{"menuItemId": 1, "quantity": 1}]}`, ErrCheckNotOpen, http.StatusConflict},
		{"品切れ", "/api/checks/1/orders", `{"items": [{"menuItemId": 5, "quantity": 1}]}`, ErrMenuItemUnavailable, http.StatusUnprocessableEntity},
		{"存在しないメニュー", "/api/checks/1/orders", `{"items": [{"menuItemId": 99, "quantity": 1}]}`, ErrMenuItemNotFound, http.StatusUnprocessableEntity},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepository{
				order: Order{ID: 1, Items: []Item{{ID: 1, MenuItemID: 1, Name: "唐揚げ", PriceYen: 680, Quantity: 2}}},
				err:   tt.repoErr,
			}
			rec := serve(t, repo, http.MethodPost, tt.target, tt.body)

			if rec.Code != tt.wantStatus {
				t.Fatalf("状態コード = %d, 期待値 %d (本体: %s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantStatus != http.StatusCreated {
				return
			}
			if len(repo.gotLines) != 1 || repo.gotLines[0] != (Line{MenuItemID: 1, Quantity: 2}) {
				t.Errorf("Repository に渡した品目 = %+v", repo.gotLines)
			}

			var got orderResponse
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("本体を読み取れません: %v", err)
			}
			if got.TotalYen != 1360 {
				t.Errorf("totalYen = %d, 期待値 1360", got.TotalYen)
			}
		})
	}
}

func TestHandlerPayAndClose(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		repoErr    error
		wantStatus int
	}{
		{"会計する", "/api/checks/1/pay", nil, http.StatusOK},
		{"二重に会計する", "/api/checks/1/pay", ErrCheckNotOpen, http.StatusConflict},
		{"存在しない伝票を会計する", "/api/checks/9/pay", ErrCheckNotFound, http.StatusNotFound},
		{"伝票を閉じる", "/api/checks/1/close", nil, http.StatusOK},
		{"会計前に閉じる", "/api/checks/1/close", ErrCheckNotPaid, http.StatusConflict},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepository{check: Check{ID: 1, Status: StatusPaid}, err: tt.repoErr}
			rec := serve(t, repo, http.MethodPost, tt.target, "")

			if rec.Code != tt.wantStatus {
				t.Fatalf("状態コード = %d, 期待値 %d (本体: %s)", rec.Code, tt.wantStatus, rec.Body)
			}
		})
	}
}

func TestHandlerHidesInternalErrors(t *testing.T) {
	repo := &fakeRepository{err: errors.New("connection refused: 10.0.0.5:5432")}
	rec := serve(t, repo, http.MethodPost, "/api/checks/1/pay", "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("状態コード = %d, 期待値 %d", rec.Code, http.StatusInternalServerError)
	}
	var body httpx.ErrorBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("エラー応答を読み取れません: %v", err)
	}
	// 接続先などの内部の情報を応答へ漏らしていないか。
	if body.Error != "会計できませんでした" {
		t.Errorf("エラー = %q", body.Error)
	}
}
