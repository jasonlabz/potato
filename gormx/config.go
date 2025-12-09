package gormx

import (
	"fmt"
	"strings"

	gormLogger "gorm.io/gorm/logger"
)

var defaultConfig = &Config{
	MaxOpenConn:     100,                // 最大连接数
	MaxIdleConn:     10,                 // 最大空闲连接数
	ConnMaxLifeTime: 1000 * 60 * 60 * 3, // 连接最大存活时间，单位ms
	ConnMaxIdleTime: 1000 * 60 * 45,     // 连接最大存活时间，单位ms
}

// Config Database configuration
type Config struct {
	MaxOpenConn     int     `mapstructure:"max_open_conn" json:"max_open_conn"`           // 最大连接数
	MaxIdleConn     int     `mapstructure:"max_idle_conn" json:"max_idle_conn"`           // 最大空闲连接数
	ConnMaxLifeTime int64   `mapstructure:"conn_max_life_time" json:"conn_max_life_time"` // 连接最大存活时间
	ConnMaxIdleTime int64   `mapstructure:"conn_max_idle_time" json:"conn_max_idle_time"` // 连接最大空闲时间
	DBName          string  `mapstructure:"db_name" json:"db_name"`                       // 数据库名称（要求唯一）
	LogMode         LogMode `mapstructure:"log_mode" json:"log_mode"`                     // 日志级别

	// 可以提供以下信息自动拼装DSN
	Connection

	Masters  []Connection `mapstructure:"masters" json:"masters" yaml:"masters" ini:"masters"`
	Replicas []Connection `mapstructure:"replicas" json:"replicas" yaml:"replicas" ini:"replicas"`
	Logger   gormLogger.Interface
}

type Connection struct {
	DBType DatabaseType `mapstructure:"db_type" json:"db_type"`
	DSN    string       `mapstructure:"dsn" json:"dsn" yaml:"dsn" ini:"dsn"` // 数据库连接串，为空则读取host、port、username、password、database

	Host     string `mapstructure:"host" json:"host" yaml:"host" ini:"host"`
	Port     int    `mapstructure:"port" json:"port" yaml:"port" ini:"port"`
	Username string `mapstructure:"username" json:"username" yaml:"username" ini:"username"`
	Password string `mapstructure:"password" json:"password" yaml:"password" ini:"password"`
	Database string `mapstructure:"database" json:"database" yaml:"database" ini:"database"`

	Args []ARG `mapstructure:"args" json:"args"`
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

func (c *Connection) genDSN() (dsn string) {
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
		dsnTemplate, ok := DatabaseDsnMap[c.DBType]
		if !ok || c.Host == "" || c.Port == 0 {
			return ""
		}
		dsn = fmt.Sprintf(dsnTemplate, c.Username, c.Password, c.Host, c.Port, dbName)
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
	return dsn
}

func (c *Config) extractDSN() (dsn string, replicas [2][]string) {
	conn := &c.Connection
	dsn = conn.genDSN()
	replicas = [2][]string{}

	if len(c.Masters) == 0 || len(c.Replicas) == 0 {
	}

	// 解析主数据库配置
	if len(c.Masters) > 0 {
		for _, master := range c.Masters {
			if master.DBType == "" {
				master.DBType = c.DBType
			}
			if len(master.Args) == 0 {
				master.Args = c.Args
			}
			if master.genDSN() != "" {
				replicas[0] = append(replicas[0], master.DSN)
			}
		}
	}
	// 解析从数据库配置
	if len(c.Replicas) > 0 {
		for _, replica := range c.Replicas {
			if replica.DBType == "" {
				replica.DBType = c.DBType
			}
			if len(replica.Args) == 0 {
				replica.Args = c.Args
			}
			if replica.genDSN() != "" {
				replicas[1] = append(replicas[1], replica.DSN)
			}
		}
	}

	// 优先取主数据库dsn填充
	if dsn == "" && len(replicas[0]) > 0 {
		dsn = replicas[0][0]
	}
	// 其次取从数据库dsn
	if dsn == "" && len(replicas[1]) > 0 {
		dsn = replicas[1][1]
	}

	return dsn, replicas
}
