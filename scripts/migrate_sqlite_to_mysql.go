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
	target, err := gorm.Open(gormmysql.Open(*mysqlDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("open mysql failed: %v", err)
	}
	if err := autoMigrate(target); err != nil {
		log.Fatalf("migrate mysql schema failed: %v", err)
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
