package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/basketikun/infinite-canvas/config"
	"github.com/basketikun/infinite-canvas/model"
	"github.com/glebarez/sqlite"
	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var promptCategories = []model.PromptCategory{
	{Category: "system", Name: "系统", Description: "系统提示词分类"},
	{Category: "awesome-gpt-image", Name: "Awesome GPT Image", Description: "ZeroLu 的中文 GPT Image 提示词分类", GithubURL: "https://github.com/ZeroLu/awesome-gpt-image", Remote: true},
	{Category: "awesome-gpt4o-image-prompts", Name: "Awesome GPT4o Image Prompts", Description: "ImgEdify 的 GPT-4o 图像提示词分类", GithubURL: "https://github.com/ImgEdify/Awesome-GPT4o-Image-Prompts", Remote: true},
	{Category: "davidwu-gpt-image2-prompts", Name: "awesome-gpt-image2-prompts", Description: "davidwuw0811-boop 整理的 GPT Image 2 提示词分类", GithubURL: "https://github.com/davidwuw0811-boop/awesome-gpt-image2-prompts", Remote: true},
}

var (
	db     *gorm.DB
	dbOnce sync.Once
	dbErr  error
)

// DB 初始化并返回全局数据库连接。
func DB() (*gorm.DB, error) {
	dbOnce.Do(func() {
		driver := strings.ToLower(strings.TrimSpace(config.Cfg.StorageDriver))
		if driver == "" {
			driver = "sqlite"
		}
		dsn := config.Cfg.DatabaseDSN
		if driver == "sqlite" && dsn != ":memory:" {
			_ = os.MkdirAll(filepath.Dir(dsn), 0755)
		}
		if isPostgresDriver(driver) {
			dbErr = ensurePostgresDatabase(dsn)
			if dbErr != nil {
				return
			}
		}
		if driver == "mysql" {
			dbErr = ensureMySQLDatabase(dsn)
			if dbErr != nil {
				return
			}
		}
		db, dbErr = gorm.Open(dialector(driver, dsn), &gorm.Config{})
		if dbErr != nil {
			return
		}
		dbErr = migrateSchema(db, driver)
	})
	return db, dbErr
}

func dialector(driver string, dsn string) gorm.Dialector {
	switch driver {
	case "mysql":
		// DefaultStringSize=191 让带 uniqueIndex 的 string 字段默认建成 VARCHAR(191)，
		// 避免 GORM 默认 longtext 建唯一索引报 "BLOB/TEXT column used in key specification without a key length"。
		// 超过 191 字符的列（如 prompts.prompt、settings.value）由 autoMigrateLongTextColumns 单独改成 LONGTEXT。
		return gormmysql.New(gormmysql.Config{DSN: dsn, DefaultStringSize: 191})
	case "postgres", "postgresql":
		return postgres.Open(dsn)
	default:
		return sqlite.Open(dsn)
	}
}

func isPostgresDriver(driver string) bool {
	return driver == "postgres" || driver == "postgresql"
}

// migrateSchema 处理启动时表结构同步。MySQL 上为了避免 DefaultStringSize=191 与 LONGTEXT 互相 ALTER，遵循：
//   - 表不存在 → AutoMigrate 一次性建表（包含被 promoteLongTextColumns 提升过的 LONGTEXT 列）。
//   - 表已存在 → 只跳过 AutoMigrate；如某些长文本列在建表后被手动加了 gorm:"type:text"，由 promoteLongTextColumns 单独修正。
func migrateSchema(db *gorm.DB, driver string) error {
	models := []any{
		&model.User{},
		&model.EmailVerificationCode{},
		&model.CreditLog{},
		&model.AIImageTask{},
		&model.RechargeOrder{},
		&model.Prompt{},
		&model.Asset{},
		&model.Setting{},
	}
	for _, m := range models {
		if db.Migrator().HasTable(m) {
			continue
		}
		if err := db.AutoMigrate(m); err != nil {
			return err
		}
	}
	if driver == "mysql" {
		if err := autoMigrateLongTextColumns(db); err != nil {
			return err
		}
	}
	return nil
}

// autoMigrateLongTextColumns 把 MySQL 上默认 VARCHAR(191) 但实际存储超过 191 字符的列改成 LONGTEXT。
// 配合 DefaultStringSize=191，避免提示词 / 设置 JSON 等长文本写不进去。
func autoMigrateLongTextColumns(db *gorm.DB) error {
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

func ensureMySQLDatabase(dsn string) error {
	cfg, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		return err
	}
	target := strings.TrimSpace(cfg.DBName)
	if target == "" {
		return nil
	}
	ctx := context.Background()
	targetDB, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	err = targetDB.PingContext(ctx)
	_ = targetDB.Close()
	if err == nil {
		return nil
	}
	if !isMySQLError(err, 1049) {
		return err
	}

	maintenance := cfg.Clone()
	maintenance.DBName = ""
	serverDB, err := sql.Open("mysql", maintenance.FormatDSN())
	if err != nil {
		return err
	}
	defer serverDB.Close()

	_, err = serverDB.ExecContext(ctx, "CREATE DATABASE "+quoteMySQLIdentifier(target)+" CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
	if isMySQLError(err, 1007) {
		return nil
	}
	return err
}

func ensurePostgresDatabase(dsn string) error {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return err
	}
	target := strings.TrimSpace(cfg.Database)
	if target == "" {
		return nil
	}
	ctx := context.Background()
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err == nil {
		_ = conn.Close(ctx)
		return nil
	}
	if !isPostgresError(err, "3D000") {
		return err
	}

	maintenance := cfg.Copy()
	maintenance.Database = "postgres"
	if strings.EqualFold(target, "postgres") {
		maintenance.Database = "template1"
	}
	conn, err = pgx.ConnectConfig(ctx, maintenance)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{target}.Sanitize(), pgx.QueryExecModeExec)
	if isPostgresError(err, "42P04") {
		return nil
	}
	return err
}

func isMySQLError(err error, number uint16) bool {
	var mysqlErr *mysqldriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == number
}

func isPostgresError(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}

func quoteMySQLIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}
