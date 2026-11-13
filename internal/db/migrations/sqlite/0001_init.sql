-- 0001（SQLite 方言）：demo 业务表 users + 真实用户体系 sys_* 五表。
-- schema 的唯一来源就是迁移文件（dbstore/seed 不再自己建表）。
-- 自增用 INTEGER PRIMARY KEY AUTOINCREMENT；外键强制由 Dialect.InitConn 开 PRAGMA。

CREATE TABLE IF NOT EXISTS users (
	id    INTEGER PRIMARY KEY AUTOINCREMENT,
	name  TEXT NOT NULL UNIQUE,
	email TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sys_users (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    username             TEXT    NOT NULL UNIQUE,
    password_hash        TEXT    NOT NULL,
    email                TEXT,
    enabled              INTEGER NOT NULL DEFAULT 1,
    password_changed_at  TIMESTAMP,
    created_at           TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sys_roles (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    description TEXT
);

CREATE TABLE IF NOT EXISTS sys_permissions (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    code TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS sys_user_roles (
    user_id INTEGER NOT NULL REFERENCES sys_users(id) ON DELETE CASCADE,
    role_id INTEGER NOT NULL REFERENCES sys_roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE IF NOT EXISTS sys_role_permissions (
    role_id       INTEGER NOT NULL REFERENCES sys_roles(id) ON DELETE CASCADE,
    permission_id INTEGER NOT NULL REFERENCES sys_permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);
