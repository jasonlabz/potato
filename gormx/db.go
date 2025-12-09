package gormx

import (
	"errors"
	"fmt"
	"sync"
	"time"

	dm "github.com/jasonlabz/gorm-dm-driver"
	"github.com/jasonlabz/oracle"
	"github.com/jasonlabz/sqlite"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"

	"github.com/jasonlabz/potato/log"
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

	dsn, replicas := config.extractDSN()
	if dsn == "" {
		return nil, errors.New("no db dsn")
	}

	if config.DBName == "" {
		config.DBName = dsn
	}

	dbLoaded, ok := dbMap.Load(config.DBName)
	if ok {
		return dbLoaded.(*gorm.DB), nil
	}
	dialector := getDialector(config.DBType, dsn)
	if dialector == nil {
		return nil, fmt.Errorf("unsupported dbType: %s", string(config.DBType))
	}
	if config.Logger == nil {
		config.Logger = LoggerAdapter(log.GetLogger(zap.AddCallerSkip(3)))
	}

	gConfig := &gorm.Config{
		Logger: config.Logger.LogMode(config.GetLogMode()),
	}
	dbOpened, err := gorm.Open(dialector, gConfig)
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
	if config.ConnMaxIdleTime == 0 {
		config.ConnMaxIdleTime = defaultConfig.ConnMaxIdleTime
	}
	sqlDB.SetMaxOpenConns(config.MaxOpenConn)
	sqlDB.SetConnMaxIdleTime(time.Duration(config.ConnMaxIdleTime) * time.Millisecond)
	sqlDB.SetMaxIdleConns(config.MaxIdleConn)
	sqlDB.SetConnMaxLifetime(time.Duration(config.ConnMaxLifeTime) * time.Millisecond)

	if len(replicas[0]) > 0 && len(replicas[1]) > 0 {
		var sourceDialectors []gorm.Dialector
		var replicaDialectors []gorm.Dialector
		for _, replicaDSN := range replicas[0] {
			sourceDialectors = append(sourceDialectors, getDialector(config.DBType, replicaDSN))
		}
		for _, replicaDSN := range replicas[1] {
			replicaDialectors = append(replicaDialectors, getDialector(config.DBType, replicaDSN))
		}

		dbOpened.Use(dbresolver.Register(dbresolver.Config{
			Sources:  sourceDialectors,
			Replicas: replicaDialectors,
			Policy:   dbresolver.RandomPolicy{},
		}).SetConnMaxIdleTime(time.Duration(config.ConnMaxIdleTime) * time.Millisecond).
			SetConnMaxLifetime(time.Duration(config.ConnMaxLifeTime) * time.Millisecond).
			SetMaxIdleConns(config.MaxIdleConn).
			SetMaxOpenConns(config.MaxOpenConn))
	}
	// Store database
	dbMap.Store(config.DBName, dbOpened)
	return dbOpened, nil
}

func getDialector(dbType DatabaseType, dsn string) gorm.Dialector {
	var dialect gorm.Dialector
	switch dbType {
	case DatabaseTypeMySQL:
		dialect = mysql.Open(dsn)
	case DatabaseTypePostgres:
		dialect = postgres.Open(dsn)
	case DatabaseTypeSqlserver:
		dialect = sqlserver.Open(dsn)
	case DatabaseTypeOracle:
		dialect = oracle.Open(dsn)
	case DatabaseTypeSQLite:
		dialect = sqlite.Open(dsn)
	case DatabaseTypeDM:
		dialect = dm.Open(dsn)
	default:
		return nil
	}
	return dialect
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
