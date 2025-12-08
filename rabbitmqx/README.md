## RabbitMQ 🥔
> rabbitmq服务简单封装了常用使用功能，支持断线重连、持续消费，支持优先级队列、延迟队列等队列参数的设置，支持rabbitmq五种常用的队列模型使用。
> 该库基于一个connection对应多个channel的使用理念来封装使用，一个队列/一个交换机对应一个channel。

RabbitMQ默认会读取conf/application.yaml下的配置：
```yaml
rabbitmq:
  enable: false                # 是否启用
  strict: true                 # 是否为下游必需，如为true则会启动时panic所遇error
  host: "*******"              
  port: 8672
  username: lucas
  password: "*******"
  limit_conf: 
    attempt_times: 3          # 重试次数
    retry_wait_time: 3000     # 重试等待时间，单位ms
    prefetch_count: 100       # 队列预读取数量
    timeout: 5000             # 超时时间，单位ms
    queue_limit: 0            # 队列长度限制
  
```
- 初始化rabbitmq
```go
// 方式一： 服务启动自动读取配置完成初始化逻辑
func init() {
	appConf := configx.GetConfig()
	if appConf.Rabbitmq.Enable {
		mqConf := &MQConfig{}
		err := utils.CopyStruct(appConf.Rabbitmq, mqConf)
		if err != nil {
			zapx.GetLogger().WithError(err).Error(context.Background(), "copy rmq config error, skipping ...")
			return
		}
		operator, err = NewRabbitMQOperator(mqConf)
		if err == nil {
			return
		}
		zapx.GetLogger().WithError(err).Error(context.Background(), "init rmq Client error")
		if appConf.Rabbitmq.Strict {
			panic(fmt.Errorf("init rmq Client error: %v", err))
		}
	}
}

// 方式二：读取配置文件手动实例化，并赋值给全局变量resource.RMQOperator
func initRMQ(ctx context.Context) {
    provider := yaml.NewConfigProvider("./conf/application.yaml")
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
    resource.RMQOperator = operator
}
```
- 使用rabbitmq
```go
func TestRMQClient(t *testing.T) {
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