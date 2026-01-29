// Package mongo
//
//   _ __ ___   __ _ _ __  _   _| |_
//  | '_ ` _ \ / _` | '_ \| | | | __|
//  | | | | | | (_| | | | | |_| | |_
//  |_| |_| |_|\__,_|_| |_|\__,_|\__|
//
//  Buddha bless, no bugs forever!
//
//  Author:    lucas
//  Email:     1783022886@qq.com
//  Created:   2026/1/29 23:22
//  Version:   v1.0.0

package mongo

import (
	"errors"

	"github.com/jasonlabz/potato/log"
)

type Config struct {
	URI                    string `yaml:"uri" json:"uri"`
	MaxPoolSize            uint64 `yaml:"max_pool_size" json:"max_pool_size"`
	MinPoolSize            uint64 `yaml:"min_pool_size" json:"min_pool_size"`
	MaxConnIdleTime        uint64 `yaml:"max_conn_idle_time" json:"max_conn_idle_time"`
	ConnectTimeout         uint64 `yaml:"connect_timeout" json:"connect_timeout"`
	SocketTimeout          uint64 `yaml:"socket_timeout" json:"socket_timeout"`
	ServerSelectionTimeout uint64 `yaml:"server_selection_timeout" json:"server_selection_timeout"`
	HeartbeatInterval      uint64 `yaml:"heartbeat_interval" json:"heartbeat_interval"`
	ReplicaSet             string `yaml:"replica_set" json:"replica_set"`

	Auth *AuthConfig `yaml:"auth" json:"auth"`

	logger *log.LoggerWrapper
}

type AuthConfig struct {
	Username      string `yaml:"username" json:"username"`
	Password      string `yaml:"password" json:"password"`
	AuthSource    string `yaml:"auth_source" json:"auth_source"`
	AuthMechanism string `yaml:"auth_mechanism" json:"auth_mechanism"`
}

// WithLogger 设置日志器
func (c *Config) WithLogger(logger *log.LoggerWrapper) *Config {
	c.logger = logger
	return c
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.URI == "" {
		return errors.New("URI is required")
	}

	if c.MaxPoolSize < c.MinPoolSize {
		return errors.New("MaxPoolSize must be greater than or equal to MinPoolSize")
	}

	if c.MaxPoolSize == 0 {
		return errors.New("MaxPoolSize must be greater than 0")
	}

	if c.ConnectTimeout <= 0 {
		return errors.New("ConnectTimeout must be greater than 0")
	}

	return nil
}
