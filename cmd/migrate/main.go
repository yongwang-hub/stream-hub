// 独立迁移命令：只跑迁移，不启动 HTTP 服务，供部署流水线/CI 使用。
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"stream-hub/internal/config"
	"stream-hub/internal/db"
)

func main() {
	configPath := flag.String("config", "config/config.yaml", "配置文件路径")
	flag.Parse()
	if err := run(*configPath); err != nil {
		log.Printf("migrate failed: %v", err)
		os.Exit(1)
	}
}

func run(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	if strings.EqualFold(cfg.Database.Driver, "memory") {
		return fmt.Errorf("driver=memory has no schema to migrate")
	}
	d, ok := db.For(cfg.Database.Driver)
	if !ok {
		return fmt.Errorf("unknown database.driver %q", cfg.Database.Driver)
	}
	sqldb, err := d.Open(cfg.Database.DSN, db.Pool{
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime.Std(),
	})
	if err != nil {
		return err
	}
	defer sqldb.Close()
	if err := db.Migrate(sqldb, d); err != nil {
		return err
	}
	log.Printf("migrations applied: driver=%s", d.Name())
	return nil
}
