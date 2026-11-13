-- 0002（SQLite 方言）：频道 / 媒体源表。
-- 时间列用 TEXT 存 RFC3339，避开三方言 TIMESTAMP 的 Scan 差异。

CREATE TABLE IF NOT EXISTS channels (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	slug        TEXT NOT NULL UNIQUE,
	name        TEXT NOT NULL,
	source_url  TEXT NOT NULL,
	status      TEXT NOT NULL DEFAULT 'idle',
	created_at  TEXT NOT NULL,
	updated_at  TEXT NOT NULL
);
