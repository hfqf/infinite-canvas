package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/basketikun/infinite-canvas/model"
	"github.com/glebarez/sqlite"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	sqliteDSN := flag.String("sqlite", "data/infinite-canvas.db", "SQLite database path or DSN")
	mysqlDSN := flag.String("mysql", "", "MySQL DSN, for example user:pass@tcp(127.0.0.1:3306)/infinite_canvas?charset=utf8mb4&parseTime=True&loc=Local")
	truncate := flag.Bool("truncate", false, "delete target MySQL table rows before importing")
	batchSize := flag.Int("batch", 500, "insert batch size")
	flag.Parse()

	if strings.TrimSpace(*mysqlDSN) == "" {
		log.Fatal("missing -mysql DSN")
	}
	if *batchSize <= 0 {
		*batchSize = 500
	}

	source, err := gorm.Open(sqlite.Open(*sqliteDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("open sqlite failed: %v", err)
	}
	target, err := gorm.Open(gormmysql.New(gormmysql.Config{DSN: *mysqlDSN, DefaultStringSize: 191}), &gorm.Config{})
	if err != nil {
		log.Fatalf("open mysql failed: %v", err)
	}
	if err := autoMigrate(source); err != nil {
		log.Fatalf("migrate sqlite schema failed: %v", err)
	}
	if err := dropTargetTables(target); err != nil {
		log.Fatalf("drop mysql tables failed: %v", err)
	}
	if err := autoMigrate(target); err != nil {
		log.Fatalf("migrate mysql schema failed: %v", err)
	}
	if err := promoteLongTextColumns(target); err != nil {
		log.Fatalf("promote longtext columns failed: %v", err)
	}

	if err := copyTable[model.User](source, target, "users", *batchSize, *truncate); err != nil {
		log.Fatal(err)
	}
	if err := copyTable[model.EmailVerificationCode](source, target, "email_verification_codes", *batchSize, *truncate); err != nil {
		log.Fatal(err)
	}
	if err := copyTable[model.CreditLog](source, target, "credit_logs", *batchSize, *truncate); err != nil {
		log.Fatal(err)
	}
	if err := copyTable[model.AIImageTask](source, target, "ai_image_tasks", *batchSize, *truncate); err != nil {
		log.Fatal(err)
	}
	if err := copyTable[model.RechargeOrder](source, target, "recharge_orders", *batchSize, *truncate); err != nil {
		log.Fatal(err)
	}
	if err := copyTable[model.Prompt](source, target, "prompts", *batchSize, *truncate); err != nil {
		log.Fatal(err)
	}
	if err := copyTable[model.Asset](source, target, "assets", *batchSize, *truncate); err != nil {
		log.Fatal(err)
	}
	if err := copyTable[model.Setting](source, target, "settings", *batchSize, *truncate); err != nil {
		log.Fatal(err)
	}
	log.Println("SQLite to MySQL migration completed")
}

func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.User{},
		&model.EmailVerificationCode{},
		&model.CreditLog{},
		&model.AIImageTask{},
		&model.RechargeOrder{},
		&model.Prompt{},
		&model.Asset{},
		&model.Setting{},
	)
}

// dropTargetTables 幂等删除目标 MySQL 中已存在的迁移表，避免 AutoMigrate 在二次运行时对已存在的数据做列类型校验时刷错。
func dropTargetTables(db *gorm.DB) error {
	tables := []string{
		"users",
		"email_verification_codes",
		"credit_logs",
		"ai_image_tasks",
		"recharge_orders",
		"prompts",
		"assets",
		"settings",
	}
	for _, t := range tables {
		stmt := fmt.Sprintf("DROP TABLE IF EXISTS `%s`", t)
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("%s: %w", stmt, err)
		}
	}
	return nil
}

// promoteLongTextColumns 把 DefaultStringSize=191 默认化的长文本列改为 LONGTEXT，
// 避免提示词等字段超过 varchar(191) 写入失败。
func promoteLongTextColumns(db *gorm.DB) error {
	columns := []struct {
		table  string
		column string
	}{
		{"prompts", "prompt"},
		{"prompts", "title"},
		{"prompts", "preview"},
		{"prompts", "tags"},
		{"prompts", "cover_url"},
		{"settings", "value"},
		{"credit_logs", "remark"},
		{"ai_image_tasks", "prompt"},
	}
	for _, c := range columns {
		stmt := fmt.Sprintf("ALTER TABLE `%s` MODIFY `%s` LONGTEXT", c.table, c.column)
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("%s: %w", stmt, err)
		}
	}
	return nil
}

func copyTable[T any](source *gorm.DB, target *gorm.DB, table string, batchSize int, truncate bool) error {
	var items []T
	if err := source.Find(&items).Error; err != nil {
		return fmt.Errorf("read %s failed: %w", table, err)
	}
	if truncate {
		if err := target.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(new(T)).Error; err != nil {
			return fmt.Errorf("truncate %s failed: %w", table, err)
		}
	}
	if len(items) == 0 {
		log.Printf("%s: 0 rows", table)
		return nil
	}
	if err := target.CreateInBatches(items, batchSize).Error; err != nil {
		return fmt.Errorf("write %s failed: %w", table, err)
	}
	log.Printf("%s: %d rows", table, len(items))
	return nil
}
