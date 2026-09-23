package migrate

import (
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/jasonlabz/potato/gormx"
)

// dialect 收敛迁移相关的方言差异：加锁、DDL 幂等判断、追踪表/锁表建表语句。
// 注册到 dialectRegistry 的方言都必须支持迁移；建库能力是可选的，见 dbCreator。
type dialect interface {
	// Locker 返回该方言的分布式锁实现，用于保证多实例并发启动时只有一个实例执行迁移。
	Locker(db *gorm.DB, lockKey string) locker
	// IsIdempotentSkippable 判断迁移执行失败的 err 是否属于"对象已存在"类幂等错误，
	// 仅作为分布式锁之外的兜底，不覆盖其余真实失败。
	IsIdempotentSkippable(err error) bool
	// MigrationsTableSQL 返回版本追踪表 schema_migrations 的建表语句（幂等）。
	MigrationsTableSQL() string
	// LockTableSQL 返回锁表 schema_migrations_lock 的建表语句（幂等）。
	// 仅 tableLocker 类方言实际使用，咨询锁方言保留实现以便随时切换锁策略。
	LockTableSQL() string
}

// dbCreator 是可选的建库能力：连接管理库判断目标库是否存在、不存在则创建。
// 达梦等要求业务库由 DBA 预先手工初始化的方言不实现该接口，EnsureDatabase 会返回
// ErrAutoCreateUnsupported 提示手动建库。
type dbCreator interface {
	// AdminDatabase 连接服务器（而非业务库）时使用的管理库名；返回空字符串表示不需要指定库（如 MySQL）。
	AdminDatabase() string
	// DBExistsQuery 判断目标库是否存在的查询语句，参数为库名。
	DBExistsQuery() string
	// CreateDatabaseSQL 返回建库语句。
	CreateDatabaseSQL(dbName string) string
}

var dialectRegistry = map[gormx.DatabaseType]dialect{
	gormx.DatabaseTypePostgres:  postgresDialect{},
	gormx.DatabaseTypeMySQL:     mysqlDialect{},
	gormx.DatabaseTypeSqlserver: sqlserverDialect{},
	gormx.DatabaseTypeOracle:    oracleDialect{},
	gormx.DatabaseTypeDM:        dmDialect{},
	gormx.DatabaseTypeSQLite:    sqliteDialect{},
}

// lookupDialect 按数据库类型查找迁移方言实现；未注册的类型返回 ok=false。
func lookupDialect(dbType gormx.DatabaseType) (dialect, bool) {
	d, ok := dialectRegistry[dbType]
	return d, ok
}

// lookupDBCreator 按数据库类型查找建库能力；方言未注册或不支持自动建库都返回 ok=false。
func lookupDBCreator(dbType gormx.DatabaseType) (dbCreator, bool) {
	d, ok := lookupDialect(dbType)
	if !ok {
		return nil, false
	}
	creator, ok := d.(dbCreator)
	return creator, ok
}

// ── 建表语句（按方言适配） ──

// genericMigrationsTableSQL 适用于 PostgreSQL / 达梦 / SQLite 的追踪表建表语句。
// 未注册方言也用它兜底（多数关系型数据库兼容该语法子集）。
const genericMigrationsTableSQL = `CREATE TABLE IF NOT EXISTS schema_migrations (
	version VARCHAR(255) NOT NULL,
	type VARCHAR(16) NOT NULL DEFAULT 'ddl' CHECK (type IN ('ddl', 'seed')),
	applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (version, type)
)`

// genericLockTableSQL 适用于 PostgreSQL / 达梦 / SQLite 的锁表建表语句。
const genericLockTableSQL = `CREATE TABLE IF NOT EXISTS schema_migrations_lock (
	lock_key VARCHAR(255) PRIMARY KEY,
	locked_by VARCHAR(255) NOT NULL,
	locked_at TIMESTAMP NOT NULL
)`

// MySQL：显式指定 InnoDB 与 utf8mb4，避免落入实例默认的 MyISAM / latin1。
const mysqlMigrationsTableSQL = `CREATE TABLE IF NOT EXISTS schema_migrations (
	version VARCHAR(255) NOT NULL,
	type VARCHAR(16) NOT NULL DEFAULT 'ddl' CHECK (type IN ('ddl', 'seed')),
	applied_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (version, type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

const mysqlLockTableSQL = `CREATE TABLE IF NOT EXISTS schema_migrations_lock (
	lock_key VARCHAR(255) PRIMARY KEY,
	locked_by VARCHAR(255) NOT NULL,
	locked_at TIMESTAMP NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

// SQL Server：T-SQL 不支持 CREATE TABLE IF NOT EXISTS，用 sys.tables 判断包裹；
// TIMESTAMP 在 SQL Server 中是 rowversion 而非日期时间，必须用 DATETIME2。
const sqlserverMigrationsTableSQL = `IF NOT EXISTS (SELECT 1 FROM sys.tables WHERE name = 'schema_migrations')
CREATE TABLE schema_migrations (
	version NVARCHAR(255) NOT NULL,
	type NVARCHAR(16) NOT NULL DEFAULT 'ddl' CHECK (type IN ('ddl', 'seed')),
	applied_at DATETIME2 NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (version, type)
)`

const sqlserverLockTableSQL = `IF NOT EXISTS (SELECT 1 FROM sys.tables WHERE name = 'schema_migrations_lock')
CREATE TABLE schema_migrations_lock (
	lock_key NVARCHAR(255) PRIMARY KEY,
	locked_by NVARCHAR(255) NOT NULL,
	locked_at DATETIME2 NOT NULL
)`

// Oracle：无 IF NOT EXISTS，用 PL/SQL 块捕获 ORA-00955（名称已被使用）实现幂等；
// VARCHAR 应写作 VARCHAR2。
const oracleMigrationsTableSQL = `BEGIN
	EXECUTE IMMEDIATE 'CREATE TABLE schema_migrations (
		version VARCHAR2(255) NOT NULL,
		type VARCHAR2(16) DEFAULT ''ddl'' NOT NULL CHECK (type IN (''ddl'', ''seed'')),
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (version, type)
	)';
EXCEPTION
	WHEN OTHERS THEN
		IF SQLCODE != -955 THEN
			RAISE;
		END IF;
END;`

const oracleLockTableSQL = `BEGIN
	EXECUTE IMMEDIATE 'CREATE TABLE schema_migrations_lock (
		lock_key VARCHAR2(255) PRIMARY KEY,
		locked_by VARCHAR2(255) NOT NULL,
		locked_at TIMESTAMP NOT NULL
	)';
EXCEPTION
	WHEN OTHERS THEN
		IF SQLCODE != -955 THEN
			RAISE;
		END IF;
END;`

// ── PostgreSQL ──

type postgresDialect struct{}

func (postgresDialect) AdminDatabase() string { return "postgres" }

func (postgresDialect) DBExistsQuery() string {
	return `SELECT 1 FROM pg_database WHERE datname = ?`
}

func (postgresDialect) CreateDatabaseSQL(dbName string) string {
	return fmt.Sprintf(`CREATE DATABASE "%s"`, dbName)
}

func (postgresDialect) Locker(db *gorm.DB, lockKey string) locker {
	return newAdvisoryLocker(db, lockKey, postgresLockDialect{})
}

func (postgresDialect) IsIdempotentSkippable(err error) bool {
	if err == nil {
		return false
	}
	// 42P07 duplicate_table / 42701 duplicate_column / 42710 duplicate_object
	msg := err.Error()
	for _, code := range []string{"42P07", "42701", "42710"} {
		if strings.Contains(msg, code) {
			return true
		}
	}
	return false
}

func (postgresDialect) MigrationsTableSQL() string { return genericMigrationsTableSQL }
func (postgresDialect) LockTableSQL() string       { return genericLockTableSQL }

// ── MySQL ──

type mysqlDialect struct{}

func (mysqlDialect) AdminDatabase() string { return "" }

func (mysqlDialect) DBExistsQuery() string {
	return `SELECT 1 FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = ?`
}

func (mysqlDialect) CreateDatabaseSQL(dbName string) string {
	return fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` DEFAULT CHARACTER SET utf8mb4", dbName)
}

func (mysqlDialect) Locker(db *gorm.DB, lockKey string) locker {
	return newAdvisoryLocker(db, lockKey, mysqlLockDialect{})
}

func (mysqlDialect) IsIdempotentSkippable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// 1050 table already exists / 1060 duplicate column name / 1061 duplicate key name
	for _, code := range []string{"1050", "1060", "1061"} {
		if strings.Contains(msg, code) {
			return true
		}
	}
	return false
}

func (mysqlDialect) MigrationsTableSQL() string { return mysqlMigrationsTableSQL }
func (mysqlDialect) LockTableSQL() string       { return mysqlLockTableSQL }

// ── SQL Server ──

type sqlserverDialect struct{}

func (sqlserverDialect) AdminDatabase() string { return "master" }

func (sqlserverDialect) DBExistsQuery() string {
	return `SELECT 1 FROM sys.databases WHERE name = ?`
}

func (sqlserverDialect) CreateDatabaseSQL(dbName string) string {
	return fmt.Sprintf("CREATE DATABASE [%s]", dbName)
}

func (sqlserverDialect) Locker(db *gorm.DB, lockKey string) locker {
	return newAdvisoryLocker(db, lockKey, sqlserverLockDialect{})
}

func (sqlserverDialect) IsIdempotentSkippable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// 2714 对象已存在 / 1801 数据库已存在 / 21002 列名重复
	for _, code := range []string{"2714", "1801", "21002"} {
		if strings.Contains(msg, code) {
			return true
		}
	}
	return false
}

func (sqlserverDialect) MigrationsTableSQL() string { return sqlserverMigrationsTableSQL }
func (sqlserverDialect) LockTableSQL() string       { return sqlserverLockTableSQL }

// ── Oracle ──

// oracleDialect 只支持迁移，不支持自动建库：Oracle 业务库/schema 通常由 DBA 预先创建，
// 因此不实现 dbCreator。锁实现使用不依赖 DBMS_LOCK 配置的 tableLocker。
type oracleDialect struct{}

func (oracleDialect) Locker(db *gorm.DB, lockKey string) locker {
	return newTableLocker(db, lockKey, oracleLockTableSQL)
}

func (oracleDialect) IsIdempotentSkippable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// ORA-00955 名称已由现有对象使用 / ORA-01430 表中已存在要添加的列
	for _, code := range []string{"ORA-00955", "ORA-01430"} {
		if strings.Contains(msg, code) {
			return true
		}
	}
	return false
}

func (oracleDialect) MigrationsTableSQL() string { return oracleMigrationsTableSQL }
func (oracleDialect) LockTableSQL() string       { return oracleLockTableSQL }

// ── 达梦（DM） ──

// dmDialect 只支持迁移，不支持自动建库：达梦实例通常由 DBA 用 dminit 预先初始化，
// 业务侵入式建库不是常规运维方式，因此不实现 dbCreator，EnsureDatabase 会提示手动建库。
//
// 达梦没有确认可用的会话级咨询锁原语（DBMS_LOCK 兼容包是否内置取决于实例配置），
// 分布式锁改用不依赖方言特性的 tableLocker（基于表的乐观锁）。
type dmDialect struct{}

func (dmDialect) Locker(db *gorm.DB, lockKey string) locker {
	return newTableLocker(db, lockKey, genericLockTableSQL)
}

func (dmDialect) IsIdempotentSkippable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// -2005 表或视图已存在；达梦错误信息也可能直接包含中英文提示
	for _, kw := range []string{"-2005", "already exists", "已存在"} {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}

func (dmDialect) MigrationsTableSQL() string { return genericMigrationsTableSQL }
func (dmDialect) LockTableSQL() string       { return genericLockTableSQL }

// ── SQLite ──

// sqliteDialect 使用锁表协调共享 SQLite 数据库中的并发迁移。
// 内存数据库仅在单进程内共享，文件数据库也可以复用同一机制。
type sqliteDialect struct{}

func (sqliteDialect) Locker(db *gorm.DB, lockKey string) locker {
	return newTableLocker(db, lockKey, genericLockTableSQL)
}

func (sqliteDialect) IsIdempotentSkippable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, keyword := range []string{"already exists", "duplicate column name"} {
		if strings.Contains(msg, keyword) {
			return true
		}
	}
	return false
}

func (sqliteDialect) MigrationsTableSQL() string { return genericMigrationsTableSQL }
func (sqliteDialect) LockTableSQL() string       { return genericLockTableSQL }
