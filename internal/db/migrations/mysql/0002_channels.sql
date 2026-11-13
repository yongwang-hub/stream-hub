-- 0002（MySQL 方言）：频道 / 媒体源表。
-- 时间列用 VARCHAR(32) 存 RFC3339；DDL 隐式提交事务，靠 IF NOT EXISTS 保证重跑幂等。

CREATE TABLE IF NOT EXISTS channels (
	id          BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
	slug        VARCHAR(64) NOT NULL UNIQUE,
	name        VARCHAR(255) NOT NULL,
	source_url  TEXT NOT NULL,
	status      VARCHAR(16) NOT NULL DEFAULT 'idle',
	created_at  VARCHAR(32) NOT NULL,
	updated_at  VARCHAR(32) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
