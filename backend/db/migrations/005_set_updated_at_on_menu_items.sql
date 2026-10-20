-- menu_items.updated_at を更新のたびに自動で書き換える。
--
-- DEFAULT now() が効くのは INSERT のときだけで、UPDATE では元の値が残る。
-- 更新する側が毎回 updated_at を指定する約束にすると、書き忘れた箇所で
-- 時刻が古いまま残るため、データベース側で必ず書き換える。

-- 他のテーブルにも updated_at を持たせた場合に使い回せるよう、
-- テーブルに依存しない関数として定義する。
CREATE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- BEFORE にすることで、書き込まれる前の行 (NEW) を差し替えられる。
CREATE TRIGGER menu_items_set_updated_at
    BEFORE UPDATE ON menu_items
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
