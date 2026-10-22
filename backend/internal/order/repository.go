package order

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// Repository は伝票と注文の永続化を担う。
//
// 状態の確認と書き込みは同じトランザクションの中で行う必要があるため、
// 「利用中の伝票にしか注文できない」といった規則の確認もこの層で行う。
// 確認と書き込みの間に別の要求が割り込むと、規則が破られてしまう。
type Repository interface {
	Open(ctx context.Context, tableNo int) (Check, error)
	List(ctx context.Context, status *Status) ([]Check, error)
	Get(ctx context.Context, id int64) (Check, error)
	AddOrder(ctx context.Context, checkID int64, lines []Line) (Order, error)
	Pay(ctx context.Context, id int64) (Check, error)
	Close(ctx context.Context, id int64) (Check, error)
}

// PostgresRepository は PostgreSQL に伝票と注文を保存する Repository の実装。
type PostgresRepository struct {
	db *sql.DB
}

// NewPostgresRepository は PostgresRepository を作る。
func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// 列の並びは scanCheck の順序と対応する。
const checkColumns = `id, table_no, status, opened_at, paid_at, closed_at`

// checks_open_table_no_idx は「1 テーブルに利用中の伝票は 1 枚だけ」を守る部分一意索引。
const openTableNoIndex = "checks_open_table_no_idx"

// Open はテーブルに新しい伝票を開く。
func (r *PostgresRepository) Open(ctx context.Context, tableNo int) (Check, error) {
	query := `INSERT INTO checks (table_no) VALUES ($1) RETURNING ` + checkColumns

	c, err := scanCheck(r.db.QueryRowContext(ctx, query, tableNo))
	if err != nil {
		// 「利用中の伝票があるか」を先に SELECT で確かめる方法では、
		// 同時に 2 つの要求が来ると両方とも「ない」と判断して通ってしまう。
		// 一意索引に判定を任せ、違反を ErrTableOccupied に翻訳する。
		if isUniqueViolation(err, openTableNoIndex) {
			return Check{}, ErrTableOccupied
		}
		return Check{}, fmt.Errorf("order: 伝票を開けませんでした: %w", err)
	}

	c.Orders = []Order{}
	return c, nil
}

// List は伝票の一覧を返す。status が nil なら状態を問わない。
func (r *PostgresRepository) List(ctx context.Context, status *Status) ([]Check, error) {
	query := `SELECT ` + checkColumns + ` FROM checks`
	var args []any
	if status != nil {
		args = append(args, string(*status))
		query += ` WHERE status = $1`
	}
	query += ` ORDER BY opened_at, id`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("order: 伝票の一覧取得に失敗しました: %w", err)
	}
	defer rows.Close()

	checks := []Check{}
	for rows.Next() {
		c, err := scanCheck(rows)
		if err != nil {
			return nil, fmt.Errorf("order: 伝票の読み取りに失敗しました: %w", err)
		}
		checks = append(checks, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("order: 伝票の読み取りに失敗しました: %w", err)
	}

	if err := r.attachOrders(ctx, checks); err != nil {
		return nil, err
	}
	return checks, nil
}

// Get は 1 枚の伝票を、注文と明細を含めて返す。
func (r *PostgresRepository) Get(ctx context.Context, id int64) (Check, error) {
	query := `SELECT ` + checkColumns + ` FROM checks WHERE id = $1`

	c, err := scanCheck(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Check{}, ErrCheckNotFound
		}
		return Check{}, fmt.Errorf("order: 伝票の取得に失敗しました: %w", err)
	}

	checks := []Check{c}
	if err := r.attachOrders(ctx, checks); err != nil {
		return Check{}, err
	}
	return checks[0], nil
}

// AddOrder は利用中の伝票に注文を 1 回分追加する。
func (r *PostgresRepository) AddOrder(ctx context.Context, checkID int64, lines []Line) (Order, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Order{}, fmt.Errorf("order: トランザクションを開始できませんでした: %w", err)
	}
	// Commit の後に呼んでも何もしないため、失敗したときの取り消しを defer で一括して引き受ける。
	defer tx.Rollback()

	// FOR UPDATE で伝票の行を押さえる。会計の UPDATE はこの行ロックの解放を
	// 待つため、「状態を確かめた直後に会計され、会計済みの伝票へ注文が入る」
	// という割り込みが起きない。
	var status Status
	err = tx.QueryRowContext(ctx, `SELECT status FROM checks WHERE id = $1 FOR UPDATE`, checkID).Scan(&status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Order{}, ErrCheckNotFound
		}
		return Order{}, fmt.Errorf("order: 伝票の取得に失敗しました: %w", err)
	}
	if status != StatusOpen {
		return Order{}, ErrCheckNotOpen
	}

	menu, err := lookupMenuItems(ctx, tx, lines)
	if err != nil {
		return Order{}, err
	}

	var o Order
	err = tx.QueryRowContext(ctx,
		`INSERT INTO orders (check_id) VALUES ($1) RETURNING id, ordered_at`, checkID,
	).Scan(&o.ID, &o.OrderedAt)
	if err != nil {
		return Order{}, fmt.Errorf("order: 注文を保存できませんでした: %w", err)
	}

	o.Items = make([]Item, 0, len(lines))
	for _, l := range lines {
		m := menu[l.MenuItemID]
		// 品名と単価はメニューから写し取る。後でメニューの価格が変わっても
		// この明細の金額は変わらない。
		it := Item{MenuItemID: l.MenuItemID, Name: m.name, PriceYen: m.priceYen, Quantity: l.Quantity}
		err := tx.QueryRowContext(ctx,
			`INSERT INTO order_items (order_id, menu_item_id, name, price_yen, quantity)
			 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			o.ID, it.MenuItemID, it.Name, it.PriceYen, it.Quantity,
		).Scan(&it.ID)
		if err != nil {
			return Order{}, fmt.Errorf("order: 明細を保存できませんでした: %w", err)
		}
		o.Items = append(o.Items, it)
	}

	if err := tx.Commit(); err != nil {
		return Order{}, fmt.Errorf("order: 注文を確定できませんでした: %w", err)
	}
	return o, nil
}

// Pay は利用中の伝票を会計済みにする。
func (r *PostgresRepository) Pay(ctx context.Context, id int64) (Check, error) {
	return r.advance(ctx, id, StatusOpen, StatusPaid, "paid_at", ErrCheckNotOpen)
}

// Close は会計済みの伝票を閉じる。
func (r *PostgresRepository) Close(ctx context.Context, id int64) (Check, error) {
	return r.advance(ctx, id, StatusPaid, StatusClosed, "closed_at", ErrCheckNotPaid)
}

// advance は伝票の状態を from から to へ進め、timeColumn に現在時刻を記録する。
// 伝票が from の状態でなければ errWrongStatus を返す。
//
// 「状態を確かめてから更新する」を 2 つの文に分けると、その間に別の要求が
// 割り込み、二重に会計できてしまう。WHERE に元の状態を含めた 1 つの UPDATE に
// まとめることで、確認と更新を不可分にする。同時に来た 2 つ目の要求は、
// 1 つ目が状態を進めた後の行を見るため、一致する行がなく何も更新しない。
func (r *PostgresRepository) advance(
	ctx context.Context, id int64, from, to Status, timeColumn string, errWrongStatus error,
) (Check, error) {
	// timeColumn は呼び出し側の固定の文字列だけを受け取る。利用者の入力を
	// 列名に使うと SQL インジェクションを招くため、ここへ渡してはならない。
	query := `UPDATE checks SET status = $1, ` + timeColumn + ` = now()
	          WHERE id = $2 AND status = $3
	          RETURNING ` + checkColumns

	c, err := scanCheck(r.db.QueryRowContext(ctx, query, string(to), id, string(from)))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// 更新されなかった理由が「伝票がない」のか「状態が違う」のかを区別する。
			return Check{}, r.missingOr(ctx, id, errWrongStatus)
		}
		return Check{}, fmt.Errorf("order: 伝票の状態を %s にできませんでした: %w", to, err)
	}

	checks := []Check{c}
	if err := r.attachOrders(ctx, checks); err != nil {
		return Check{}, err
	}
	return checks[0], nil
}

// missingOr は伝票が存在しなければ ErrCheckNotFound を、存在すれば otherwise を返す。
func (r *PostgresRepository) missingOr(ctx context.Context, id int64, otherwise error) error {
	var exists bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM checks WHERE id = $1)`, id).Scan(&exists)
	if err != nil {
		return fmt.Errorf("order: 伝票の確認に失敗しました: %w", err)
	}
	if !exists {
		return ErrCheckNotFound
	}
	return otherwise
}

// menuSnapshot は明細へ写し取るメニューの値。
type menuSnapshot struct {
	name      string
	priceYen  int
	available bool
}

// lookupMenuItems は注文された品目のメニューをまとめて引き、すべて注文できるかを確かめる。
//
// menu パッケージの Repository を通さず直接引くのは、伝票の確認と同じ
// トランザクションの中で読む必要があるため。
func lookupMenuItems(ctx context.Context, tx *sql.Tx, lines []Line) (map[int64]menuSnapshot, error) {
	ids := make([]int64, 0, len(lines))
	for _, l := range lines {
		ids = append(ids, l.MenuItemID)
	}

	// 品目ごとに問い合わせると、品目の数だけ往復が発生する。ANY でまとめて引く。
	rows, err := tx.QueryContext(ctx,
		`SELECT id, name, price_yen, available FROM menu_items WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, fmt.Errorf("order: メニューの取得に失敗しました: %w", err)
	}
	defer rows.Close()

	menu := make(map[int64]menuSnapshot, len(ids))
	for rows.Next() {
		var (
			id int64
			m  menuSnapshot
		)
		if err := rows.Scan(&id, &m.name, &m.priceYen, &m.available); err != nil {
			return nil, fmt.Errorf("order: メニューの読み取りに失敗しました: %w", err)
		}
		menu[id] = m
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("order: メニューの読み取りに失敗しました: %w", err)
	}

	for _, id := range ids {
		m, ok := menu[id]
		if !ok {
			return nil, ErrMenuItemNotFound
		}
		if !m.available {
			return nil, ErrMenuItemUnavailable
		}
	}
	return menu, nil
}

// attachOrders は伝票それぞれに注文と明細を読み込む。
//
// 伝票ごとに問い合わせると、一覧の件数だけ往復が発生する (N+1 問題)。
// 対象の伝票の注文と明細を 1 回の問い合わせでまとめて引き、振り分ける。
func (r *PostgresRepository) attachOrders(ctx context.Context, checks []Check) error {
	if len(checks) == 0 {
		return nil
	}

	ids := make([]int64, len(checks))
	index := make(map[int64]int, len(checks))
	for i := range checks {
		ids[i] = checks[i].ID
		index[checks[i].ID] = i
		// 注文のない伝票も JSON で null ではなく [] になるよう初期化する。
		checks[i].Orders = []Order{}
	}

	// 注文は必ず 1 つ以上の明細を伴って保存されるため、内部結合で取りこぼしはない。
	rows, err := r.db.QueryContext(ctx, `
		SELECT o.check_id, o.id, o.ordered_at,
		       i.id, i.menu_item_id, i.name, i.price_yen, i.quantity
		FROM orders o
		JOIN order_items i ON i.order_id = o.id
		WHERE o.check_id = ANY($1)
		ORDER BY o.check_id, o.ordered_at, o.id, i.id`, ids)
	if err != nil {
		return fmt.Errorf("order: 注文の取得に失敗しました: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			checkID int64
			o       Order
			it      Item
		)
		err := rows.Scan(&checkID, &o.ID, &o.OrderedAt,
			&it.ID, &it.MenuItemID, &it.Name, &it.PriceYen, &it.Quantity)
		if err != nil {
			return fmt.Errorf("order: 注文の読み取りに失敗しました: %w", err)
		}

		// 明細 1 行ごとに注文の列が繰り返し現れる。並び順を注文単位に揃えているため、
		// 直前と同じ注文であれば明細を足し、違えば新しい注文を始める。
		c := &checks[index[checkID]]
		if n := len(c.Orders); n == 0 || c.Orders[n-1].ID != o.ID {
			c.Orders = append(c.Orders, o)
		}
		last := &c.Orders[len(c.Orders)-1]
		last.Items = append(last.Items, it)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("order: 注文の読み取りに失敗しました: %w", err)
	}
	return nil
}

// scanner は *sql.Row と *sql.Rows のどちらも受け取るための最小の約束。
type scanner interface {
	Scan(dest ...any) error
}

// scanCheck は 1 行を Check へ読み取る。順序は checkColumns と一致させる。
func scanCheck(s scanner) (Check, error) {
	var c Check
	// paid_at と closed_at は NULL を取りうる。ポインタで受けると NULL は nil になる。
	err := s.Scan(&c.ID, &c.TableNo, &c.Status, &c.OpenedAt, &c.PaidAt, &c.ClosedAt)
	if err != nil {
		return Check{}, err
	}
	return c, nil
}

// isUniqueViolation は err が指定した一意制約への違反であるかを返す。
//
// database/sql はドライバ固有のエラーを包まずに返すため、
// errors.As で pgx のエラー型を取り出して SQLSTATE を確かめる。
func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	// 23505 は unique_violation。
	return pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}
