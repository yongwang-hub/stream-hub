-- 0003（PostgreSQL 方言）：运行时系统配置表，固定单行 id = 1。
-- 时间列用 TEXT 存 RFC3339；watermark 用 SMALLINT 存 0/1，三方言 Scan 到 int 一致。

CREATE TABLE IF NOT EXISTS sys_config (
	id              BIGINT PRIMARY KEY,
	product_name    TEXT NOT NULL,
	login_bg        TEXT NOT NULL DEFAULT '',
	product_icon    TEXT NOT NULL DEFAULT '',
	warning_text    TEXT NOT NULL DEFAULT '',
	watermark       SMALLINT NOT NULL DEFAULT 0,
	activation_code TEXT NOT NULL DEFAULT '',
	updated_at      TEXT NOT NULL
);
