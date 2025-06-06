package gormx

import (
	"errors"
	"fmt"
	"sync"

	dm "github.com/jasonlabz/gorm-dm-driver"
	"github.com/jasonlabz/oracle"
	"github.com/jasonlabz/potato/log"
	"github.com/jasonlabz/sqlite"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
)

var (
	dbMap    *sync.Map
	mockMode = false

	ErrDBConfigIsNil   = errors.New("db config is nil")
	ErrDBInstanceIsNil = errors.New("db instance is nil")
	ErrSqlDBIsNil      = errors.New("sql db is nil")
	ErrDBNameIsEmpty   = errors.New("empty db name")
)

func init() {
	dbMap = &sync.Map{}

}

// StoreDB store database instance
func StoreDB(dbName string, db *gorm.DB) error {
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
func InitConfig(config *Config) (db *gorm.DB, err error) {
	if config == nil {
		return nil, errors.New("no db config")
	}

	if config.DSN == "" &&
		config.GenDSN() == "" {
		return nil, errors.New("no db dsn")
	}

	if config.DBName == "" {
		config.DBName = config.GenDSN()
	}

	dbLoaded, ok := dbMap.Load(config.DBName)
	if ok {
		return dbLoaded.(*gorm.DB), nil
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
		return nil, errors.New(fmt.Sprintf("unsupported dbType: %s", string(config.DBType)))
	}

	if config.Logger == nil {
		config.Logger = LoggerAdapter(log.GetLogger(zap.AddCallerSkip(3)))
	}

	dbOpened, err := gorm.Open(dialect, &gorm.Config{
		Logger: config.Logger.LogMode(config.GetLogMode()),
	})

	if err != nil {
		return nil, err
	}

	sqlDB, err := dbOpened.DB()
	if err != nil {
		return nil, err
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
	dbMap.Store(config.DBName, dbOpened)
	return dbOpened, nil
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
