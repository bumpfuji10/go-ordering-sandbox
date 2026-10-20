// Package httpx は HTTP の応答に関する共通の処理を提供する。
package httpx

import (
	"encoding/json"
	"log"
	"net/http"
)

// ErrorBody はエラー応答の本体。
//
// 正常時と異なる形を返すと、受け取る側が分岐を書く必要が出る。
// エラーの形を 1 つに定めておくことで、画面側の扱いを揃えられる。
type ErrorBody struct {
	Error string `json:"error"`
}

// JSON は指定した状態コードで値を JSON として書き出す。
func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if body == nil {
		return
	}

	// ここで失敗しても状態コードは送信済みであり、応答を訂正できない。
	// 書き出しの途中で接続が切れた場合などが該当するため、記録に留める。
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("httpx: 応答の書き出しに失敗しました: %v", err)
	}
}

// Error は指定した状態コードでエラーを返す。
func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, ErrorBody{Error: message})
}
