-- 伝票（1 組の来店につき 1 枚）
--
-- 注文は食事中に何度も発生するが、会計は退店時に一度だけ行われる。
-- 注文の単位と会計の単位が異なるため、複数の注文をまとめる入れ物として伝票を置く。
--
-- 状態は入店から退店まで次の順に進み、逆戻りはしない。
--   open (利用中) -> paid (会計済) -> closed (退店済)
CREATE TABLE checks (
    id         bigserial   PRIMARY KEY,
    table_no   integer     NOT NULL CHECK (table_no > 0),
    status     text        NOT NULL DEFAULT 'open'
                           CHECK (status IN ('open', 'paid', 'closed')),
    opened_at  timestamptz NOT NULL DEFAULT now(),
    -- 会計前・退店前は時刻が存在しないため NULL を許容する。
    paid_at    timestamptz,
    closed_at  timestamptz,

    -- 状態と時刻の辻褄が合わない行を作らせない。
    -- 例えば status が paid なのに paid_at が空、といった状態を防ぐ。
    CONSTRAINT checks_paid_at_matches_status CHECK (
        (status = 'open'   AND paid_at IS NULL)
        OR (status IN ('paid', 'closed') AND paid_at IS NOT NULL)
    ),
    CONSTRAINT checks_closed_at_matches_status CHECK (
        (status IN ('open', 'paid') AND closed_at IS NULL)
        OR (status = 'closed' AND closed_at IS NOT NULL)
    )
);

-- 1 つのテーブルに利用中の伝票を同時に 2 枚作らせない。
--
-- WHERE 句により open の行だけが一意性の対象になる。
-- 会計済み・退店済みの行は同じ table_no が何件あっても構わないため、
-- 同じ席に何組でも続けて案内できる。
CREATE UNIQUE INDEX checks_open_table_no_idx
    ON checks (table_no)
    WHERE status = 'open';

-- 利用中の伝票の一覧は頻繁に引くため、状態で絞り込めるようにする。
CREATE INDEX checks_status_idx ON checks (status);
