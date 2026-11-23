-- 0004（SQLite 方言）：摄像机 / 设备表。
-- RTSP 设备只需要名称、流地址与启停状态；地址唯一，避免重复接入同一路流。

CREATE TABLE IF NOT EXISTS cameras (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	name        TEXT NOT NULL,
	rtsp_url    TEXT NOT NULL,
	enabled     INTEGER NOT NULL DEFAULT 1,
	created_at  TEXT NOT NULL,
	updated_at  TEXT NOT NULL,
	UNIQUE (rtsp_url)
);
