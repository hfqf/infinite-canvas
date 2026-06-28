package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"github.com/basketikun/infinite-canvas/model"
	"github.com/glebarez/sqlite"
	"github.com/joho/godotenv"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const defaultRemovedPromptCategories = "gpt-image-2-prompts,youmind-gpt-image-2,youmind-nano-banana-pro"

type cleanupOptions struct {
	driver     string
	dsn        string
	categories string
}

func main() {
	defaults := defaultCleanupOptions()
	driver := flag.String("driver", defaults.driver, "database driver: sqlite, mysql, or postgresql")
	dsn := flag.String("dsn", defaults.dsn, "database DSN")
	categoriesText := flag.String("categories", defaults.categories, "comma separated prompt categories to delete")
	dryRun := flag.Bool("dry-run", false, "count rows without deleting")
	flag.Parse()

	categories := splitCategories(*categoriesText)
	if len(categories) == 0 {
		log.Fatal("missing categories")
	}
	db, err := gorm.Open(dialector(*driver, *dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("open database failed: %v", err)
	}
	var count int64
	if err := db.Model(&model.Prompt{}).Where("category IN ?", categories).Count(&count).Error; err != nil {
		log.Fatalf("count prompts failed: %v", err)
	}
	if *dryRun {
		log.Printf("dry run: %d prompts would be deleted from categories: %s", count, strings.Join(categories, ", "))
		return
	}
	if err := db.Where("category IN ?", categories).Delete(&model.Prompt{}).Error; err != nil {
		log.Fatalf("delete prompts failed: %v", err)
	}
	log.Printf("deleted %d prompts from categories: %s", count, strings.Join(categories, ", "))
}

func defaultCleanupOptions() cleanupOptions {
	_ = godotenv.Load()
	return cleanupOptions{
		driver:     firstNonEmpty(os.Getenv("DATABASE_DRIVER"), os.Getenv("STORAGE_DRIVER"), "sqlite"),
		dsn:        firstNonEmpty(os.Getenv("DATABASE_DSN"), "data/infinite-canvas.db"),
		categories: defaultRemovedPromptCategories,
	}
}

func dialector(driver string, dsn string) gorm.Dialector {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "mysql":
		return gormmysql.New(gormmysql.Config{DSN: dsn, DefaultStringSize: 191})
	case "postgres", "postgresql":
		return postgres.Open(dsn)
	default:
		return sqlite.Open(dsn)
	}
}

func splitCategories(value string) []string {
	items := []string{}
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
