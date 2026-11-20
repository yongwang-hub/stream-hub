-- 0003（SQLite 方言）：运行时系统配置表，固定单行 id = 1。
-- 只存「前端可改」字段；型号/序列号/版本/授权/机器码是启动期只读元数据，不入库。
-- 时间列用 TEXT 存 RFC3339，避开三方言 TIMESTAMP 的 Scan 差异。

CREATE TABLE IF NOT EXISTS sys_config (
	id              INTEGER PRIMARY KEY CHECK (id = 1),
	product_name    TEXT NOT NULL,
	login_bg        TEXT NOT NULL DEFAULT '',
	product_icon    TEXT NOT NULL DEFAULT '',
	warning_text    TEXT NOT NULL DEFAULT '',
	watermark       INTEGER NOT NULL DEFAULT 0,
	activation_code TEXT NOT NULL DEFAULT '',
	updated_at      TEXT NOT NULL
);
