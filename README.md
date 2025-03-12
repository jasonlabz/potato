## Potato 🥔
Potato 是一个轻量级的 Go 语言工具库，专注于简化常见开发任务（如配置管理、HTTP 请求处理等）。通过提供简洁的 API 和模块化设计，帮助开发者快速构建高效的应用，包含常用加解密cryptox、错误定义errors、本地缓存gocache、gorm框架、httpclient、常用日志框架等等。

全局拥有默认的配置文件方式，也提供手动初始化配置的方式,其中redis默认会读取conf/application.yaml下的配置：
```yaml
application:
  name: lg_server
  port: 8630
debug: true
kafka:
  topic: ["XXX"]
  group_id: "XXX"
  bootstrap_servers: "XXX:XX,XXX:XX,XXX:XX"
  security_protocol: "PLAINTEXT"
  sasl_mechanism: "PLAIN"
  sasl_username: "XXX"
  sasl_password: "XXX"
database:
  db_type: "mysql"
  dsn: "root:xxxxxxx!~04@tcp(1.117.xxxx.xxxx:8306)/lg_server?charset=utf8mb4&parseTime=True&loc=Local"
  #  dsn: "user=postgres password=halojeff host=127.0.0.1 port=8432 dbname=lg_server sslmode=disable TimeZone=Asia/Shanghai"
  charset: "utf-8"
  log_mode: "info"
  max_idle_conn: 100
  max_open_conn: 10
redis:
  endpoints:
    - "1.117.xxx.xxx:8379"
  password: "xxxxxx"
  index_db: 0
  MinIdleConns: 10
  max_idle_conns: 50
  max_active_conns: 10
  max_retry_times: 5
  master_name:
  sentinel_username:
  sentinel_password:
rabbitmq:
  host: 1.117.xxx.xxx
  port: 8672
  username: lucas
  password: xxxxxxxx
crypto:
  - type: aes
    key: "xxxxxxxxxxxxxx"
  - type: des
    key: "xxxxxxx"
log:
  # 是否写入文件
  write_file: true
  # true|false
  json_log: false
  # error|warn|info|debug
  log_level: debug
  # 文件配置
  log_file_conf:
    log_file_path: ./log/lg_server.log
    max_size: 10
    max_age: 28
    max_backups: 100
    compress: false
```

## crypto
> 包含几种常见的加解密算法封装使用，使用默认秘钥进行初始化，可手动设置秘钥进行初始化
## log
> 主要针对zap和slog两个日志库进行了简单的封装，支持trace_id,user_id等参数打印

日志默认配置文件：
    conf/application.yaml (或者conf/app.yaml)
配置文件格式：
```yaml
log:
  # 是否写入文件
  write_file: true
  # true|false
  json_log: false
  # error|warn|info|debug
  log_level: debug
  # 文件配置
  log_file_conf:
    log_file_path: ./log/lg_server.log
    max_size: 10
    max_age: 28
    max_backups: 100
    compress: false
```

当没有配置文件时会使用默认配置初始化，初始化方式可以通过导入包路径或者显示调用初始化函数：

方式一：
```go
import _ "github.com/jasonlabz/potato/log/zapx"
// 使用默认配置
```

方式二：
```go
zapx.InitLogger(zapx.WithConfigPath("conf/application.yaml"))

将需要在日志中打印指定上下文信息时，例如trace_id,user_id等等时，可以通过初始化配置加入指定字段(且同时在context中缓存指定的字段，进行日志记录时会从上下文中读取并打印)：
zapx.InitLogger(zapx.WithConfigPath("conf/application.yaml"),zapx.WithFields("user_id", "trace_id"))

```
打印效果：
```json
2024-01-07 16:08:10.147852      INFO    middleware/logger.go:68   [GIN] request   {"trace_id": "33a5fb6c15b54cc3b820c5cd1efb3b97", "remote_addr": "127.0.0.1", "method": "GET", "agent": "apifox/1.0.0 (https://www.apifox.cn)", "body": "", "client_ip": "127.0.0.1", "path": ""}
2024-01-07 16:08:10.349345      INFO    impl/user_dao_impl.go:52  [DB] [200.987ms] [rows:1] SELECT * FROM `user` WHERE `user_id` = 1 ORDER BY `user`.`user_id` LIMIT 1    {"trace_id": "33a5fb6c15b54cc3b820c5cd1efb3b97", "remote_addr": "127.0.0.1"}
2024-01-07 16:08:10.349345      INFO    middleware/logger.go:77   [GIN] response  {"trace_id": "33a5fb6c15b54cc3b820c5cd1efb3b97", "remote_addr": "127.0.0.1", "error_message": "", "body": "{\"code\":0,\"version\"
:\"v1\",\"current_time\":\"2024-01-07 16:08:10\",\"data\":[{\"user_id\":1,\"user_name\":\"武秀兰\",\"gender\":1,\"register_ip\":\"127.0.0.1\",\"register_time\":\"2023-12-23T16:51:44+08:00\"}]}", "path": "", "status_code": "200", "cost": "201ms"}
```

- 使用日志zap
```go
// 获取日志对象
logger := log.GetLogger().WithContext(ctx)
// 打印日志
logger.WithError(err).Error("get user error")
```

## rabbitmq
> rabbitmq服务简单封装了常用使用功能，支持断线重连、持续消费，支持优先级队列、延迟队列等队列参数的设置，支持rabbitmq五种常用的队列模型使用。
> 该库基于一个connection对应多个channel的使用理念来封装使用，一个队列/一个交换机对应一个channel。

- 使用rabbitmq
```go
func TestName(t *testing.T) {
    ctx := context.Background()
    
    provider := yaml.NewConfigProvider("../../conf/application.yaml")
    config.AddProviders(DefaultRMQConfName, provider)
    operator, err := rabbitmqx.NewRabbitMQOperator(&rabbitmqx.MQConfig{
        Username: config.GetString(DefaultRMQConfName, "rabbitmq.username"),
        Password: config.GetString(DefaultRMQConfName, "rabbitmq.password"),
        Host:     config.GetString(DefaultRMQConfName, "rabbitmq.host"),
        Port:     config.GetInt(DefaultRMQConfName, "rabbitmq.port"),
    })
    if err != nil {
        panic(err)
    }
    defer func(operator *rabbitmqx.RabbitMQOperator) {
        err := operator.Close()
        if err != nil {
            fmt.Println(err)
        }
    }(operator)
    logger := zapx.GetLogger().WithContext(ctx)
    pdmsg := &rabbitmqx.PushDelayBody{}
    pmsg := &rabbitmqx.PushBody{}
    msg := &rabbitmqx.ExchangePushBody{}
    msg1 := &rabbitmqx.QueuePushBody{}
    
    for i := 0; i < 1000; i++ {
        err = operator.PushDelayMessage(ctx, pdmsg.SetMsg([]byte("hello")).
            SetExchangeName("test_exchange").
            BindQueue("test_delay", "test_delay").
            BindQueue("test_delay1", "test_delay1").
            SetXMaxPriority(4, "test_delay", "test_delay1").
            SetDelayTime(25*time.Second))
        if err != nil {
            logger.Error(err.Error())
            err = nil
        }
        
        err = operator.Push(ctx, pmsg.SetMsg([]byte("hello")).
            SetExchangeName("test").
            SetPriority(1).
            BindQueue("test_queue", "test_queue").
            SetXMaxPriority(4, "test_queue"))
        if err != nil {
            logger.Error(err.Error())
            err = nil
        }
        
        err = operator.PushExchange(ctx, msg.SetMsg([]byte("hello")).
            SetExchangeName("test").
            SetPriority(1).
            BindQueue("testdddd", "testdddd").
            BindQueue("test_queue", "test_queue").
            SetXMaxPriority(4, "test_queue", "testdddd"))
        if err != nil {
            logger.Error(err.Error())
            err = nil
        }
        //err = operator.PushExchange(ctx, msg.SetMsg([]byte("hello")).OpenConfirmMode(false).SetExchangeName("test").SetPriority(1).BindQueue("testdddd", "testdddd").BindQueue("test_queue", "test_queue").SetXMaxPriority(4, "test_queue", "testdddd"))
        err = operator.PushQueue(ctx, msg1.SetMsg([]byte("hello")).
            SetQueueName("testdddd").
            SetPriority(1).
            SetXMaxPriority(4))
        if err != nil {
            logger.Error(err.Error())
            err = nil
        }
    }
    body := &rabbitmqx.ConsumeBody{}
    
    deliveries, err := operator.Consume(ctx, body.SetQueueName("test_delay").SetPrefetchCount(10))
    if err != nil {
        logger.Error(err.Error())
        return
    }
    for delivery := range deliveries {
        fmt.Println("get msg: " + delivery.MessageId + " -- " + string(delivery.Body))
        operator.Ack(ctx, delivery)
    }
}

```

## redis 
> 对redis五种数据类型的操作进行简单封装，支持延迟消息队列、发布订阅等功能
> 

- redis使用

```go
func TestName(t *testing.T) {
    ctx := context.Background()
    
    op, err := goredis.NewRedisOperator(&goredis.Config{
        Addrs:    []string{"1.117.***.***:8379"},
        Password: "*******",
    })
    if err != nil {
        panic(err)
    }
    success, err := op.Set(ctx, "hello", "world")
    if err != nil {
        panic(err)
    }
    if success {
        fmt.Println("set success")
    }
    value, err := op.Get(ctx, "hello")
    if err != nil {
        fmt.Println(err)
    }
    fmt.Println(value)
}

func TestDelayQueue(t *testing.T) {
    ctx := context.Background()
    op, err := goredis.NewRedisOperator(&goredis.Config{
        Addrs:    []string{"1.117.***.***:8379"},
        Password: "**********",
    })
    if err != nil {
        panic(err)
    }
    //result, err := op.ZRangeWithScores(ctx, "potato_delay_queue_timeout:test_delay", 0, 1715224481294)
    err = op.PushDelayMessage(ctx, "test_delay", "hello", 10*time.Second)
    if err != nil {
        panic(err)
    }
    time.Sleep(20 * time.Second)
    //fmt.Println(result)
}
```
