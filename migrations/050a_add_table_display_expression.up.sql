-- Per-table display expression: how a single row of this table should be
-- labelled in UIs (e.g. `author_name + " " + screen_name`). Concatenation of
-- column tokens and quoted string literals joined with '+'; evaluated by the
-- frontend, never interpolated into SQL.
ALTER TABLE tables
    ADD COLUMN IF NOT EXISTS display_expression TEXT;
