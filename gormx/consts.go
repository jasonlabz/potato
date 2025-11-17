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
	DatabaseTypeMySQL:     "%s:%s@tcp(%s:%d)/%s",                                  // args: [{"name":"charset","value":"utf8"},{"name":"parseTime","value":"True"},{"name":"loc","value":"Local"}...]
	DatabaseTypePostgres:  "user=%s password=%s host=%s port=%d dbname=%s",        // args: [{"name":"sslmode","value":"disable"},{"name":"TimeZone","value":"Asia/Shanghai"}...]
	DatabaseTypeSqlserver: "user id=%s;password=%s;server=%s;port=%d;database=%s", // args: [{"name":"encrypt","value":"disable"}...`]
}

// DatabaseDsnSepMap 关系型数据库类型DSN 参数拼接
var DatabaseDsnSepMap = map[DatabaseType]string{
	DatabaseTypeSQLite:    "&",
	DatabaseTypeDM:        "&",
	DatabaseTypeOracle:    "&",
	DatabaseTypeMySQL:     "&",
	DatabaseTypePostgres:  " ",
	DatabaseTypeSqlserver: ";",
}

// DatabaseDsnPrefixMap 关系型数据库类型DSN 参数拼接
var DatabaseDsnPrefixMap = map[DatabaseType]string{
	DatabaseTypeSQLite:    "?",
	DatabaseTypeDM:        "?",
	DatabaseTypeOracle:    "?",
	DatabaseTypeMySQL:     "?",
	DatabaseTypePostgres:  " ",
	DatabaseTypeSqlserver: ";",
}

// DatabaseDsnEqualSignMap 关系型数据库类型DSN 参数拼接
// var DatabaseDsnEqualSignMap = map[DatabaseType]string{
//	DatabaseTypeSQLite:    "%s=%s",
//	DatabaseTypeDM:        "%s=%s",
//	DatabaseTypeOracle:    "%s=%s",
//	DatabaseTypeMySQL:     "%s=%s",
//	DatabaseTypePostgres:  "%s=%s",
//	DatabaseTypeSqlserver: "%s=%s",
// }
