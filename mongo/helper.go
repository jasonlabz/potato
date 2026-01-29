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
//  Created:   2026/1/30 1:02
//  Version:   v1.0.0

package mongo

import (
	"time"

	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

// applyDefaultConfig 应用默认配置
func applyDefaultConfig(config *Config) *Config {
	// 创建一个新的配置副本，避免修改原始配置
	result := &Config{
		URI:        config.URI,
		ReplicaSet: config.ReplicaSet,
		Auth:       config.Auth,
		logger:     config.logger,
	}

	// 设置数值型配置，如果为0则使用默认值
	result.MaxPoolSize = setIfZero(config.MaxPoolSize, defaultConfig.MaxPoolSize)
	result.MinPoolSize = setIfZero(config.MinPoolSize, defaultConfig.MinPoolSize)
	result.MaxConnIdleTime = setIfZero(config.MaxConnIdleTime, defaultConfig.MaxConnIdleTime)
	result.ConnectTimeout = setIfZero(config.ConnectTimeout, defaultConfig.ConnectTimeout)
	result.SocketTimeout = setIfZero(config.SocketTimeout, defaultConfig.SocketTimeout)
	result.ServerSelectionTimeout = setIfZero(config.ServerSelectionTimeout, defaultConfig.ServerSelectionTimeout)
	result.HeartbeatInterval = setIfZero(config.HeartbeatInterval, defaultConfig.HeartbeatInterval)

	// 处理认证配置
	if config.Auth != nil {
		if result.Auth == nil {
			result.Auth = &AuthConfig{}
		}
		result.Auth.Username = config.Auth.Username
		result.Auth.Password = config.Auth.Password
		result.Auth.AuthSource = setIfEmpty(config.Auth.AuthSource, "admin")
		result.Auth.AuthMechanism = config.Auth.AuthMechanism
	}

	return result
}

// buildClientOptions 构建客户端选项
func buildClientOptions(config *Config) *options.ClientOptions {
	clientOpts := options.Client().
		ApplyURI(config.URI).
		SetMaxPoolSize(config.MaxPoolSize).
		SetMinPoolSize(config.MinPoolSize).
		SetMaxConnIdleTime(time.Duration(config.MaxConnIdleTime) * time.Second).
		SetConnectTimeout(time.Duration(config.ConnectTimeout) * time.Second).
		SetSocketTimeout(time.Duration(config.SocketTimeout) * time.Second).
		SetServerSelectionTimeout(time.Duration(config.ServerSelectionTimeout) * time.Second).
		SetHeartbeatInterval(time.Duration(config.HeartbeatInterval) * time.Second).
		SetReadPreference(readpref.Primary()).
		SetWriteConcern(writeconcern.Majority()).
		SetRetryWrites(true).
		SetRetryReads(true)

	// 设置副本集
	if config.ReplicaSet != "" {
		clientOpts.SetReplicaSet(config.ReplicaSet)
	}

	// 设置认证信息
	if config.Auth != nil && config.Auth.Username != "" {
		credential := options.Credential{
			Username: config.Auth.Username,
			Password: config.Auth.Password,
		}
		if config.Auth.AuthSource != "" {
			credential.AuthSource = config.Auth.AuthSource
		}
		if config.Auth.AuthMechanism != "" {
			credential.AuthMechanism = config.Auth.AuthMechanism
		}
		clientOpts.SetAuth(credential)
	}

	return clientOpts
}

// ==================== 辅助函数 ====================

// setIfZero 如果值为0则设置默认值
func setIfZero(value, defaultValue uint64) uint64 {
	if value == 0 {
		return defaultValue
	}
	return value
}

// setIfZeroInt 如果值为0则设置默认值（int类型）
func setIfZeroInt(value, defaultValue int) int {
	if value == 0 {
		return defaultValue
	}
	return value
}

// setIfEmpty 如果字符串为空则设置默认值
func setIfEmpty(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

// maskURI 隐藏敏感信息的URI
func maskURI(uri string) string {
	if len(uri) == 0 {
		return ""
	}

	// 简单处理：显示协议和主机部分
	const maxLength = 50
	if len(uri) <= maxLength {
		return uri
	}

	// 查找协议结束位置
	var protocolEnd int
	if idx := len("mongodb://"); idx < len(uri) && uri[:idx] == "mongodb://" {
		protocolEnd = idx
	} else if idx := len("mongodb+srv://"); idx < len(uri) && uri[:idx] == "mongodb+srv://" {
		protocolEnd = idx
	} else {
		return uri[:maxLength] + "..."
	}

	// 查找@符号位置（如果有认证信息）
	atIndex := -1
	for i := protocolEnd; i < len(uri); i++ {
		if uri[i] == '@' {
			atIndex = i
			break
		}
	}

	if atIndex > 0 {
		// 有认证信息，隐藏密码部分
		// mongodb://username:password@host...
		// 显示为：mongodb://username:****@host...
		userPassPart := uri[protocolEnd:atIndex]
		hostPart := uri[atIndex:]

		// 分割用户名和密码
		colonIndex := -1
		for i := 0; i < len(userPassPart); i++ {
			if userPassPart[i] == ':' {
				colonIndex = i
				break
			}
		}

		if colonIndex > 0 {
			// 有密码，隐藏它
			username := userPassPart[:colonIndex]
			maskedURI := uri[:protocolEnd] + username + ":****" + hostPart
			if len(maskedURI) <= maxLength {
				return maskedURI
			}
			return maskedURI[:maxLength] + "..."
		}
	}

	// 没有认证信息或格式异常，简单截断
	return uri[:maxLength] + "..."
}

// ==================== 配置相关 ====================

// SetDefaultConfig 设置全局默认配置
func SetDefaultConfig(config *Config) {
	if config == nil {
		return
	}

	operatorMu.Lock()
	defer operatorMu.Unlock()

	// 只更新非零值
	if config.MaxPoolSize > 0 {
		defaultConfig.MaxPoolSize = config.MaxPoolSize
	}
	if config.MinPoolSize > 0 {
		defaultConfig.MinPoolSize = config.MinPoolSize
	}
	if config.MaxConnIdleTime > 0 {
		defaultConfig.MaxConnIdleTime = config.MaxConnIdleTime
	}
	if config.ConnectTimeout > 0 {
		defaultConfig.ConnectTimeout = config.ConnectTimeout
	}
	if config.SocketTimeout > 0 {
		defaultConfig.SocketTimeout = config.SocketTimeout
	}
	if config.ServerSelectionTimeout > 0 {
		defaultConfig.ServerSelectionTimeout = config.ServerSelectionTimeout
	}
	if config.HeartbeatInterval > 0 {
		defaultConfig.HeartbeatInterval = config.HeartbeatInterval
	}
	if config.ReplicaSet != "" {
		defaultConfig.ReplicaSet = config.ReplicaSet
	}
	if config.Auth != nil {
		defaultConfig.Auth = config.Auth
	}
}

// GetDefaultConfig 获取默认配置
func GetDefaultConfig() *Config {
	operatorMu.RLock()
	defer operatorMu.RUnlock()

	// 返回副本，避免外部修改
	return &Config{
		URI:                    defaultConfig.URI,
		MaxPoolSize:            defaultConfig.MaxPoolSize,
		MinPoolSize:            defaultConfig.MinPoolSize,
		MaxConnIdleTime:        defaultConfig.MaxConnIdleTime,
		ConnectTimeout:         defaultConfig.ConnectTimeout,
		SocketTimeout:          defaultConfig.SocketTimeout,
		ServerSelectionTimeout: defaultConfig.ServerSelectionTimeout,
		HeartbeatInterval:      defaultConfig.HeartbeatInterval,
		ReplicaSet:             defaultConfig.ReplicaSet,
		Auth:                   defaultConfig.Auth,
	}
}
