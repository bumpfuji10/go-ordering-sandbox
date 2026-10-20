package menu

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"example.com/ordering/internal/httpx"
)

// Handler は品目に対する HTTP の要求を受け付ける。
type Handler struct {
	repo Repository
}

// NewHandler は Handler を作る。
//
// 引数をインターフェースにしているため、試験では偽の実装を渡せる。
// Handler は品目がどこに保存されているかを知らない。
func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

// Register は経路をルータへ登録する。
//
// Go 1.22 以降の ServeMux は "メソッド 経路" の形を解釈し、
// {id} のような区切りを r.PathValue で取り出せる。
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/menu-items", h.list)
	mux.HandleFunc("GET /api/menu-items/{id}", h.get)
}

// list は品目の一覧を返す。
//
//	GET /api/menu-items
//	GET /api/menu-items?category=food&availableOnly=true
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	filter, err := parseListFilter(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	items, err := h.repo.List(r.Context(), filter)
	if err != nil {
		// 失敗の詳細は記録に残し、応答には含めない。
		// 問い合わせ文や接続先が外へ漏れると、攻撃の手がかりを与えてしまう。
		log.Printf("menu: 一覧の取得に失敗しました: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "品目の取得に失敗しました")
		return
	}

	httpx.JSON(w, http.StatusOK, items)
}

// get は 1 件の品目を返す。
//
//	GET /api/menu-items/{id}
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.Error(w, http.StatusBadRequest, "品目の識別子が不正です")
		return
	}

	item, err := h.repo.Get(r.Context(), id)
	if err != nil {
		// 該当がないのは異常ではなく、要求に対する正当な結果である。
		// repository が database/sql のエラーを ErrNotFound へ翻訳しているため、
		// この層は保存方法を知らないまま 404 を判断できる。
		if errors.Is(err, ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "品目が見つかりません")
			return
		}
		log.Printf("menu: 品目の取得に失敗しました (id=%d): %v", id, err)
		httpx.Error(w, http.StatusInternalServerError, "品目の取得に失敗しました")
		return
	}

	httpx.JSON(w, http.StatusOK, item)
}

// parseListFilter は問い合わせ文字列を絞り込み条件へ変換する。
func parseListFilter(r *http.Request) (ListFilter, error) {
	var filter ListFilter

	if raw := r.URL.Query().Get("category"); raw != "" {
		category := Category(raw)
		if !category.Valid() {
			return ListFilter{}, errors.New("分類は food か drink を指定してください")
		}
		filter.Category = &category
	}

	if raw := r.URL.Query().Get("availableOnly"); raw != "" {
		availableOnly, err := strconv.ParseBool(raw)
		if err != nil {
			return ListFilter{}, errors.New("availableOnly は true か false を指定してください")
		}
		filter.AvailableOnly = availableOnly
	}

	return filter, nil
}
