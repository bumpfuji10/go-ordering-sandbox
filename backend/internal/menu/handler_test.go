package menu

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"example.com/ordering/internal/httpx"
)

// fakeRepository は試験用の Repository。
//
// 受け取った引数を記録し、あらかじめ決めた結果を返す。
// データベースなしで Handler の振る舞いだけを確かめられる。
type fakeRepository struct {
	items []Item
	item  Item
	err   error

	gotFilter ListFilter
	gotID     int64
}

func (f *fakeRepository) List(_ context.Context, filter ListFilter) ([]Item, error) {
	f.gotFilter = filter
	return f.items, f.err
}

func (f *fakeRepository) Get(_ context.Context, id int64) (Item, error) {
	f.gotID = id
	return f.item, f.err
}

// serve は Handler を登録したルータに要求を 1 つ送り、応答を返す。
//
// Handler のメソッドを直接呼ばずルータを通すことで、
// 経路の登録と {id} の取り出しまで含めて確かめる。
func serve(t *testing.T, repo Repository, target string) *httptest.ResponseRecorder {
	t.Helper()

	mux := http.NewServeMux()
	NewHandler(repo).Register(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestHandlerList(t *testing.T) {
	food := CategoryFood

	tests := []struct {
		name       string
		target     string
		wantStatus int
		wantFilter ListFilter
	}{
		{
			name:       "条件なし",
			target:     "/api/menu-items",
			wantStatus: http.StatusOK,
			wantFilter: ListFilter{},
		},
		{
			name:       "分類と提供中のみを指定",
			target:     "/api/menu-items?category=food&availableOnly=true",
			wantStatus: http.StatusOK,
			wantFilter: ListFilter{Category: &food, AvailableOnly: true},
		},
		{
			name:       "未定義の分類",
			target:     "/api/menu-items?category=dessert",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "真偽値でない availableOnly",
			target:     "/api/menu-items?availableOnly=yes",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepository{items: []Item{}}
			rec := serve(t, repo, tt.target)

			if rec.Code != tt.wantStatus {
				t.Fatalf("状態コード = %d, 期待値 %d (本体: %s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantStatus != http.StatusOK {
				return
			}
			if !equalFilter(repo.gotFilter, tt.wantFilter) {
				t.Errorf("絞り込み条件 = %+v, 期待値 %+v", repo.gotFilter, tt.wantFilter)
			}
		})
	}
}

func TestHandlerListReturnsEmptyArray(t *testing.T) {
	rec := serve(t, &fakeRepository{items: []Item{}}, "/api/menu-items")

	// 0 件のとき null ではなく [] を返す約束を守れているか。
	if got := rec.Body.String(); got != "[]\n" {
		t.Errorf("本体 = %q, 期待値 %q", got, "[]\n")
	}
}

func TestHandlerListRepositoryError(t *testing.T) {
	rec := serve(t, &fakeRepository{err: errors.New("接続が切れました")}, "/api/menu-items")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("状態コード = %d, 期待値 %d", rec.Code, http.StatusInternalServerError)
	}
	// 内部のエラー文を応答へ漏らしていないか。
	if body := decodeError(t, rec); body.Error != "品目の取得に失敗しました" {
		t.Errorf("エラー = %q", body.Error)
	}
}

func TestHandlerGet(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		repo       *fakeRepository
		wantStatus int
	}{
		{
			name:       "存在する品目",
			target:     "/api/menu-items/1",
			repo:       &fakeRepository{item: Item{ID: 1, Name: "枝豆", Category: CategoryFood, PriceYen: 380}},
			wantStatus: http.StatusOK,
		},
		{
			name:       "存在しない品目",
			target:     "/api/menu-items/999",
			repo:       &fakeRepository{err: ErrNotFound},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "数値でない識別子",
			target:     "/api/menu-items/abc",
			repo:       &fakeRepository{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "0 以下の識別子",
			target:     "/api/menu-items/0",
			repo:       &fakeRepository{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "保存先の異常",
			target:     "/api/menu-items/1",
			repo:       &fakeRepository{err: errors.New("接続が切れました")},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(t, tt.repo, tt.target)

			if rec.Code != tt.wantStatus {
				t.Fatalf("状態コード = %d, 期待値 %d (本体: %s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantStatus != http.StatusOK {
				return
			}

			var got Item
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("本体を読み取れません: %v", err)
			}
			if got.ID != tt.repo.item.ID || got.Name != tt.repo.item.Name {
				t.Errorf("品目 = %+v, 期待値 %+v", got, tt.repo.item)
			}
			if tt.repo.gotID != tt.repo.item.ID {
				t.Errorf("Repository に渡した識別子 = %d, 期待値 %d", tt.repo.gotID, tt.repo.item.ID)
			}
		})
	}
}

// equalFilter は 2 つの絞り込み条件が同じ内容かを返す。
//
// Category はポインタのため == ではアドレスを比べてしまう。指す先の値で比べる。
func equalFilter(a, b ListFilter) bool {
	if a.AvailableOnly != b.AvailableOnly {
		return false
	}
	if a.Category == nil || b.Category == nil {
		return a.Category == b.Category
	}
	return *a.Category == *b.Category
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) httpx.ErrorBody {
	t.Helper()

	var body httpx.ErrorBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("エラー応答を読み取れません: %v", err)
	}
	return body
}
