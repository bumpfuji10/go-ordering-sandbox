-- 注文の明細（1 品目ぶん）
--
-- 品名と単価はメニューを参照せず、注文した時点の値をここへ写し取る。
-- 参照のままにすると、メニューを値上げした瞬間に過去の伝票の金額まで
-- 変わってしまい、会計の記録として成り立たないため。
CREATE TABLE order_items (
    id           bigserial PRIMARY KEY,
    -- 注文が消えれば、その明細も残す意味がないため連鎖削除する。
    order_id     bigint    NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    -- どのメニューを頼んだかの参照。
    -- 連鎖削除は付けない。メニューを 1 つ消したら過去の売上記録まで
    -- 消えてしまうため。提供をやめる場合は menu_items.available を倒す。
    menu_item_id bigint    NOT NULL REFERENCES menu_items(id),
    name         text      NOT NULL CHECK (name <> ''),
    price_yen    integer   NOT NULL CHECK (price_yen > 0),
    quantity     integer   NOT NULL CHECK (quantity > 0)
);

-- 注文に紐づく明細を引くため索引を張る。
CREATE INDEX order_items_order_id_idx ON order_items (order_id);
