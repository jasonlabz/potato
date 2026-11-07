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

// DSN 的形状（模板）、各位置的转义规则、参数拼接方式都集中在 dsn.go 的 dsnDialects 里，
// 新增数据库类型只需在 dsnDialects 里加一条，无需改动 genDSN。
