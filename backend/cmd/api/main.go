// 注文システムの API サーバー。
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/ordering/internal/db"
	"example.com/ordering/internal/env"
	"example.com/ordering/internal/server"
)

func main() {
	// run に処理を預け、main では終了コードだけを扱う。
	// main の中で log.Fatal を呼ぶと defer が実行されないまま終了するため、
	// 後片付けの必要な処理は通常の関数に閉じ込める。
	if err := run(); err != nil {
		log.Printf("異常が発生しました: %v", err)
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

	srv := &http.Server{
		Handler: server.NewRouter(database),
		// 時間の上限を定めないと、応答しない相手との接続が残り続け、
		// やがて新しい要求を受け付けられなくなる。
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// ListenAndServe はポートの確保と受け付けをまとめて行うため、
	// 確保に失敗したかどうかを受け付けの開始前に知る手段がない。
	// 先に Listen でポートを押さえ、成功してから開始を記録する。
	addr := ":" + env.Or("PORT", "8080")
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("ポート %s を確保できませんでした: %w", addr, err)
	}
	log.Printf("要求の受け付けを開始します: http://localhost%s", addr)

	// Serve は停止するまで戻らないため、別の流れで動かし、
	// 本体は終了の合図を待つ。
	errCh := make(chan error, 1)
	go func() {
		// 正常な停止でも ErrServerClosed が返るため、異常とは区別する。
		// ln の後始末は Serve が引き受け、戻る時点で閉じられている。
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Println("停止の合図を受け取りました")
	}

	// 処理中の要求を中断せず、終わるのを待ってから閉じる。
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}

	log.Println("停止しました")
	return nil
}
