package rocketmqx

import (
	"github.com/apache/rocketmq-client-go/v2/primitive"
)

// ProduceMessage 生产消息体
type ProduceMessage struct {
	Topic      string
	Tags       string
	Keys       []string
	Body       []byte
	Properties map[string]string
	DelayLevel int
}

func (m *ProduceMessage) SetTopic(topic string) *ProduceMessage {
	m.Topic = topic
	return m
}

func (m *ProduceMessage) SetTags(tags string) *ProduceMessage {
	m.Tags = tags
	return m
}

func (m *ProduceMessage) SetKeys(keys ...string) *ProduceMessage {
	m.Keys = keys
	return m
}

func (m *ProduceMessage) SetBody(body []byte) *ProduceMessage {
	m.Body = body
	return m
}

func (m *ProduceMessage) SetProperty(key, value string) *ProduceMessage {
	if m.Properties == nil {
		m.Properties = make(map[string]string)
	}
	m.Properties[key] = value
	return m
}

// SetDelayLevel 设置延迟级别
// RocketMQ 支持 18 个延迟级别：
// 1=1s 2=5s 3=10s 4=30s 5=1m 6=2m 7=3m 8=4m 9=5m 10=6m
// 11=7m 12=8m 13=9m 14=10m 15=20m 16=30m 17=1h 18=2h
func (m *ProduceMessage) SetDelayLevel(level int) *ProduceMessage {
	m.DelayLevel = level
	return m
}

func (m *ProduceMessage) toPrimitiveMessage() *primitive.Message {
	msg := primitive.NewMessage(m.Topic, m.Body)
	if m.Tags != "" {
		msg.WithTag(m.Tags)
	}
	if len(m.Keys) > 0 {
		msg.WithKeys(m.Keys)
	}
	if m.DelayLevel > 0 {
		msg.WithDelayTimeLevel(m.DelayLevel)
	}
	for k, v := range m.Properties {
		msg.WithProperty(k, v)
	}
	return msg
}

// ConsumeConfig 消费配置
type ConsumeConfig struct {
	Topic         string
	GroupName     string
	Tags          string // tag 过滤表达式，如 "tagA || tagB"
	ConsumerModel ConsumerModel
	ConsumeFrom   ConsumeFromWhere
	OrderConsume  bool
}

type ConsumerModel int

const (
	Clustering   ConsumerModel = iota // 集群消费（默认）
	BroadCasting                      // 广播消费
)

type ConsumeFromWhere int

const (
	ConsumeFromLastOffset  ConsumeFromWhere = iota // 从最新偏移量开始（默认）
	ConsumeFromFirstOffset                         // 从最早偏移量开始
	ConsumeFromTimestamp                           // 从指定时间戳开始
)

func (c *ConsumeConfig) SetTopic(topic string) *ConsumeConfig {
	c.Topic = topic
	return c
}

func (c *ConsumeConfig) SetGroupName(name string) *ConsumeConfig {
	c.GroupName = name
	return c
}

func (c *ConsumeConfig) SetTags(tags string) *ConsumeConfig {
	c.Tags = tags
	return c
}

func (c *ConsumeConfig) SetConsumerModel(model ConsumerModel) *ConsumeConfig {
	c.ConsumerModel = model
	return c
}

func (c *ConsumeConfig) SetConsumeFrom(from ConsumeFromWhere) *ConsumeConfig {
	c.ConsumeFrom = from
	return c
}

func (c *ConsumeConfig) SetOrderConsume(order bool) *ConsumeConfig {
	c.OrderConsume = order
	return c
}
