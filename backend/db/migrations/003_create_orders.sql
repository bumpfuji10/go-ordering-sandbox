-- 注文（厨房に伝えた 1 回分）
--
-- 1 枚の伝票に対し、食事中に何度でも追加される。
-- 「生ビールと枝豆」を頼んだ 1 回ぶんがこの 1 行にあたり、
-- 個々の品目は order_items が持つ。
CREATE TABLE orders (
    id         bigserial   PRIMARY KEY,
    -- 伝票が消えれば、その注文も残す意味がないため連鎖削除する。
    check_id   bigint      NOT NULL REFERENCES checks(id) ON DELETE CASCADE,
    ordered_at timestamptz NOT NULL DEFAULT now()
);

-- 伝票の明細を引くたびに check_id で絞り込むため索引を張る。
-- 外部キー制約があっても索引は自動では作られない。
CREATE INDEX orders_check_id_idx ON orders (check_id);
