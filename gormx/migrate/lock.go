package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"os"
	"time"

	"gorm.io/gorm"
)

// locker 迁移分布式锁的最小接口。多实例同时启动时，只有拿到锁的实例才能真正执行迁移，
// 其余实例阻塞等待直至超时，避免同一份 DDL 被并发执行。
type locker interface {
	Lock(ctx context.Context) error
	Unlock(ctx context.Context) error
}

// lockDialect 描述某个方言的加锁/解锁 SQL，由 advisoryLocker 统一驱动。
// 加锁与解锁必须发生在同一条物理连接上（同一 session），否则会话级锁（如 pg_advisory_lock、
// MySQL GET_LOCK、sp_getapplock）会失效或释放到错误的会话。
type lockDialect interface {
	LockSQL() string
	UnlockSQL() string
}

// advisoryLocker 通过独占一条 *sql.Conn 来保证 lock/unlock 落在同一会话，
// 这是 golang-migrate 内部 postgres/mysql driver 处理会话级锁的核心做法。
type advisoryLocker struct {
	db      *gorm.DB
	lockKey string
	dialect lockDialect
	conn    *sql.Conn
}

func newAdvisoryLocker(db *gorm.DB, lockKey string, dialect lockDialect) *advisoryLocker {
	return &advisoryLocker{db: db, lockKey: lockKey, dialect: dialect}
}

// lockKeyInt64 把任意字符串 key 映射为稳定的 int64，供 pg_advisory_lock / GET_LOCK 等使用。
func lockKeyInt64(key string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return int64(h.Sum64())
}

func (l *advisoryLocker) Lock(ctx context.Context) error {
	sqlDB, err := l.db.DB()
	if err != nil {
		return fmt.Errorf("获取底层连接失败: %w", err)
	}
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("独占连接失败: %w", err)
	}
	if _, err := conn.ExecContext(ctx, l.dialect.LockSQL(), lockKeyInt64(l.lockKey)); err != nil {
		_ = conn.Close()
		return fmt.Errorf("加锁失败: %w", err)
	}
	l.conn = conn
	return nil
}

func (l *advisoryLocker) Unlock(ctx context.Context) error {
	if l.conn == nil {
		return nil
	}
	defer func() {
		_ = l.conn.Close()
		l.conn = nil
	}()
	if _, err := l.conn.ExecContext(ctx, l.dialect.UnlockSQL(), lockKeyInt64(l.lockKey)); err != nil {
		return fmt.Errorf("解锁失败: %w", err)
	}
	return nil
}

// ── 各方言的加锁/解锁 SQL ──

// postgresLockDialect 使用会话级 advisory lock：pg_advisory_lock 会一直阻塞直至拿到锁，
// 连接关闭时数据库也会自动释放，作为兜底防止 Unlock 未被调用时锁泄漏。
type postgresLockDialect struct{}

func (postgresLockDialect) LockSQL() string   { return `SELECT pg_advisory_lock($1)` }
func (postgresLockDialect) UnlockSQL() string { return `SELECT pg_advisory_unlock($1)` }

// mysqlLockDialect 使用 GET_LOCK，超时时间给一个较大值（秒），配合外层 ctx 控制整体等待时间。
// 会话断开时锁会自动释放。
type mysqlLockDialect struct{}

func (mysqlLockDialect) LockSQL() string   { return `SELECT GET_LOCK(?, 55)` }
func (mysqlLockDialect) UnlockSQL() string { return `SELECT RELEASE_LOCK(?)` }

// sqlserverLockDialect 使用 sp_getapplock，@LockMode='Exclusive'、@LockOwner='Session'，
// 会话断开时自动释放。
type sqlserverLockDialect struct{}

func (sqlserverLockDialect) LockSQL() string {
	return `EXEC sp_getapplock @Resource = @p1, @LockMode = 'Exclusive', @LockOwner = 'Session', @LockTimeout = 55000`
}

func (sqlserverLockDialect) UnlockSQL() string {
	return `EXEC sp_releaseapplock @Resource = @p1, @LockOwner = 'Session'`
}

// ── 基于表的乐观锁（无原生咨询锁原语的方言兜底） ──

// staleLockThreshold 锁记录超过该时长未释放，视为持有者已崩溃，允许被强占。
// 远超正常迁移执行耗时，避免误抢占仍在运行的迁移。
const staleLockThreshold = 5 * time.Minute

// tableLockRetryInterval 抢占失败（锁未超过陈旧阈值）时的重试间隔。
const tableLockRetryInterval = 2 * time.Second

// tableLocker 用一张锁表 + 主键唯一约束实现跨实例互斥，不依赖任何数据库特有的
// 咨询锁原语（如 pg_advisory_lock、GET_LOCK），只要求关系型数据库最基本的主键约束能力。
// 用于达梦、Oracle 等未确认有可用会话锁原语的方言，理论上也可作为任意新方言的默认兜底。
type tableLocker struct {
	db       *gorm.DB
	lockKey  string
	tableSQL string
	lockedBy string
}

func newTableLocker(db *gorm.DB, lockKey, tableSQL string) *tableLocker {
	return &tableLocker{
		db:       db,
		lockKey:  lockKey,
		tableSQL: tableSQL,
		lockedBy: fmt.Sprintf("%s:%d", tableLockerHostname(), os.Getpid()),
	}
}

func tableLockerHostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "unknown"
	}
	return h
}

// Lock 循环尝试抢占锁记录，直至成功或 ctx 超时。
func (l *tableLocker) Lock(ctx context.Context) error {
	if err := l.db.WithContext(ctx).Exec(l.tableSQL).Error; err != nil {
		return fmt.Errorf("创建锁表失败: %w", err)
	}

	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("等待锁超时: %w", err)
		}

		now := time.Now()
		insertErr := l.db.WithContext(ctx).Exec(
			`INSERT INTO schema_migrations_lock (lock_key, locked_by, locked_at) VALUES (?, ?, ?)`,
			l.lockKey, l.lockedBy, now,
		).Error
		if insertErr == nil {
			return nil
		}

		var lockedAt time.Time
		queryErr := l.db.WithContext(ctx).Raw(
			`SELECT locked_at FROM schema_migrations_lock WHERE lock_key = ?`, l.lockKey,
		).Scan(&lockedAt).Error
		if queryErr == nil && !lockedAt.IsZero() && time.Since(lockedAt) > staleLockThreshold {
			// 陈旧锁：带上原 locked_at 做条件删除，避免与另一个同时判断陈旧的实例重复抢占。
			l.db.WithContext(ctx).Exec(
				`DELETE FROM schema_migrations_lock WHERE lock_key = ? AND locked_at = ?`,
				l.lockKey, lockedAt,
			)
			continue
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("等待锁超时: %w", ctx.Err())
		case <-time.After(tableLockRetryInterval):
		}
	}
}

// Unlock 只删除自己持有的那条锁记录，避免误删已被其他实例抢占的新锁。
func (l *tableLocker) Unlock(ctx context.Context) error {
	return l.db.WithContext(ctx).Exec(
		`DELETE FROM schema_migrations_lock WHERE lock_key = ? AND locked_by = ?`,
		l.lockKey, l.lockedBy,
	).Error
}

// ── 封装入口 ──

// acquireMigrationLock 尝试获取迁移分布式锁。
//
// 返回的 unlock 函数总是非 nil，可安全 defer 调用；若当前数据库类型未注册方言，
// unlock 为空操作，并在日志中提示多实例部署时需自行保证串行执行。
func acquireMigrationLock(ctx context.Context, cfg Config, db *gorm.DB, d dialect, registered bool) (unlock func(), err error) {
	if !registered {
		cfg.Logger.Warnf(ctx, "[migrate] 数据库类型 %s 未注册迁移方言，多实例部署时请确保串行执行迁移", cfg.DBType)
		return func() {}, nil
	}
	lockerImpl := d.Locker(db, cfg.LockKey)

	lockCtx, cancel := context.WithTimeout(ctx, cfg.LockTimeout)
	defer cancel()
	if err := lockerImpl.Lock(lockCtx); err != nil {
		return func() {}, fmt.Errorf("等待迁移锁超时或失败: %w", err)
	}

	unlock = func() {
		unlockCtx, unlockCancel := context.WithTimeout(context.Background(), cfg.LockTimeout)
		defer unlockCancel()
		if err := lockerImpl.Unlock(unlockCtx); err != nil {
			cfg.Logger.Errorf(ctx, "[migrate] 释放迁移锁失败: %v", err)
		}
	}
	return unlock, nil
}
