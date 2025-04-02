package database

import (
	"fmt"
	"os"
	"strings"
	"time"
	"vesta/utils"

	"github.com/fatih/color"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Init() (*gorm.DB, error) {
	args := os.Args
	connStr := ""
	for _, arg := range args {
		if strings.HasPrefix(arg, "-db=") {
			connStr = strings.TrimPrefix(arg, "-db=")
			break
		}
	}

	if connStr == "" {
		return nil, fmt.Errorf("database connection string not provided. Use -db=<connection_string>")
	}

	dsn := connStr

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		NowFunc: time.Now().UTC,
		Logger:  logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %v", err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err = sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	utils.LogWithTimestamp(color.BlueString, false, "Connected to database")
	DB = db
	return db, nil
}

func Get() *gorm.DB {
	return DB
}

func Close() error {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err != nil {
			return fmt.Errorf("failed to get database instance: %v", err)
		}
		return sqlDB.Close()
	}
	return nil
}
