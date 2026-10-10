// 注文システムの API サーバー。
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"example.com/ordering/internal/db"
)

func main() {
	// run に処理を預け、main では終了コードだけを扱う。
	// main の中で log.Fatal を呼ぶと defer が実行されないまま終了するため、
	// 後片付けの必要な処理は通常の関数に閉じ込める。
	if err := run(); err != nil {
		log.Printf("起動に失敗しました: %v", err)
		os.Exit(1)
	}
}

func run() error {
	// Ctrl-C や docker stop を受け取ったら ctx が終了する。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := db.LoadConfig()
	log.Printf("データベースへ接続します: %s:%s/%s", cfg.Host, cfg.Port, cfg.Database)

	database, err := db.Open(ctx, cfg)
	if err != nil {
		return err
	}
	defer database.Close()

	log.Println("データベースへの接続を確認しました")

	// 疎通の確認として、メニューの件数を数えてみる。
	var count int
	row := database.QueryRowContext(ctx, "SELECT count(*) FROM menu_items")
	if err := row.Scan(&count); err != nil {
		return err
	}
	log.Printf("登録されているメニューは %d 件です", count)

	return nil
}
