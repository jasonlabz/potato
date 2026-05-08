package grpcx

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/jasonlabz/potato/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

const (
	DefaultTimeout     = 10000 // 默认超时 10s
	DefaultRetryCount  = 3
	DefaultRetryWaitMs = 1000
	DefaultMaxRecvSize = 4 * 1024 * 1024 // 4MB
	DefaultMaxSendSize = 4 * 1024 * 1024 // 4MB
)

// ClientConn 封装 grpc.ClientConn，提供连接管理和日志能力
type ClientConn struct {
	conn   *grpc.ClientConn
	config *Config
	logger *log.LoggerWrapper
}

// newClientConn 根据 Config 创建 gRPC 客户端连接
func newClientConn(cfg *Config) (*ClientConn, error) {
	if !cfg.commonSet {
		cfg.Validate()
	}

	// 设置默认值
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeout
	}
	if cfg.MaxRecvSize <= 0 {
		cfg.MaxRecvSize = DefaultMaxRecvSize
	}
	if cfg.MaxSendSize <= 0 {
		cfg.MaxSendSize = DefaultMaxSendSize
	}

	dialOpts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(cfg.MaxRecvSize),
			grpc.MaxCallSendMsgSize(cfg.MaxSendSize),
		),
	}

	// 传输凭证
	switch cfg.Protocol {
	case "grpcs":
		tlsCfg := &tls.Config{
			InsecureSkipVerify: cfg.InsecureSkipVerify,
		}
		// 加载根证书
		if cfg.RootCertFile != "" {
			caCert, err := os.ReadFile(cfg.RootCertFile)
			if err != nil {
				return nil, fmt.Errorf("grpcx: failed to read root cert: %w", err)
			}
			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM(caCert) {
				return nil, fmt.Errorf("grpcx: failed to append root cert")
			}
			tlsCfg.RootCAs = caCertPool
		}
		// 加载客户端证书（mTLS）
		if cfg.CertFile != "" && cfg.KeyFile != "" {
			cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
			if err != nil {
				return nil, fmt.Errorf("grpcx: failed to load client cert: %w", err)
			}
			tlsCfg.Certificates = []tls.Certificate{cert}
		}
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	default:
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Keepalive
	if cfg.KeepaliveTime > 0 || cfg.KeepaliveTimeout > 0 {
		kaTime := cfg.KeepaliveTime
		if kaTime <= 0 {
			kaTime = 30 * time.Second
		}
		kaTimeout := cfg.KeepaliveTimeout
		if kaTimeout <= 0 {
			kaTimeout = 10 * time.Second
		}
		dialOpts = append(dialOpts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                kaTime,
			Timeout:             kaTimeout,
			PermitWithoutStream: true,
		}))
	}

	// 拦截器
	var unaryInterceptors []grpc.UnaryClientInterceptor
	var streamInterceptors []grpc.StreamClientInterceptor

	if cfg.Debug {
		unaryInterceptors = append(unaryInterceptors, unaryLoggingInterceptor(cfg.logger))
		streamInterceptors = append(streamInterceptors, streamLoggingInterceptor(cfg.logger))
	}

	// Recovery 拦截器
	unaryInterceptors = append(unaryInterceptors, unaryRecoveryInterceptor(cfg.logger))
	streamInterceptors = append(streamInterceptors, streamRecoveryInterceptor(cfg.logger))

	if len(unaryInterceptors) > 0 {
		dialOpts = append(dialOpts, grpc.WithChainUnaryInterceptor(unaryInterceptors...))
	}
	if len(streamInterceptors) > 0 {
		dialOpts = append(dialOpts, grpc.WithChainStreamInterceptor(streamInterceptors...))
	}

	// 阻塞连接（非懒连接）
	dialOpts = append(dialOpts, grpc.WithBlock())

	// 连接超时
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Timeout)*time.Millisecond)
	defer cancel()

	conn, err := grpc.DialContext(ctx, cfg.GetTarget(), dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("grpcx: failed to connect to %s: %w", cfg.GetTarget(), err)
	}

	return &ClientConn{
		conn:   conn,
		config: cfg,
		logger: cfg.logger,
	}, nil
}

// Conn 返回底层 grpc.ClientConn，用于创建服务 stub
//
// 用法：
//
//	conn := grpcx.GetServiceConn("user_service")
//	client := pb.NewUserServiceClient(conn.Conn())
//	resp, err := client.GetUser(ctx, &pb.GetUserReq{...})
func (c *ClientConn) Conn() *grpc.ClientConn {
	return c.conn
}

// Close 关闭连接
func (c *ClientConn) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// State 返回连接状态
func (c *ClientConn) State() string {
	if c.conn == nil {
		return "IDLE"
	}
	return c.conn.GetState().String()
}

// Target 返回连接目标地址
func (c *ClientConn) Target() string {
	if c.conn == nil {
		return c.config.GetTarget()
	}
	return c.conn.Target()
}
