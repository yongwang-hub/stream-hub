// 方言层：把"只有数据库驱动不同"的差异全部收口到这一处，业务 SQL 仍保持 `?` 风格手写。
//
// 设计取舍（延续本项目"零 ORM、轻依赖、门面隔离"的取向）：不引 GORM，而是给每种数据库
// 建一个极薄的 Dialect——它只回答四类问题：用哪个驱动打开、占位符怎么绑、幂等插入怎么写、
// 唯一冲突怎么识别、DDL 走哪个目录。切换只发生在启动时读配置的一次性装配，运行期不可换。
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"   // 注册 "mysql" 驱动（纯 Go）+ 供错误码识别
	"github.com/jackc/pgx/v5/pgconn"   // PG 唯一冲突错误码识别
	_ "github.com/jackc/pgx/v5/stdlib" // 注册 "pgx" 驱动（database/sql 兼容层）
	_ "modernc.org/sqlite"             // 注册 "sqlite" 驱动（纯 Go，无 CGO）
)

// 支持的驱动标识（对应 config.database.driver）。
const (
	SQLite   = "sqlite"
	MySQL    = "mysql"
	Postgres = "postgres"
)

// Dialect 描述一种数据库的方言差异。零值等价于 SQLite（够用即可，不导出字段）。
type Dialect struct {
	kind string
}

// For 把配置里的 driver 文本映射成 Dialect；无法识别返回 ok=false（如 "memory"）。
func For(driver string) (Dialect, bool) {
	switch strings.ToLower(driver) {
	case SQLite:
		return Dialect{SQLite}, true
	case MySQL:
		return Dialect{MySQL}, true
	case Postgres, "postgresql", "pg":
		return Dialect{Postgres}, true
	default:
		return Dialect{}, false
	}
}

func (d Dialect) Name() string { return d.kind }

// DriverName 返回 database/sql 注册名。注意 Postgres 用 pgx 的 "pgx"。
func (d Dialect) DriverName() string {
	switch d.kind {
	case MySQL:
		return "mysql"
	case Postgres:
		return "pgx"
	default:
		return "sqlite"
	}
}

// Rebind 把 `?` 风格占位符转成该方言的形式：Postgres 需要 $1,$2…，其余原样返回。
// 前提：手写 SQL 里没有出现在字符串字面量中的 `?`（本项目全部是参数占位符，满足）。
func (d Dialect) Rebind(q string) string {
	if d.kind != Postgres {
		return q
	}
	var b strings.Builder
	b.Grow(len(q) + 8)
	n := 0
	for i := 0; i < len(q); i++ {
		if q[i] == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
		} else {
			b.WriteByte(q[i])
		}
	}
	return b.String()
}

// IgnoredInsert 把以 sqlite 范式书写的 `INSERT OR IGNORE INTO …` 改写成各方言的幂等插入：
//   - sqlite：原样
//   - mysql：INSERT IGNORE INTO …
//   - postgres：INSERT INTO … ON CONFLICT DO NOTHING
//
// 这样 seed 只需按一种"权威写法"维护，翻译交给方言层。
func (d Dialect) IgnoredInsert(q string) string {
	const canon = "INSERT OR IGNORE INTO"
	if !strings.HasPrefix(q, canon) {
		return q
	}
	switch d.kind {
	case MySQL:
		return "INSERT IGNORE INTO" + q[len(canon):]
	case Postgres:
		return "INSERT INTO" + q[len(canon):] + " ON CONFLICT DO NOTHING"
	default:
		return q
	}
}

// SupportsLastInsertID：pgx 驱动的 Result.LastInsertId 不可靠，Postgres 需改用 `RETURNING id`。
func (d Dialect) SupportsLastInsertID() bool { return d.kind != Postgres }

// MigrationsSubdir 对应内嵌目录名 migrations/<sub>，每个方言一份 DDL。
func (d Dialect) MigrationsSubdir() string { return d.kind }

// InitConn 做该方言的连接级初始化：sqlite 需每连接开外键（默认关），其余无操作。
func (d Dialect) InitConn(db *sql.DB) error {
	if d.kind == SQLite {
		if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
			return fmt.Errorf("enable foreign keys: %w", err)
		}
	}
	return nil
}

// Pool 是连接池调优参数（对 MySQL/PG 才真正生效；SQLite 走单写者策略会忽略它们）。
// 零值字段由 Open 回退到安全默认——这样测试直接传 db.Pool{} 也不会退化到 database/sql
// 那个「只留 2 条空闲」的坑默认值。
type Pool struct {
	MaxOpenConns    int           // 池内最大并发连接数（含在用+空闲）
	MaxIdleConns    int           // 最大空闲连接数（建议 == MaxOpenConns，避免频繁建连/断连）
	ConnMaxLifetime time.Duration // 单条连接最长存活（定期回收，应对服务端 wait_timeout/网络抖动）
}

// Open 按方言打开连接池：sql.Open → Ping 体检 → 连接池调优。
// SQLite 用 SetMaxOpenConns(1) 串行写避免 "database is locked"；MySQL/PG 显式设池，
// 否则 database/sql 默认只留 2 条空闲连接，高并发下会疯狂建连/断连拖垮吞吐。
func (d Dialect) Open(dsn string, pool Pool) (*sql.DB, error) {
	sqldb, err := sql.Open(d.DriverName(), dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", d.kind, err)
	}
	if err := sqldb.Ping(); err != nil { // Ping 才真正建连，第一时间暴露 DSN/服务错误
		sqldb.Close()
		return nil, fmt.Errorf("ping %s: %w", d.kind, err)
	}

	if d.kind == SQLite {
		// 单文件库：串行化写避免锁库；空闲/寿命对内存或本地文件无意义，设 1 即可。
		sqldb.SetMaxOpenConns(1)
		sqldb.SetMaxIdleConns(1)
		return sqldb, nil
	}

	maxOpen := pool.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 25
	}
	maxIdle := pool.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = maxOpen // 官方建议 idle==open：空闲连接不销毁，直接复用
	}
	life := pool.ConnMaxLifetime
	if life <= 0 {
		life = 30 * time.Minute
	}
	sqldb.SetMaxOpenConns(maxOpen)
	sqldb.SetMaxIdleConns(maxIdle)
	sqldb.SetConnMaxLifetime(life)
	return sqldb, nil
}

// IsUniqueErr 判定驱动错误是否为唯一约束冲突——用于把数据库报错翻译成业务哨兵错误。
// 三种驱动给出的错误类型完全不同，正是需要方言收口的典型点。
func (d Dialect) IsUniqueErr(err error) bool {
	if err == nil {
		return false
	}
	switch d.kind {
	case MySQL:
		var me *mysql.MySQLError
		if errors.As(err, &me) {
			return me.Number == 1062 // 1062 = ER_DUP_ENTRY
		}
		return false
	case Postgres:
		var pe *pgconn.PgError
		if errors.As(err, &pe) {
			return pe.Code == "23505" // 23505 = unique_violation
		}
		return false
	default: // sqlite：modernc 无导出错误码，按报错文本识别
		return strings.Contains(strings.ToLower(err.Error()), "unique constraint")
	}
}
