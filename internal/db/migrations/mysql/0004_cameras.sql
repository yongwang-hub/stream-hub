-- 0004（MySQL 方言）：摄像机 / 设备表。
-- DDL 隐式提交事务，靠 IF NOT EXISTS 保证重跑幂等。

CREATE TABLE IF NOT EXISTS cameras (
	id          BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
	name        VARCHAR(64) NOT NULL,
	rtsp_url    VARCHAR(512) NOT NULL,
	enabled     TINYINT(1) NOT NULL DEFAULT 1,
	created_at  VARCHAR(32) NOT NULL,
	updated_at  VARCHAR(32) NOT NULL,
	UNIQUE KEY uk_cameras_rtsp_url (rtsp_url)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
