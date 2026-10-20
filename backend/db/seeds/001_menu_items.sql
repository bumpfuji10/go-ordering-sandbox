-- 開発用のメニュー
--
-- 画面や API を動かして確かめるための仮のデータ。
-- スキーマの定義ではないため migrations とは分けて置く。
INSERT INTO menu_items (name, category, price_yen, available) VALUES
    ('枝豆', 'food', 380, true),
    ('唐揚げ', 'food', 680, true),
    ('だし巻き卵', 'food', 580, true),
    ('刺身の盛り合わせ', 'food', 1280, true),
    -- 品切れの品目。availableOnly で除外されることを確かめるために置く。
    ('焼きおにぎり', 'food', 420, false),
    ('生ビール', 'drink', 600, true),
    ('ハイボール', 'drink', 500, true),
    ('烏龍茶', 'drink', 350, true),
    ('日本酒（一合）', 'drink', 750, false);
