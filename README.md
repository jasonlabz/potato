# potato
> potato主要对go语言开发过程中常用的基础组件进行封装，包含常用加解密cryptox、错误定义errors、本地缓存gocache、gorm框架、httpclient、常用日志框架等等。
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
logger := log.GetLogger(ctx)
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
	operator, err := rabbitmqx.NewRabbitMQ(ctx, &rabbitmqx.MQConfig{
		UserName: config.GetString(DefaultRMQConfName, "rabbitmq.username"),
		Password: config.GetString(DefaultRMQConfName, "rabbitmq.password"),
		Host:     config.GetString(DefaultRMQConfName, "rabbitmq.host"),
		Port:     config.GetInt(DefaultRMQConfName, "rabbitmq.port"),
	})
	if err != nil {
		panic(err)
	}
	defer func(operator *rabbitmqx.RabbitOperator) {
		err := operator.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(operator)
	logger := zapx.GetLogger(ctx)
	msg := &rabbitmqx.ExchangePushBody{}
	msg1 := &rabbitmqx.QueuePushBody{}
	for i := 0; i < 1000; i++ {
		//err = operator.Push(ctx, msg.SetMsg([]byte("hello")).SetExchangeName("test").SetPriority(1).BindQueue("test_queue", "test_queue").SetQueueMaxPriority(4, "test_queue"))
		err = operator.PushExchange(ctx, msg.SetMsg([]byte("hello")).SetExchangeName("test").SetPriority(1).BindQueue("test_queue", "test_queue").SetXMaxPriority(4, "test_queue"))
		err = operator.PushQueue(ctx, msg1.SetMsg([]byte("hello")).SetQueueName("testdddd").SetPriority(1).SetXMaxPriority(4))
		if err != nil {
			logger.Error(err.Error())
		}
	}
}
```

## redis 
> 对redis五种数据类型的操作进行简单封装，后续会支持延迟队列、发布订阅等功能的封装支持
> 

- redis使用
```go
func TestName(t *testing.T) {
	ctx := context.Background()

	op, err := redisx.NewRedisOperator(&redisx.Config{
		&redis.Options{
			Addr:     "1.117.XXX.XXX:8379",
			Password: "*************",
		},
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
```