package menu

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Repository は品目の永続化を担う。
//
// 構造体ではなくインターフェースとして定義することで、利用する側は
// 「どう保存されるか」を知らずに済む。テストでは差し替えられる。
type Repository interface {
	List(ctx context.Context, f ListFilter) ([]Item, error)
	Get(ctx context.Context, id int64) (Item, error)
}

// ListFilter は一覧を絞り込む条件。
//
// ポインタ型にしているのは、指定の有無を表現するため。
// Category が nil なら分類を問わず、値が入っていればその分類だけを返す。
type ListFilter struct {
	Category      *Category
	AvailableOnly bool
}

// PostgresRepository は PostgreSQL に品目を保存する Repository の実装。
type PostgresRepository struct {
	db *sql.DB
}

// NewPostgresRepository は PostgresRepository を作る。
//
// 戻り値をインターフェース型にせず具体的な型で返すのは Go の慣習による。
// インターフェースは受け取る側が定義するもので、返す側は具体的な型を返す。
func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// 列の並びは Scan の順序と対応するため、問い合わせ間で共通の定数にまとめる。
const itemColumns = `id, name, category, price_yen, available, created_at, updated_at`

// List は条件に合う品目を返す。
func (r *PostgresRepository) List(ctx context.Context, f ListFilter) ([]Item, error) {
	// 条件の有無で WHERE 句が変わるため、プレースホルダの番号を数えながら組み立てる。
	// 値を文字列結合で埋め込むと SQL インジェクションを招くため、必ず引数として渡す。
	query := `SELECT ` + itemColumns + ` FROM menu_items`
	var (
		conditions []string
		args       []any
	)

	if f.Category != nil {
		args = append(args, string(*f.Category))
		conditions = append(conditions, fmt.Sprintf("category = $%d", len(args)))
	}
	if f.AvailableOnly {
		conditions = append(conditions, "available = true")
	}

	for i, cond := range conditions {
		if i == 0 {
			query += " WHERE " + cond
			continue
		}
		query += " AND " + cond
	}

	// 並び順を指定しない SELECT の結果は保証されないため、常に明示する。
	query += " ORDER BY category, id"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("menu: 品目の一覧取得に失敗しました: %w", err)
	}
	// rows は接続を保持し続けるため、必ず閉じる。
	defer rows.Close()

	// nil のスライスは JSON で null になってしまう。
	// 0 件のときに [] を返したいので、空のスライスで初期化しておく。
	items := []Item{}
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	// 繰り返しの途中で起きた通信の異常は rows.Next() では分からず、
	// ここで初めて報告される。確認を怠ると欠けた結果を正常として扱ってしまう。
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("menu: 品目の読み取りに失敗しました: %w", err)
	}

	return items, nil
}

// Get は 1 件の品目を返す。存在しない場合は ErrNotFound を返す。
func (r *PostgresRepository) Get(ctx context.Context, id int64) (Item, error) {
	query := `SELECT ` + itemColumns + ` FROM menu_items WHERE id = $1`

	item, err := scanItem(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		// 0 件であることは異常ではなく、呼び出し側が判断すべき結果である。
		// sql.ErrNoRows のままでは database/sql の都合が外へ漏れるため、
		// このパッケージの言葉である ErrNotFound に置き換える。
		if errors.Is(err, sql.ErrNoRows) {
			return Item{}, ErrNotFound
		}
		return Item{}, fmt.Errorf("menu: 品目の取得に失敗しました: %w", err)
	}

	return item, nil
}

// scanner は *sql.Row と *sql.Rows のどちらも受け取るための最小の約束。
//
// 両者は Scan を持つ点で共通するが、共通の型を持たない。
// 必要な操作だけを並べた小さなインターフェースを置くことで、
// 1 件取得と一覧取得で読み取り処理を共有できる。
type scanner interface {
	Scan(dest ...any) error
}

// scanItem は 1 行を Item へ読み取る。
//
// Scan へ渡す順序は SELECT の列の並びと一致していなければならない。
// 食い違うと型の不一致か、値の取り違えを起こす。
func scanItem(s scanner) (Item, error) {
	var item Item
	err := s.Scan(
		&item.ID,
		&item.Name,
		&item.Category,
		&item.PriceYen,
		&item.Available,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return Item{}, err
	}
	return item, nil
}
