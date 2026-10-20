// Package db は PostgreSQL への接続を扱う。
package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"example.com/ordering/internal/env"

	// pgx のドライバを database/sql に登録するためだけに読み込む。
	// このパッケージの関数を直接呼ぶことはないため、識別子を _ にして
	// 「副作用のための import」であることを示す。
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Config は接続先の情報。
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	SSLMode  string
}

// LoadConfig は環境変数から接続情報を読む。
// 未設定の項目には compose.yml に合わせた既定値を使う。
func LoadConfig() Config {
	return Config{
		Host:     env.Or("DB_HOST", "localhost"),
		Port:     env.Or("DB_PORT", "5432"),
		User:     env.Or("DB_USER", "app"),
		Password: env.Or("DB_PASSWORD", "password"),
		Database: env.Or("DB_NAME", "ordering"),
		SSLMode:  env.Or("DB_SSLMODE", "disable"),
	}
}

// DSN は接続文字列を組み立てる。
func (c Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode,
	)
}

// Open は接続プールを用意し、疎通するところまで確かめる。
//
// sql.Open は名前に反して接続を確立せず、設定を組み立てるだけで返る。
// 起動時に接続先の異常へ気づけるよう、ここで Ping まで済ませる。
func Open(ctx context.Context, cfg Config) (*sql.DB, error) {
	database, err := sql.Open("pgx", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("接続設定の組み立てに失敗しました: %w", err)
	}

	// *sql.DB は接続 1 本ではなく接続プールであり、複数のリクエストで
	// 使い回される。上限を定めないと負荷時に接続を作りすぎてしまう。
	database.SetMaxOpenConns(25)
	database.SetMaxIdleConns(25)
	database.SetConnMaxLifetime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := database.PingContext(pingCtx); err != nil {
		// 疎通しない場合、この接続プールはもう使わないため閉じておく。
		database.Close()
		return nil, fmt.Errorf("データベースに接続できませんでした: %w", err)
	}

	return database, nil
}
