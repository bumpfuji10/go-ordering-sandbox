package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// maxBodyBytes は受け付ける要求本体の上限。
//
// 上限がないと、巨大な本体を送りつけられたときに読み切るまでメモリを使い続ける。
// 注文 1 回分の JSON はこれに遠く及ばない。
const maxBodyBytes = 1 << 20 // 1 MiB

// DecodeJSON は要求本体を JSON として T へ読み取る。
//
// 読み取り先を引数で受け取る形 (dst any) にすると、ポインタを渡し忘れても
// コンパイルが通ってしまう。型引数で受け取り、値として返すことでそれを防ぐ。
//
// 返すエラーの文面は、そのまま利用者に見せることを想定している。
func DecodeJSON[T any](w http.ResponseWriter, r *http.Request) (T, error) {
	var dst, zero T
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	dec := json.NewDecoder(r.Body)
	// 定義にない項目を黙って無視すると、項目名の綴り違いに気づけない。
	dec.DisallowUnknownFields()

	if err := dec.Decode(&dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return zero, errors.New("要求の本体が大きすぎます")
		}
		return zero, errors.New("要求の本体を JSON として読み取れません")
	}

	// Decode は最初の値を読んだ時点で戻るため、後ろに続くデータは確かめない。
	// {"a":1}{"b":2} のような本体を受け入れないよう、末尾であることを確かめる。
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return zero, errors.New("要求の本体には JSON の値を 1 つだけ含めてください")
	}
	return dst, nil
}
