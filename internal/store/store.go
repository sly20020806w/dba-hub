package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dba-hub/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(driver, dsn string) (*gorm.DB, error) {
	var dial gorm.Dialector
	switch driver {
	case "mysql":
		dial = mysql.Open(dsn)
	case "sqlite", "":
		if i := strings.LastIndex(dsn, string(os.PathSeparator)); i > 0 {
			_ = os.MkdirAll(dsn[:i], 0o755)
		}
		if dir := filepath.Dir(dsn); dir != "." {
			_ = os.MkdirAll(dir, 0o755)
		}
		dial = sqlite.Open(dsn)
	default:
		return nil, fmt.Errorf("unsupported db driver: %s", driver)
	}
	db, err := gorm.Open(dial, &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.AutoMigrate(model.AllModels()...); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}
