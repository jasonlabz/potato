package configx

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/bytedance/sonic/decoder"
	"github.com/spf13/viper"
	"gopkg.in/ini.v1"
	"gopkg.in/yaml.v3"

	"github.com/jasonlabz/potato/configx/file"
	"github.com/jasonlabz/potato/utils"
)

var confPaths = []string{"./conf/application.yaml", "./conf/server.yaml", "./conf/config.yaml",
	"./conf/application.ini", "./conf/server.ini", "./conf/config.ini", "./conf/application.ini",
	"./conf/server.ini", "./conf/config.ini"}

type CryptoType string

const (
	CryptoTypeAES  CryptoType = "aes"
	CryptoTypeDES  CryptoType = "des"
	CryptoTypeHMAC CryptoType = "hmac"
)

// CryptoConfig 加密配置
type CryptoConfig struct {
	Type string `mapstructure:"type" json:"type" ini:"type" yaml:"type"`
	Key  string `mapstructure:"key" json:"key" ini:"key" yaml:"key"`
}

// KafkaConfig 配置
type KafkaConfig struct {
	Topic            []string `mapstructure:"topic" json:"topic" yaml:"topic" ini:"topic"`
	Strict           bool     `mapstructure:"strict" json:"strict" yaml:"strict" ini:"strict"`
	GroupId          string   `mapstructure:"group_id" json:"group_id" yaml:"group_id" ini:"group_id"`
	BootstrapServers string   `mapstructure:"bootstrap_servers" json:"bootstrap_servers" yaml:"bootstrap_servers" ini:"bootstrap_servers"`
	SecurityProtocol string   `mapstructure:"security_protocol" json:"security_protocol" yaml:"security_protocol" ini:"security_protocol"`
	SaslMechanism    string   `mapstructure:"sasl_mechanism" json:"sasl_mechanism" yaml:"sasl_mechanism" ini:"sasl_mechanism"`
	SaslUsername     string   `mapstructure:"sasl_username" json:"sasl_username" yaml:"sasl_username" ini:"sasl_username"`
	SaslPassword     string   `mapstructure:"sasl_password" json:"sasl_password" yaml:"sasl_password" ini:"sasl_password"`
}

// Database 连接配置
type Database struct {
	Enable  bool   `mapstructure:"enable" json:"enable" yaml:"enable" ini:"enable"`
	Strict  bool   `mapstructure:"strict" json:"strict" yaml:"strict" ini:"strict"`
	DBType  string `mapstructure:"db_type" json:"db_type" yaml:"db_type" ini:"db_type"`
	DSN     string `mapstructure:"dsn" json:"dsn" yaml:"dsn" ini:"dsn"`
	LogMode string `mapstructure:"log_mode" json:"log_mode" yaml:"log_mode" ini:"log_mode"`

	Host            string `mapstructure:"host" json:"host" yaml:"host" ini:"host"`
	Port            int    `mapstructure:"port" json:"port" yaml:"port" ini:"port"`
	Username        string `mapstructure:"username" json:"username" yaml:"username" ini:"username"`
	Password        string `mapstructure:"password" json:"password" yaml:"password" ini:"password"`
	Database        string `mapstructure:"database" json:"database" yaml:"database" ini:"database"`
	Args            []ARG  `mapstructure:"args" json:"args" yaml:"args" ini:"args"`
	Charset         string `mapstructure:"charset" json:"charset" yaml:"charset" ini:"charset"`
	MaxIdleConn     int    `mapstructure:"max_idle_conn" json:"max_idle_conn" yaml:"max_idle_conn" ini:"max_idle_conn"`
	MaxOpenConn     int    `mapstructure:"max_open_conn" json:"max_open_conn" yaml:"max_open_conn" ini:"max_open_conn"`
	ConnMaxLifeTime int64  `mapstructure:"conn_max_life_time" json:"conn_max_life_time" yaml:"conn_max_life_time" ini:"conn_max_life_time"`
}

type ARG struct {
	Name  string `mapstructure:"name" json:"name" yaml:"name" ini:"name"`
	Value string `mapstructure:"value" json:"value" yaml:"value" ini:"value"`
}

// RedisConfig 连接配置
type RedisConfig struct {
	Enable           bool     `mapstructure:"enable" json:"enable" yaml:"enable" ini:"enable"`
	Strict           bool     `mapstructure:"strict" json:"strict" yaml:"strict" ini:"strict"`
	Endpoints        []string `mapstructure:"endpoints" json:"endpoints" yaml:"endpoints" ini:"endpoints"`
	Username         string   `mapstructure:"username" json:"username" yaml:"username" ini:"username"`
	Password         string   `mapstructure:"password" json:"password" yaml:"password" ini:"password"`
	ClientName       string   `mapstructure:"client_name" json:"client_name" yaml:"client_name" ini:"client_name"` // 自定义客户端名
	MasterName       string   `mapstructure:"master_name" json:"master_name" yaml:"master_name" ini:"master_name"` // 主节点
	IndexDB          int      `mapstructure:"index_db" json:"index_db" yaml:"index_db" ini:"index_db"`
	MinIdleConns     int      `mapstructure:"min_idle_conns" json:"min_idle_conns" yaml:"min_idle_conns" ini:"min_idle_conns"`
	MaxIdleConns     int      `mapstructure:"max_idle_conns" json:"max_idle_conns" yaml:"max_idle_conns" ini:"max_idle_conns"`
	MaxActiveConns   int      `mapstructure:"max_active_conns" json:"max_active_conns" yaml:"max_active_conns" ini:"max_active_conns"`
	MaxRetryTimes    int      `mapstructure:"max_retry_times" json:"max_retry_times" yaml:"max_retry_times" ini:"max_retry_times"`
	SentinelUsername string   `mapstructure:"sentinel_username" json:"sentinel_username" yaml:"sentinel_username" ini:"sentinel_username"`
	SentinelPassword string   `mapstructure:"sentinel_password" json:"sentinel_password" yaml:"sentinel_password" ini:"sentinel_password"`
}

type Elasticsearch struct {
	Enable             bool     `mapstructure:"enable" json:"enable" yaml:"enable" ini:"enable"`
	Strict             bool     `mapstructure:"strict" json:"strict" yaml:"strict" ini:"strict"`
	Endpoints          []string `mapstructure:"endpoints" json:"endpoints" yaml:"endpoints" ini:"endpoints"`
	Username           string   `mapstructure:"username" json:"username" yaml:"username" ini:"username"`
	Password           string   `mapstructure:"password" json:"password" yaml:"password" ini:"password"`
	IsHttps            bool     `mapstructure:"is_https" json:"is_https" yaml:"is_https" ini:"is_https"`
	CloudId            string   `mapstructure:"cloud_id" json:"cloud_id" yaml:"cloud_id" ini:"cloud_id"`
	APIKey             string   `mapstructure:"api_key" json:"api_key" yaml:"api_key" ini:"api_key"`
	CACert             string   `mapstructure:"ca_cert" json:"ca_cert" yaml:"ca_cert" ini:"ca_cert"`                                                                          // 客户端证书, 例如："certs/client.pem"
	InsecureSkipVerify bool     `mapstructure:"insecure_skip_verify" json:"insecure_skip_verify" yaml:"insecure_skip_verify" ini:"insecure_skip_verifyinsecure_skip_verifyv"` // 跳过证书认证，生产应为false
}

type MongodbConf struct {
	Enable          bool   `mapstructure:"enable" json:"enable" yaml:"enable" ini:"enable"`
	Strict          bool   `mapstructure:"strict" json:"strict" yaml:"strict" ini:"strict"`
	Host            string `mapstructure:"host" json:"host" yaml:"host" ini:"host"`
	Port            int    `mapstructure:"port" json:"port" yaml:"port" ini:"port"`
	Username        string `mapstructure:"username" json:"username" yaml:"username" ini:"username"`
	Password        string `mapstructure:"password" json:"password" yaml:"password" ini:"password"`
	MaxPoolSize     int    `mapstructure:"max_pool_size" json:"max_pool_size" yaml:"max_pool_size" ini:"max_pool_size"`
	ConnectTimeout  int    `mapstructure:"connect_timeout" json:"connect_timeout" yaml:"connect_timeout" ini:"connect_timeout"`
	MaxConnIdleTime int    `mapstructure:"max_conn_idle_time" json:"max_conn_idle_time" yaml:"max_conn_idle_time" ini:"max_conn_idle_time"`
}

type RabbitMQConf struct {
	Enable    bool      `mapstructure:"enable" json:"enable" yaml:"enable" ini:"enable"`
	Strict    bool      `mapstructure:"strict" json:"strict" yaml:"strict" ini:"strict"`
	Host      string    `mapstructure:"host" json:"host" yaml:"host" ini:"host"`
	Port      int       `mapstructure:"port" json:"port" yaml:"port" ini:"port"`
	Username  string    `mapstructure:"username" json:"username" yaml:"username" ini:"username"`
	Password  string    `mapstructure:"password" json:"password" yaml:"password" ini:"password"`
	LimitConf LimitConf `mapstructure:"limit_conf" json:"limit_conf" yaml:"limit_conf" ini:"limit_conf"`
}

type LimitConf struct {
	Enable        bool  `mapstructure:"enable" json:"enable" yaml:"enable" ini:"enable"`
	AttemptTimes  int   `mapstructure:"attempt_times" json:"attempt_times" yaml:"attempt_times" ini:"attempt_times"`
	RetryWaitTime int64 `mapstructure:"retry_wait_time" json:"retry_wait_time" yaml:"retry_wait_time" ini:"retry_wait_time"`
	PrefetchCount int   `mapstructure:"prefetch_count" json:"prefetch_count" yaml:"prefetch_count" ini:"prefetch_count"`
	Timeout       int64 `mapstructure:"timeout" json:"timeout" yaml:"timeout" ini:"timeout"`
	QueueLimit    int   `mapstructure:"queue_limit" json:"queue_limit" yaml:"queue_limit" ini:"queue_limit"`
}

// ServerConfig 新增的配置结构体
type ServerConfig struct {
	HTTP HTTPConfig `mapstructure:"http" json:"http" yaml:"http" ini:"http"`
	GRPC GRPCConfig `mapstructure:"grpc" json:"grpc" yaml:"grpc" ini:"grpc"`
}

type HTTPConfig struct {
	Port         int    `mapstructure:"port" json:"port" yaml:"port" ini:"port"`
	ReadTimeout  string `mapstructure:"read_timeout" json:"read_timeout" yaml:"read_timeout" ini:"read_timeout"`
	WriteTimeout string `mapstructure:"write_timeout" json:"write_timeout" yaml:"write_timeout" ini:"write_timeout"`
}

type GRPCConfig struct {
	Port                 int    `mapstructure:"port" json:"port" yaml:"port" ini:"port"`
	MaxConcurrentStreams uint32 `mapstructure:"max_concurrent_streams" json:"max_concurrent_streams" yaml:"max_concurrent_streams" ini:"max_concurrent_streams"`
}

type MonitorConfig struct {
	Prometheus PrometheusConfig `mapstructure:"prometheus" json:"prometheus" yaml:"prometheus" ini:"prometheus"`
	PProf      PProfConfig      `mapstructure:"pprof" json:"pprof" yaml:"pprof" ini:"pprof"`
}

type PrometheusConfig struct {
	Enable         bool   `mapstructure:"enable" json:"enable" yaml:"enable" ini:"enable"`
	Path           string `mapstructure:"path" json:"path" yaml:"path" ini:"path"`
	ScrapeInterval string `mapstructure:"scrape_interval" json:"scrape_interval" yaml:"scrape_interval" ini:"scrape_interval"`
}

type PProfConfig struct {
	Enable           bool     `mapstructure:"enable" json:"enable" yaml:"enable" ini:"enable"`
	Port             int      `mapstructure:"port" json:"port" yaml:"port" ini:"port"`
	EnabledEndpoints []string `mapstructure:"enabled_endpoints" json:"enabled_endpoints" yaml:"enabled_endpoints" ini:"enabled_endpoints"`
	Pusher           Pusher   `mapstructure:"pusher" json:"pusher" yaml:"pusher" ini:"pusher"` // 推送到 pushgateway
}

// Pusher push to pushGateway 配置
type Pusher struct {
	Enable     bool   `mapstructure:"enable" json:"enable" yaml:"enable" ini:"enable"`                     // Enable backend job push metrics to remote pushgateway
	JobName    string `mapstructure:"job_name" json:"job_name" yaml:"job_name" ini:"job_name"`             // Name of current push job
	RemoteAddr string `mapstructure:"remote_addr" json:"remote_addr" yaml:"remote_addr" ini:"remote_addr"` // Remote address of pushgateway
	IntervalMs int    `mapstructure:"IntervalMs" json:"IntervalMs" yaml:"interval_ms" ini:"interval_ms"`   // Push interval in milliseconds
	BasicAuth  string `mapstructure:"basic_auth" json:"basic_auth" yaml:"basic_auth" ini:"basic_auth"`     // Basic auth of pushgateway
}

// Application 配置
type Application struct {
	Host       string        `mapstructure:"host" json:"host" yaml:"host" ini:"host"`
	Name       string        `mapstructure:"name" json:"name" yaml:"name" ini:"name"`
	Port       int           `mapstructure:"port" json:"port" yaml:"port" ini:"port"`
	Debug      bool          `mapstructure:"debug" json:"debug" yaml:"debug" ini:"debug"`
	FileServer bool          `mapstructure:"file_server" json:"file_server" yaml:"file_server" ini:"file_server"`
	Server     ServerConfig  `mapstructure:"server" json:"server" yaml:"server" ini:"server"`
	Monitor    MonitorConfig `mapstructure:"monitor" json:"monitor" yaml:"monitor" ini:"monitor"`
}

// 更新主配置结构体
type Config struct {
	Application Application    `mapstructure:"application" json:"application" yaml:"application" ini:"application"`
	Database    Database       `mapstructure:"database" json:"database" yaml:"database" ini:"database"`
	Crypto      []CryptoConfig `mapstructure:"crypto" json:"crypto" yaml:"crypto" ini:"crypto"`
	Kafka       KafkaConfig    `mapstructure:"kafka" json:"kafka" yaml:"kafka" ini:"kafka"`
	Rabbitmq    RabbitMQConf   `mapstructure:"rabbitmq" json:"rabbitmq" yaml:"rabbitmq" ini:"rabbitmq"`
	Redis       RedisConfig    `mapstructure:"redis" json:"redis" yaml:"redis" ini:"redis"`
	ES          Elasticsearch  `mapstructure:"es" json:"es" yaml:"es" ini:"es"`
	Mongodb     MongodbConf    `mapstructure:"mongodb" json:"mongodb" yaml:"mongodb" ini:"mongodb"`
}

var applicationConfig = new(Config)

func GetConfig() *Config {
	return applicationConfig
}

func (c *Config) GetHTTPPort() int {
	// 优先使用新的 server.http.port 配置
	if c.Application.Server.HTTP.Port > 0 {
		return c.Application.Server.HTTP.Port
	}
	// 回退到旧的 application.port 配置
	return c.Application.Port
}

func (c *Config) GetName() string {
	return c.Application.Name
}

func (c *Config) IsDebugMode() bool {
	return c.Application.Debug
}

func (c *Config) GetPrometheusConfig() PrometheusConfig {
	return c.Application.Monitor.Prometheus
}

func (c *Config) GetPProfConfig() PProfConfig {
	return c.Application.Monitor.PProf
}

// GetHTTPReadTimeout 获取超时时间（转换为 time.Duration）
func (c *Config) GetHTTPReadTimeout() (time.Duration, error) {
	if c.Application.Server.HTTP.ReadTimeout != "" {
		return time.ParseDuration(c.Application.Server.HTTP.ReadTimeout)
	}
	return 30 * time.Second, nil // 默认值
}

func (c *Config) GetHTTPWriteTimeout() (time.Duration, error) {
	if c.Application.Server.HTTP.WriteTimeout != "" {
		return time.ParseDuration(c.Application.Server.HTTP.WriteTimeout)
	}
	return 30 * time.Second, nil // 默认值
}

func (c *Config) GetGRPCPort() int {
	if c.Application.Server.GRPC.Port > 0 {
		return c.Application.Server.GRPC.Port
	}
	return 8631 // 默认值
}

// Validate 配置验证
func (c *Config) Validate() error {
	if c.Application.Name == "" {
		return errors.New("application name is required")
	}

	httpPort := c.GetHTTPPort()
	if httpPort <= 0 || httpPort > 65535 {
		return fmt.Errorf("invalid HTTP port: %d", httpPort)
	}

	grpcPort := c.GetGRPCPort()
	if grpcPort <= 0 || grpcPort > 65535 {
		return fmt.Errorf("invalid GRPC port: %d", grpcPort)
	}

	// 验证超时配置
	if _, err := c.GetHTTPReadTimeout(); err != nil {
		return fmt.Errorf("invalid HTTP read_timeout: %w", err)
	}

	if _, err := c.GetHTTPWriteTimeout(); err != nil {
		return fmt.Errorf("invalid HTTP write_timeout: %w", err)
	}

	return nil
}

func LoadConfigFromJson(configPath string, body any) {
	// 打开文件
	cf, _ := os.Open(configPath)
	// 关闭文件
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			panic(err)
		}
	}(cf)
	// NewDecoder创建一个从file读取并解码json对象的*Decoder，解码器有自己的缓冲，并可能超前读取部分json数据。
	dc := decoder.NewStreamDecoder(cf)
	// Decode从输入流读取下一个json编码值并保存在v指向的值里
	err := dc.Decode(body)
	if err != nil {
		panic(err)
	}
}

func LoadConfigFromIni(configPath string, body any) {
	err := ini.MapTo(body, configPath)
	if err != nil {
		panic(err)
	}
}

func LoadConfigFromYaml(configPath string, body any) {
	file, err := os.ReadFile(configPath)
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(file, body)
	if err != nil {
		panic(err)
	}
}

func LoadConfigFromToml(configPath string, body any) {
	_, err := toml.DecodeFile(configPath, body)
	if err != nil {
		panic(err)
	}
}

func ParseConfigByViper(configFile string, dest any) (err error) {
	v := viper.New()
	v.SetConfigFile(configFile)
	err = v.ReadInConfig()
	if err != nil {
		err = fmt.Errorf("failed to read config file: %w", err)
		return
	}
	// 直接反序列化为Struct
	if err = v.Unmarshal(dest); err != nil {
		err = fmt.Errorf("failed to Unmarshal: %w", err)
		return
	}
	return
}

func init() {
	// 读取服务配置文件
	var configLoad bool
	for _, config := range confPaths {
		if configLoad {
			break
		}
		if !utils.IsExist(config) {
			continue
		}
		err := file.ParseConfigByViper(config, applicationConfig)
		if err != nil {
			log.Printf(fmt.Sprintf("[init] -- failed to read config file: %s, err:%v", config, err))
			continue
		}
		configLoad = true
	}

	if !configLoad {
		log.Print("[init] -- there is no application config.")
	}
}
