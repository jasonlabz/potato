package gormx

const (
	DefaultDBNameMaster = "__default_master__"
	DefaultDBNameSlave  = "__default_slave__"
	DBNameMock          = "__mock__"
)

type LogMode string

const (
	LogModeSilent LogMode = "silent"
	LogModeInfo   LogMode = "info"
	LogModeWarn   LogMode = "warn"
	LogModeError  LogMode = "error"
)

type DatabaseType string

const (
	DatabaseTypePostgres  DatabaseType = "postgres"
	DatabaseTypeMySQL     DatabaseType = "mysql"
	DatabaseTypeSqlserver DatabaseType = "sqlserver"
	DatabaseTypeOracle    DatabaseType = "oracle"
	DatabaseTypeSQLite    DatabaseType = "sqlite"
	DatabaseTypeDM        DatabaseType = "dm"
)

// DatabaseDsnMap 关系型数据库类型  username、password、address、port、dbname
var DatabaseDsnMap = map[DatabaseType]string{
	DatabaseTypeSQLite:    "%s",
	DatabaseTypeDM:        "dm://%s:%s@%s:%d?schema=%s",
	DatabaseTypeOracle:    "%s/%s@%s:%d/%s",
	DatabaseTypeMySQL:     "%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True&loc=Local&timeout=1000ms",
	DatabaseTypePostgres:  "user=%s password=%s host=%s port=%d dbname=%s sslmode=disable TimeZone=Asia/Shanghai",
	DatabaseTypeSqlserver: "user id=%s;password=%s;server=%s;port=%d;database=%s;encrypt=disable",
}
