package order

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"example.com/ordering/internal/httpx"
)

// Handler は伝票と注文に対する HTTP の要求を受け付ける。
type Handler struct {
	repo Repository
}

// NewHandler は Handler を作る。
func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

// Register は経路をルータへ登録する。
//
// 会計と退店は伝票の状態を進める操作であり、伝票の一部を書き換える
// PATCH ではなく、操作ごとの経路を POST で受ける。状態を自由に書き換え
// られる形にすると、paid から open へ戻すといった逆戻りを許してしまう。
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/checks", h.open)
	mux.HandleFunc("GET /api/checks", h.list)
	mux.HandleFunc("GET /api/checks/{id}", h.get)
	mux.HandleFunc("POST /api/checks/{id}/orders", h.addOrder)
	mux.HandleFunc("POST /api/checks/{id}/pay", h.pay)
	mux.HandleFunc("POST /api/checks/{id}/close", h.close)
}

// open は伝票を開く。
//
//	POST /api/checks  {"tableNo": 3}
func (h *Handler) open(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TableNo int `json:"tableNo"`
	}
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := ValidateTableNo(req.TableNo); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	c, err := h.repo.Open(r.Context(), req.TableNo)
	if err != nil {
		writeError(w, err, "伝票を開けませんでした")
		return
	}
	httpx.JSON(w, http.StatusCreated, newCheckResponse(c))
}

// list は伝票の一覧を返す。
//
//	GET /api/checks
//	GET /api/checks?status=open
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	var status *Status
	if raw := r.URL.Query().Get("status"); raw != "" {
		s := Status(raw)
		if !s.Valid() {
			httpx.Error(w, http.StatusBadRequest, "状態は open、paid、closed のいずれかを指定してください")
			return
		}
		status = &s
	}

	checks, err := h.repo.List(r.Context(), status)
	if err != nil {
		writeError(w, err, "伝票の取得に失敗しました")
		return
	}

	res := make([]checkResponse, len(checks))
	for i, c := range checks {
		res[i] = newCheckResponse(c)
	}
	httpx.JSON(w, http.StatusOK, res)
}

// get は 1 枚の伝票を返す。
//
//	GET /api/checks/{id}
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	c, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeError(w, err, "伝票の取得に失敗しました")
		return
	}
	httpx.JSON(w, http.StatusOK, newCheckResponse(c))
}

// addOrder は伝票に注文を追加する。
//
//	POST /api/checks/{id}/orders  {"items": [{"menuItemId": 1, "quantity": 2}]}
func (h *Handler) addOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	var req struct {
		Items []struct {
			MenuItemID int64 `json:"menuItemId"`
			Quantity   int   `json:"quantity"`
		} `json:"items"`
	}
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	lines := make([]Line, len(req.Items))
	for i, it := range req.Items {
		lines[i] = Line{MenuItemID: it.MenuItemID, Quantity: it.Quantity}
	}
	if err := ValidateLines(lines); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	o, err := h.repo.AddOrder(r.Context(), id, lines)
	if err != nil {
		writeError(w, err, "注文を追加できませんでした")
		return
	}
	httpx.JSON(w, http.StatusCreated, newOrderResponse(o))
}

// pay は伝票を会計済みにし、合計金額を含む伝票を返す。
//
//	POST /api/checks/{id}/pay
func (h *Handler) pay(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	c, err := h.repo.Pay(r.Context(), id)
	if err != nil {
		writeError(w, err, "会計できませんでした")
		return
	}
	httpx.JSON(w, http.StatusOK, newCheckResponse(c))
}

// close は会計済みの伝票を閉じる。
//
//	POST /api/checks/{id}/close
func (h *Handler) close(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	c, err := h.repo.Close(r.Context(), id)
	if err != nil {
		writeError(w, err, "伝票を閉じられませんでした")
		return
	}
	httpx.JSON(w, http.StatusOK, newCheckResponse(c))
}

// parseID は経路の {id} を読み取る。不正であれば 400 を書き出して false を返す。
func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.Error(w, http.StatusBadRequest, "伝票の識別子が不正です")
		return 0, false
	}
	return id, true
}

// writeError はドメインのエラーを状態コードへ変換して書き出す。
//
// 想定外のエラーは記録に残し、応答には failMessage だけを返す。
func writeError(w http.ResponseWriter, err error, failMessage string) {
	switch {
	case errors.Is(err, ErrCheckNotFound):
		httpx.Error(w, http.StatusNotFound, "伝票が見つかりません")
	// 要求の形は正しいが、伝票の今の状態では受け付けられない。
	case errors.Is(err, ErrTableOccupied):
		httpx.Error(w, http.StatusConflict, "このテーブルには利用中の伝票があります")
	case errors.Is(err, ErrCheckNotOpen):
		httpx.Error(w, http.StatusConflict, "伝票が利用中ではありません")
	case errors.Is(err, ErrCheckNotPaid):
		httpx.Error(w, http.StatusConflict, "伝票が会計済みではありません")
	// JSON としては正しいが、指している品目が注文できない。
	case errors.Is(err, ErrMenuItemNotFound):
		httpx.Error(w, http.StatusUnprocessableEntity, "存在しないメニューが含まれています")
	case errors.Is(err, ErrMenuItemUnavailable):
		httpx.Error(w, http.StatusUnprocessableEntity, "品切れのメニューが含まれています")
	default:
		log.Printf("order: %s: %v", failMessage, err)
		httpx.Error(w, http.StatusInternalServerError, failMessage)
	}
}

// 以下は応答の形。
//
// 合計や小計は保存されている値ではなく、ドメインが明細から計算した値である。
// ドメインの型に JSON の都合を持ち込まず、応答を組み立てる時点で計算結果を詰める。

type checkResponse struct {
	ID       int64           `json:"id"`
	TableNo  int             `json:"tableNo"`
	Status   Status          `json:"status"`
	OpenedAt time.Time       `json:"openedAt"`
	PaidAt   *time.Time      `json:"paidAt"`
	ClosedAt *time.Time      `json:"closedAt"`
	Orders   []orderResponse `json:"orders"`
	TotalYen int             `json:"totalYen"`
}

type orderResponse struct {
	ID        int64          `json:"id"`
	OrderedAt time.Time      `json:"orderedAt"`
	Items     []itemResponse `json:"items"`
	TotalYen  int            `json:"totalYen"`
}

type itemResponse struct {
	ID          int64  `json:"id"`
	MenuItemID  int64  `json:"menuItemId"`
	Name        string `json:"name"`
	PriceYen    int    `json:"priceYen"`
	Quantity    int    `json:"quantity"`
	SubtotalYen int    `json:"subtotalYen"`
}

func newCheckResponse(c Check) checkResponse {
	// make で長さを決めておくと、注文が 0 件でも null ではなく [] になる。
	orders := make([]orderResponse, len(c.Orders))
	for i, o := range c.Orders {
		orders[i] = newOrderResponse(o)
	}
	return checkResponse{
		ID:       c.ID,
		TableNo:  c.TableNo,
		Status:   c.Status,
		OpenedAt: c.OpenedAt,
		PaidAt:   c.PaidAt,
		ClosedAt: c.ClosedAt,
		Orders:   orders,
		TotalYen: c.TotalYen(),
	}
}

func newOrderResponse(o Order) orderResponse {
	items := make([]itemResponse, len(o.Items))
	for i, it := range o.Items {
		items[i] = itemResponse{
			ID:          it.ID,
			MenuItemID:  it.MenuItemID,
			Name:        it.Name,
			PriceYen:    it.PriceYen,
			Quantity:    it.Quantity,
			SubtotalYen: it.SubtotalYen(),
		}
	}
	return orderResponse{
		ID:        o.ID,
		OrderedAt: o.OrderedAt,
		Items:     items,
		TotalYen:  o.TotalYen(),
	}
}
