// Package server は HTTP の経路を組み立てる。
package server

import (
	"database/sql"
	"net/http"

	"example.com/ordering/internal/httpx"
	"example.com/ordering/internal/menu"
	"example.com/ordering/internal/order"
)

// NewRouter は依存を組み立て、経路を登録したルータを返す。
//
// どの実装を使うかの判断をこの一箇所に集めることで、
// 各ハンドラは自分が何に依存しているかだけを知っていればよくなる。
func NewRouter(db *sql.DB) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", health)

	menuHandler := menu.NewHandler(menu.NewPostgresRepository(db))
	menuHandler.Register(mux)

	orderHandler := order.NewHandler(order.NewPostgresRepository(db))
	orderHandler.Register(mux)

	return mux
}

// health は死活監視に応える。
func health(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
