package gormx

import (
	"errors"
	"fmt"
	"sync"
	"time"

	dm "github.com/jasonlabz/gorm-dm-driver"
	"github.com/jasonlabz/oracle"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

var (
	dbMap    *sync.Map
	dbLogger gormLogger.Interface

	mockMode = false

	ErrDBConfigIsNil   = errors.New("db config is nil")
	ErrDBInstanceIsNil = errors.New("db instance is nil")
	ErrSqlDBIsNil      = errors.New("sql db is nil")
	ErrDBNameIsEmpty   = errors.New("empty db name")
)

func init() {
	dbMap = &sync.Map{}
	dbLogger = NewLogger(
		&gormLogger.Config{
			SlowThreshold:             time.Second,       // Slow SQL threshold
			LogLevel:                  gormLogger.Silent, // Log level
			IgnoreRecordNotFoundError: false,             // Ignore ErrRecordNotFound error for logger
			Colorful:                  false,             // Disable color
		},
	)
}

// InitWithDB init database instance with db instance
func InitWithDB(dbName string, db *gorm.DB) error {
	if db == nil {
		return errors.New("no db")
	}
	if dbName == "" {
		return errors.New("no db name")
	}
	_, ok := dbMap.Load(dbName)
	if ok {
		return nil
	}
	// Store database
	dbMap.Store(dbName, db)
	return nil
}

// InitConfig init database instance with db configuration and dialect
func InitConfig(config *Config) error {
	if config == nil {
		return errors.New("no db config")
	}

	if config.DSN == "" &&
		config.GenDSN() == "" {
		return errors.New("no db dsn")
	}

	if config.DBName == "" {
		config.DBName = config.GenDSN()
	}

	_, ok := dbMap.Load(config.DBName)
	if ok {
		return nil
	}

	var dialect gorm.Dialector
	switch config.DBType {
	case DatabaseTypeMySQL:
		dialect = mysql.Open(config.DSN)
	case DatabaseTypePostgres:
		dialect = postgres.Open(config.DSN)
	case DatabaseTypeSqlserver:
		dialect = sqlserver.Open(config.DSN)
	case DatabaseTypeOracle:
		dialect = oracle.Open(config.DSN)
	case DatabaseTypeSQLite:
		dialect = sqlite.Open(config.DSN)
	case DatabaseTypeDM:
		dialect = dm.Open(config.DSN)
	default:
		return errors.New(fmt.Sprintf("unsupported dbType: %s", string(config.DBType)))
	}

	db, err := gorm.Open(dialect, &gorm.Config{
		Logger: dbLogger.LogMode(config.GetLogMode()),
	})
	if err != nil {
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	if config.MaxOpenConn == 0 {
		config.MaxOpenConn = defaultConfig.MaxOpenConn
	}
	if config.MaxIdleConn == 0 {
		config.MaxIdleConn = defaultConfig.MaxIdleConn
	}
	if config.ConnMaxLifeTime == 0 {
		config.ConnMaxLifeTime = defaultConfig.ConnMaxLifeTime
	}
	sqlDB.SetMaxOpenConns(config.MaxOpenConn)
	sqlDB.SetMaxIdleConns(config.MaxIdleConn)
	sqlDB.SetConnMaxLifetime(config.ConnMaxLifeTime)

	// Store database
	dbMap.Store(config.DBName, db)
	return nil
}

func GetDB(name string) (*gorm.DB, error) {
	db, ok := dbMap.Load(name)
	if !ok {
		return nil, errors.New("no db instance")
	}
	return db.(*gorm.DB), nil
}

func GetDBWithPanic(name string) *gorm.DB {
	db, ok := dbMap.Load(name)
	if !ok {
		panic("no db instance")
	}
	return db.(*gorm.DB)
}

func DefaultMaster() *gorm.DB {
	if mockMode {
		return GetDBWithPanic(DBNameMock)
	}
	return GetDBWithPanic(DefaultDBNameMaster)
}

func DefaultSlave() *gorm.DB {
	if mockMode {
		return GetDBWithPanic(DBNameMock)
	}
	return GetDBWithPanic(DefaultDBNameSlave)
}

func SetMockMode(flag bool) {
	mockMode = flag
}

func IsErrNoRows(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// Close close database
func Close(name string) error {
	if name == "" {
		return ErrDBNameIsEmpty
	}
	v, ok := dbMap.LoadAndDelete(name)
	if !ok || v == nil {
		return nil
	}
	db, err := v.(*gorm.DB).DB()
	if err != nil {
		return err
	}
	if db == nil {
		return nil
	}
	return db.Close()
}

// Ping verifies a connection to the database is still alive, establishing a connection if necessary
func Ping(dbName string) error {
	db, err := GetDB(dbName)
	if err != nil {
		return err
	}
	if db == nil {
		return ErrDBInstanceIsNil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	if sqlDB == nil {
		return ErrSqlDBIsNil
	}
	return sqlDB.Ping()
}
