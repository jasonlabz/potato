package grpcx

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/jasonlabz/potato/log"
)

var (
	connMap     sync.Map // service name -> *Config
	connInstMap sync.Map // service name -> *grpc.ClientConn (lazy initialized)
	dupMap      = map[string]struct{}{}
)

// Config gRPC 客户端配置，对应 conf/servicer/*.yaml 中的服务配置
//
// 当 Protocol 为 "grpc" 或 "grpcs" 时，initServicer 会将配置注册到 grpcx
type Config struct {
	Name          string
	Debug         bool
	RetryCount    int
	RetryWaitTime int64  // 毫秒
	Timeout       int64  // 毫秒
	Protocol      string // "grpc" 或 "grpcs"

	Host string
	Port int

	// TLS 配置（Protocol 为 "grpcs" 时生效）
	CertFile           string
	KeyFile            string
	RootCertFile       string
	InsecureSkipVerify bool

	// gRPC 专用配置
	MaxRecvSize       int           // 最大接收消息大小（字节），默认 4MB
	MaxSendSize       int           // 最大发送消息大小（字节），默认 4MB
	InitialWindowSize int32         // 初始窗口大小
	InitialConnWindow int32         // 初始连接窗口大小
	KeepaliveTime     time.Duration // Keepalive 探测间隔
	KeepaliveTimeout  time.Duration // Keepalive 超时

	commonSet bool
	logger    *log.LoggerWrapper
}

// Validate 校验配置
func (c *Config) Validate() {
	if c.Name == "" {
		panic("grpcx: Config must have Name, Host: " + c.Host + ":" + strconv.Itoa(c.Port))
	}
	if _, ok := dupMap[c.Name]; ok {
		panic("grpcx: service name duplicate: " + c.Name)
	}
	dupMap[c.Name] = struct{}{}

	if c.Protocol != "grpc" && c.Protocol != "grpcs" {
		panic("grpcx: Protocol must be \"grpc\" or \"grpcs\", service: " + c.Name)
	}

	if c.logger == nil {
		c.logger = log.GetLogger()
	}
}

// GetTarget 返回 gRPC 连接目标地址
func (c *Config) GetTarget() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// Store 注册 gRPC 服务配置
func Store(service string, value *Config) {
	connMap.Store(service, value)
}

// Load 获取 gRPC 服务配置
func Load(service string) *Config {
	value, ok := connMap.Load(service)
	if ok {
		return value.(*Config)
	}
	return nil
}

// GetServiceConn 通过 service name 获取对应的 gRPC 客户端连接（懒初始化 + 缓存）
//
// 配置由 initServicer 从 conf/servicer/*.yaml 加载并通过 Store 存入
func GetServiceConn(service string) *ClientConn {
	// 快速路径：已初始化的连接直接返回
	if inst, ok := connInstMap.Load(service); ok {
		return inst.(*ClientConn)
	}

	// 加载配置
	cfg := Load(service)
	if cfg == nil {
		panic("grpcx: service not found: " + service)
	}

	// 创建新连接
	cfg.commonSet = true
	conn, err := newClientConn(cfg)
	if err != nil {
		panic("grpcx: failed to create connection for " + service + ": " + err.Error())
	}

	actual, loaded := connInstMap.LoadOrStore(service, conn)
	if loaded {
		// 另一个 goroutine 已经创建了连接，关闭当前并使用已存在的
		conn.Close()
		return actual.(*ClientConn)
	}
	return conn
}
