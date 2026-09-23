// Package migrate 提供数据库结构迁移能力：基线 + 增量版本管理、多实例分布式锁、
// 按方言适配的追踪表/锁表建表语句，以及可选的自动建库（EnsureDatabase）。
//
// 迁移文件约定：
//   - 目录下仅扫描 .sql 文件，按版本号排序执行。
//   - 版本号优先取文件头部的 "-- @version YYYYMMDD_NNN"（也支持 --@version），
//     普通文件缺失时从文件名前缀提取，如 20240701_001_add_email.sql。
//   - 基线文件名以 00000000_000 开头，有且仅有一个，且必须显式声明头部版本。
//
// 执行策略：
//   - 新库 → 执行基线 → 跳过版本 ≤ 基线版本的增量 → 执行剩余增量
//   - 已有库 → 只执行版本 > 最新已应用版本的增量
//   - seed → 在全部 DDL 完成后按同样的版本规则执行，失败仅告警不中断
package migrate

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/jasonlabz/potato/gormx"
	"github.com/jasonlabz/potato/internal/log"
	potatolog "github.com/jasonlabz/potato/log"
)

const (
	// BaselinePrefix 基线文件名前缀，有且仅有一个。
	BaselinePrefix = "00000000_000"
	// DefaultLockKey 默认迁移锁标识，建议各项目通过 Config.LockKey 覆盖为自己的名称。
	DefaultLockKey = "potato:schema-migrations"
	// DefaultLockTimeout 等待迁移锁的默认最长时间。
	DefaultLockTimeout = 60 * time.Second

	baselineName   = BaselinePrefix + "_baseline.sql"
	versionPrefix  = "-- @version " // 推荐版本声明，也支持 --@version
	versionPrefix2 = "--@version "
)

// MigrationType 迁移类型：DDL 结构迁移或 seed 种子数据。
type MigrationType string

const (
	MigrationTypeDDL  MigrationType = "ddl"
	MigrationTypeSeed MigrationType = "seed"
)

// Config 一次迁移执行所需的全部配置。
type Config struct {
	DBType      gormx.DatabaseType // 数据库类型，决定锁实现、幂等判断与建表方言
	DDLDir      string             // DDL 迁移目录，如 conf/migrations
	SeedDir     string             // 种子数据目录，如 conf/seed；为空则跳过 seed
	LockKey     string             // 迁移锁标识；为空使用 DefaultLockKey
	Logger      log.Logger         // 日志；为 nil 使用 potato 默认 logger
	LockTimeout time.Duration      // 等待迁移锁的最长时间；为零使用 DefaultLockTimeout
}

func (c Config) withDefaults() Config {
	if c.LockKey == "" {
		c.LockKey = DefaultLockKey
	}
	if c.LockTimeout <= 0 {
		c.LockTimeout = DefaultLockTimeout
	}
	if c.Logger == nil {
		c.Logger = potatolog.GetLogger()
	}
	return c
}

// ── 数据结构 ──

// migFile 迁移文件元信息。
// 版本优先取头部 -- @version，普通文件缺失时从文件名前缀提取。
// baseline 由文件名是否以 00000000_000 开头决定。
type migFile struct {
	name     string
	path     string
	version  string
	kind     MigrationType
	baseline bool
}

// ── 公开入口 ──

// ErrAutoCreateUnsupported 表示当前数据库类型不支持自动建库，
// 需要 DBA 预先手工创建业务库。
var ErrAutoCreateUnsupported = errors.New("当前数据库类型不支持自动建库")

// EnsureDatabase 检查目标数据库是否存在，不存在则创建。
// 通过 gormx.InitConfig 临时连接管理库，用完 Close，不污染全局连接池。
//
// 返回 created=true 表示本次新建了数据库；库已存在返回 (false, nil)；
// 方言不支持自动建库（如达梦、Oracle）返回 ErrAutoCreateUnsupported；
// SQLite 没有建库概念，直接返回 (false, nil)。
func EnsureDatabase(ctx context.Context, conn gormx.Connection, database string, logger log.Logger) (created bool, err error) {
	if conn.DBType == gormx.DatabaseTypeSQLite {
		return false, nil
	}

	creator, ok := lookupDBCreator(conn.DBType)
	if !ok {
		return false, ErrAutoCreateUnsupported
	}

	// 管理库连接：复用业务连接参数，但库名换成管理库；DSN 指向业务库，必须清空重拼。
	adminConn := conn
	adminConn.Database = creator.AdminDatabase()
	adminConn.DSN = ""
	adminCfg := &gormx.Config{
		DBName:     "__ensure_db__",
		Connection: adminConn,
		LogMode:    gormx.LogModeError,
	}
	if logger != nil {
		adminCfg.Logger = gormx.LoggerAdapter(logger)
	}
	adminDB, err := gormx.InitConfig(adminCfg)
	if err != nil {
		return false, fmt.Errorf("连接服务器失败: %w", err)
	}
	defer func() {
		if closeErr := gormx.Close(adminCfg.DBName); closeErr != nil && logger != nil {
			logger.Errorf(ctx, "[migrate] 关闭管理员数据库连接失败: %v", closeErr)
		}
	}()

	if dbExists(adminDB, creator, database) {
		return false, nil
	}
	if err = adminDB.Exec(creator.CreateDatabaseSQL(database)).Error; err != nil {
		return false, fmt.Errorf("创建数据库失败: %w", err)
	}
	return true, nil
}

// Run 执行 DDL 迁移和种子数据（先 DDL 后 seed）。
//
// DDL 失败返回 error（是否中断启动由调用方决定）；seed 失败仅记录告警并跳过，
// 与既有服务的启动语义保持一致。
func Run(ctx context.Context, db *gorm.DB, cfg Config) error {
	cfg = cfg.withDefaults()
	db = withErrorLogger(db)

	d, registered := lookupDialect(cfg.DBType)
	unlock, err := acquireMigrationLock(ctx, cfg, db, d, registered)
	if err != nil {
		return fmt.Errorf("[migrate] 获取迁移锁失败: %w", err)
	}
	defer unlock()

	tableSQL := genericMigrationsTableSQL
	if registered {
		tableSQL = d.MigrationsTableSQL()
	}
	if err = db.Exec(tableSQL).Error; err != nil {
		return fmt.Errorf("[migrate] 创建追踪表失败: %w", err)
	}

	ddlFiles := loadMigrationFiles(ctx, cfg.Logger, cfg.DDLDir, MigrationTypeDDL)
	var seedFiles []migFile
	if cfg.SeedDir != "" {
		seedFiles = loadMigrationFiles(ctx, cfg.Logger, cfg.SeedDir, MigrationTypeSeed)
	}
	if len(ddlFiles) == 0 && len(seedFiles) == 0 {
		return nil
	}

	if err = runMigrationFiles(ctx, cfg.Logger, db, d, ddlFiles); err != nil {
		return err
	}

	if err = runMigrationFiles(ctx, cfg.Logger, db, d, seedFiles); err != nil {
		cfg.Logger.Warnf(ctx, "[seed] 迁移失败(已跳过): %v", err)
	}
	return nil
}

// ── 文件加载与解析 ──

// loadMigrationFiles 扫描目录、按目录类型解析版本号并排序。
// 文件必须使用 YYYYMMDD_NNN_desc.sql 命名；无法解析版本号的文件会被跳过并告警。
func loadMigrationFiles(ctx context.Context, logger log.Logger, dir string, kind MigrationType) []migFile {
	names := listSQLFiles(dir)
	files := make([]migFile, 0, len(names))

	for _, name := range names {
		if kind == MigrationTypeDDL &&
			strings.HasPrefix(name, BaselinePrefix) &&
			name != baselineName {
			continue
		}

		path := filepath.Join(dir, name)
		ver := resolveVersion(path, name)
		if ver == "" {
			logger.Warnf(ctx, "[migrate] 跳过 %s: 无法解析版本号", name)
			continue
		}
		files = append(files, migFile{
			name:     name,
			path:     path,
			version:  ver,
			kind:     kind,
			baseline: strings.HasPrefix(name, BaselinePrefix),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].version != files[j].version {
			return files[i].version < files[j].version
		}
		return files[i].name < files[j].name
	})
	return files
}

// resolveVersion 解析迁移版本号。DDL 和 seed 使用同一套规则。
func resolveVersion(path, name string) string {
	filenameVersion := extractNameVersion(name)
	if filenameVersion == "" {
		return ""
	}
	if version := parseHeaderVersion(path); isMigrationVersion(version) {
		return version
	}
	if strings.HasPrefix(name, BaselinePrefix) {
		return ""
	}
	return filenameVersion
}

// parseHeaderVersion 读取 SQL 文件前若干行，查找 -- @version xxx 或 --@version xxx。
func parseHeaderVersion(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	// 仅读取头部声明，关闭失败不会影响已提取的值。
	defer func() {
		_ = f.Close()
	}()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if value, ok := cutVersion(line, versionPrefix); ok {
			return strings.TrimSpace(value)
		}
		if value, ok := cutVersion(line, versionPrefix2); ok {
			return strings.TrimSpace(value)
		}
		// 遇到非注释非空行说明头部结束
		if line != "" && !strings.HasPrefix(line, "--") {
			break
		}
	}
	return ""
}

func cutVersion(line, prefix string) (string, bool) {
	if strings.HasPrefix(line, prefix) {
		return line[len(prefix):], true
	}
	return "", false
}

// extractNameVersion 从标准文件名提取版本号 YYYYMMDD_NNN。
// 例如 "20240701_001_add_email.sql" → "20240701_001"。
func extractNameVersion(name string) string {
	if !strings.HasSuffix(name, ".sql") {
		return ""
	}
	base := strings.TrimSuffix(name, ".sql")
	parts := strings.SplitN(base, "_", 3)
	if len(parts) == 3 &&
		len(parts[0]) == 8 &&
		len(parts[1]) == 3 &&
		parts[2] != "" &&
		isDigits(parts[0]) &&
		isDigits(parts[1]) {
		return parts[0] + "_" + parts[1]
	}
	return ""
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isMigrationVersion(value string) bool {
	parts := strings.Split(value, "_")
	return len(parts) == 2 &&
		len(parts[0]) == 8 &&
		len(parts[1]) == 3 &&
		isDigits(parts[0]) &&
		isDigits(parts[1])
}

// listSQLFiles 返回目录下所有 .sql 文件名（不含路径），按名称排序。
func listSQLFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// ── 迁移执行 ──

func runMigrationFiles(ctx context.Context, logger log.Logger, db *gorm.DB, d dialect, files []migFile) error {
	if len(files) == 0 {
		return nil
	}

	kind := files[0].kind
	var baseline *migFile
	for i := range files {
		if !files[i].baseline {
			continue
		}
		if baseline != nil {
			return fmt.Errorf("[%s] 存在多个 baseline 文件", kind)
		}
		baseline = &files[i]
	}

	latest, err := latestVersion(db, kind)
	if err != nil {
		return fmt.Errorf("[%s] 查询最新版本失败: %w", kind, err)
	}

	if latest == "" {
		if baseline == nil {
			return fmt.Errorf("[migrate] 缺少基线文件（文件名需以 %s 开头）", BaselinePrefix)
		}
		logger.Infof(ctx, "[migrate] 执行基线 %s (版本 %s)", baseline.name, baseline.version)
		if err := execFile(db, d, logger, baseline); err != nil {
			return fmt.Errorf("[migrate] 基线失败: %w", err)
		}
		latest = baseline.version
	}

	for i := range files {
		mf := &files[i]
		if mf.kind != kind || mf.baseline || mf.version <= latest {
			continue
		}
		done, err := isApplied(db, mf.version, mf.kind)
		if err != nil {
			return fmt.Errorf("[migrate] 查询状态失败 %s: %w", mf.name, err)
		}
		if done {
			continue
		}
		logger.Infof(ctx, "[migrate] 执行 %s (版本 %s)", mf.name, mf.version)
		if err := execFile(db, d, logger, mf); err != nil {
			return fmt.Errorf("[%s] 迁移失败 %s: %w", mf.kind, mf.name, err)
		}
		latest = mf.version
	}
	return nil
}

// execFile 在事务中执行迁移文件并记录版本号。
//
// 分布式锁已保证同一时刻只有一个实例执行迁移；这里的幂等兜底只覆盖锁保护之外的场景
// （例如历史遗留、手工误操作导致的结构已存在），命中"对象已存在"类错误时记录警告后
// 视为已应用，其余错误仍然中断迁移。
func execFile(db *gorm.DB, d dialect, logger log.Logger, mf *migFile) error {
	content, err := os.ReadFile(mf.path)
	if err != nil {
		return fmt.Errorf("读取文件: %w", err)
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(string(content)).Error; err != nil {
			if mf.kind == MigrationTypeSeed || d == nil || !d.IsIdempotentSkippable(err) {
				return fmt.Errorf("执行SQL: %w", err)
			}
			logger.Warnf(context.Background(),
				"[migrate] %s 执行报重复对象错误，视为已应用: %v", mf.name, err)
		}
		if err := tx.Exec(
			`INSERT INTO schema_migrations (version, type) VALUES (?, ?)`,
			mf.version, mf.kind,
		).Error; err != nil {
			return fmt.Errorf("记录版本: %w", err)
		}
		return nil
	})
}

// ── schema_migrations 查询 ──

func latestVersion(db *gorm.DB, kind MigrationType) (string, error) {
	var v string
	err := db.Raw(
		`SELECT COALESCE(MAX(version), '') FROM schema_migrations WHERE type = ?`,
		kind,
	).Scan(&v).Error
	return v, err
}

func isApplied(db *gorm.DB, version string, kind MigrationType) (bool, error) {
	var n int64
	err := db.Raw(
		`SELECT COUNT(1) FROM schema_migrations WHERE version = ? AND type = ?`,
		version, kind,
	).Scan(&n).Error
	return n > 0, err
}

func withErrorLogger(db *gorm.DB) *gorm.DB {
	return db.Session(&gorm.Session{Logger: db.Logger.LogMode(logger.Error)})
}

// ── EnsureDatabase 辅助 ──

// dbExists 通过 GORM 查询目标数据库是否存在。
func dbExists(db *gorm.DB, creator dbCreator, dbName string) bool {
	q := creator.DBExistsQuery()
	if q == "" {
		return false
	}
	var n int
	if err := db.Raw(q, dbName).Scan(&n).Error; err != nil {
		return false
	}
	return n > 0
}
