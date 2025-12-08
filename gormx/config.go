package gormx

import (
	"fmt"
	"strings"
	"time"

	gormLogger "gorm.io/gorm/logger"
)

var defaultConfig = &Config{
	MaxOpenConn:     100,             // 最大连接数
	MaxIdleConn:     10,              // 最大空闲连接数
	ConnMaxLifeTime: 5 * time.Minute, // 连接最大存活时间
}

// Config Database configuration
type Config struct {
	MaxOpenConn     int           `mapstructure:"max_open_conn" json:"max_open_conn"`           // 最大连接数
	MaxIdleConn     int           `mapstructure:"max_idle_conn" json:"max_idle_conn"`           // 最大空闲连接数
	ConnMaxLifeTime time.Duration `mapstructure:"conn_max_life_time" json:"conn_max_life_time"` // 连接最大存活时间
	DBName          string        `mapstructure:"db_name" json:"db_name"`                       // 数据库名称（要求唯一）
	DSN             string        `mapstructure:"dsn" json:"dsn"`                               // 数据库连接串
	LogMode         LogMode       `mapstructure:"log_mode" json:"log_mode"`                     // 日志级别

	// 可以提供以下信息自动拼装DSN
	DBType DatabaseType `mapstructure:"db_type" json:"db_type"`

	Host     string `mapstructure:"host" json:"host"`
	Port     int    `mapstructure:"port" json:"port"`
	User     string `mapstructure:"username" json:"username"`
	Password string `mapstructure:"password" json:"password"`
	Database string `mapstructure:"database" json:"database"`
	Args     []ARG  `mapstructure:"args" json:"args"`
	Logger   gormLogger.Interface
}

type ARG struct {
	Name  string `mapstructure:"name" json:"name"`
	Value string `mapstructure:"value" json:"value"`
}

// GetLogMode _
func (c *Config) GetLogMode() gormLogger.LogLevel {
	switch c.LogMode {
	case LogModeInfo:
		return gormLogger.Info
	case LogModeWarn:
		return gormLogger.Warn
	case LogModeError:
		return gormLogger.Error
	case LogModeSilent:
		return gormLogger.Silent
	}
	return gormLogger.Info
}

func (c *Config) GenDSN() (dsn string) {
	if c.DSN != "" {
		return c.DSN
	}

	switch c.DBType {
	case DatabaseTypeSQLite:
		if c.DSN == "" {
			dsn = "file::memory:?cache=shared"
		}
	default:
		dbName := c.Database
		if dbName == "" {
			dbName = c.DBName
		}
		dsnTemplate, ok := DatabaseDsnMap[c.DBType]
		if !ok {
			return
		}
		dsn = fmt.Sprintf(dsnTemplate, c.User, c.Password, c.Host, c.Port, dbName)
		if len(c.Args) > 0 {
			sep, ok := DatabaseDsnSepMap[c.DBType]
			if !ok {
				return
			}
			prefix, ok := DatabaseDsnPrefixMap[c.DBType]
			if !ok {
				return
			}
			var args []string
			for _, arg := range c.Args {
				args = append(args, fmt.Sprintf("%s=%s", arg.Name, arg.Value))
			}
			dsn += prefix
			dsn += strings.Join(args, sep)
		}
	}
	c.DSN = dsn
	return
}
