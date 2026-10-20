// Package menu は飲食店が提供する品目を扱う。
package menu

import (
	"errors"
	"time"
)

// Category は品目の分類。
//
// string をそのまま使わず独自の型にすることで、分類を受け取るはずの場所へ
// 誤って品名を渡すといった取り違えをコンパイル時に防ぐ。
type Category string

const (
	CategoryFood  Category = "food"
	CategoryDrink Category = "drink"
)

// Valid は定義済みの分類であるかを返す。
func (c Category) Valid() bool {
	switch c {
	case CategoryFood, CategoryDrink:
		return true
	default:
		return false
	}
}

// ErrNotFound は該当する品目が存在しないことを表す。
//
// 呼び出し側は errors.Is でこの値と照合し、HTTP の 404 へ変換する。
var ErrNotFound = errors.New("menu: 品目が見つかりません")

// Item は提供する 1 品目。
//
// 金額は円単位の整数で持つ。浮動小数点数は 0.1 を正確に表現できず、
// 加算を重ねるうちに誤差が生じるため金額には用いない。
// カラム名と同じく単位をフィールド名に含め、読んだだけで単位が分かるようにしている。
type Item struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Category  Category  `json:"category"`
	PriceYen  int       `json:"priceYen"`
	Available bool      `json:"available"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
