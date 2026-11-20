-- 0003（MySQL 方言）：运行时系统配置表，固定单行 id = 1。
-- DDL 隐式提交事务，靠 IF NOT EXISTS 保证重跑幂等。

CREATE TABLE IF NOT EXISTS sys_config (
	id              BIGINT NOT NULL PRIMARY KEY,
	product_name    VARCHAR(255) NOT NULL,
	login_bg        VARCHAR(512) NOT NULL DEFAULT '',
	product_icon    VARCHAR(512) NOT NULL DEFAULT '',
	warning_text    VARCHAR(512) NOT NULL DEFAULT '',
	watermark       TINYINT(1) NOT NULL DEFAULT 0,
	activation_code TEXT NOT NULL,
	updated_at      VARCHAR(32) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
