// Package order は伝票・注文・明細を扱う。
//
// 注文は食事中に何度も発生するが、会計は退店時に一度だけ行われる。
// 単位の異なる 2 つを結びつけるため、複数の注文をまとめる入れ物として伝票を置く。
//
//	伝票 (Check) 1 ─ N 注文 (Order) 1 ─ N 明細 (Item)
package order

import (
	"errors"
	"time"
)

// Status は伝票の状態。
//
// 入店から退店まで次の順に進み、逆戻りはしない。
//
//	open (利用中) -> paid (会計済) -> closed (退店済)
type Status string

const (
	StatusOpen   Status = "open"
	StatusPaid   Status = "paid"
	StatusClosed Status = "closed"
)

// Valid は定義済みの状態であるかを返す。
func (s Status) Valid() bool {
	switch s {
	case StatusOpen, StatusPaid, StatusClosed:
		return true
	default:
		return false
	}
}

// 呼び出し側は errors.Is でこれらと照合し、HTTP の状態コードへ変換する。
var (
	ErrCheckNotFound = errors.New("order: 伝票が見つかりません")
	// 1 つのテーブルに退店していない伝票は同時に 1 枚だけ。会計を済ませても、
	// 退店するまでは席にいるため、paid の伝票も数える。
	ErrTableOccupied = errors.New("order: このテーブルには退店していない伝票があります")
	// 注文の追加と会計は、利用中の伝票にしか行えない。
	ErrCheckNotOpen = errors.New("order: 伝票が利用中ではありません")
	// 伝票を閉じられるのは、会計が済んだ伝票だけ。
	ErrCheckNotPaid        = errors.New("order: 伝票が会計済みではありません")
	ErrMenuItemNotFound    = errors.New("order: 存在しないメニューが含まれています")
	ErrMenuItemUnavailable = errors.New("order: 品切れのメニューが含まれています")
)

// Check は 1 組の来店につき 1 枚の伝票。
type Check struct {
	ID       int64
	TableNo  int
	Status   Status
	OpenedAt time.Time
	// 会計前・退店前は時刻が存在しないため nil になる。
	PaidAt   *time.Time
	ClosedAt *time.Time
	Orders   []Order
}

// TotalYen は伝票の合計金額を返す。
//
// 合計は保存せず、その都度明細から計算する。保存すると明細との
// 食い違いが起こりうるうえ、どちらが正しいかを判断できなくなる。
func (c Check) TotalYen() int {
	total := 0
	for _, o := range c.Orders {
		total += o.TotalYen()
	}
	return total
}

// Order は厨房に伝えた 1 回分の注文。
type Order struct {
	ID        int64
	OrderedAt time.Time
	Items     []Item
}

// TotalYen は注文 1 回分の金額を返す。
func (o Order) TotalYen() int {
	total := 0
	for _, it := range o.Items {
		total += it.SubtotalYen()
	}
	return total
}

// Item は注文の明細（1 品目ぶん）。
//
// 品名と単価は注文した時点のメニューから写し取った値であり、
// その後にメニューの価格が変わってもこの値は変わらない。
type Item struct {
	ID         int64
	MenuItemID int64
	Name       string
	PriceYen   int
	Quantity   int
}

// SubtotalYen は明細の小計（単価 × 数量）を返す。
func (i Item) SubtotalYen() int {
	return i.PriceYen * i.Quantity
}

// Line は注文で頼まれた 1 品目。どのメニューをいくつ頼んだかだけを持つ。
//
// 品名と単価は受け取らない。画面から送られた価格を信じると、
// 書き換えられた金額で注文できてしまうため、保存時にメニューから引く。
type Line struct {
	MenuItemID int64
	Quantity   int
}

// ValidateTableNo はテーブル番号として妥当かを確かめる。
func ValidateTableNo(tableNo int) error {
	if tableNo <= 0 {
		return errors.New("テーブル番号は 1 以上を指定してください")
	}
	return nil
}

// ValidateLines は注文の中身として妥当かを確かめる。
//
// 返すエラーの文面は、そのまま利用者に見せることを想定している。
func ValidateLines(lines []Line) error {
	if len(lines) == 0 {
		return errors.New("注文する品目を 1 つ以上指定してください")
	}
	for _, l := range lines {
		if l.MenuItemID <= 0 {
			return errors.New("メニューの識別子が不正です")
		}
		if l.Quantity <= 0 {
			return errors.New("数量は 1 以上を指定してください")
		}
	}
	return nil
}
