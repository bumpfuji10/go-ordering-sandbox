-- 1 つのテーブルに、退店していない伝票を同時に 2 枚作らせない。
--
-- 002 では open の行だけを一意性の対象にしていた。しかし会計を済ませても、
-- お客様は退店するまで席にいる。paid の伝票が残るテーブルに新しい伝票を
-- 開けると、1 つの席に 2 組がいることになってしまうため、paid も対象に含める。
--
-- closed の行は対象外のままとし、退店した後は同じ席に次の組を案内できる。
DROP INDEX checks_open_table_no_idx;

CREATE UNIQUE INDEX checks_seated_table_no_idx
    ON checks (table_no)
    WHERE status IN ('open', 'paid');
