-- メニュー（提供する料理とドリンク）
CREATE TABLE menu_items (
    id         bigserial   PRIMARY KEY,
    name       text        NOT NULL CHECK (name <> ''),
    category   text        NOT NULL CHECK (category IN ('food', 'drink')),
    price_yen  integer     NOT NULL CHECK (price_yen > 0),
    available  boolean     NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- 一覧は分類で絞り込んで表示するため、category に索引を張る。
CREATE INDEX menu_items_category_idx ON menu_items (category);
