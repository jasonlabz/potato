package kafkax

import (
	"time"

	"github.com/segmentio/kafka-go"
)

// ProduceMessage 生产消息体
type ProduceMessage struct {
	Topic   string
	Key     []byte
	Value   []byte
	Headers []kafka.Header
}

func (m *ProduceMessage) SetTopic(topic string) *ProduceMessage {
	m.Topic = topic
	return m
}

func (m *ProduceMessage) SetKey(key []byte) *ProduceMessage {
	m.Key = key
	return m
}

func (m *ProduceMessage) SetValue(value []byte) *ProduceMessage {
	m.Value = value
	return m
}

func (m *ProduceMessage) AddHeader(key string, value []byte) *ProduceMessage {
	m.Headers = append(m.Headers, kafka.Header{Key: key, Value: value})
	return m
}

func (m *ProduceMessage) toKafkaMessage() kafka.Message {
	msg := kafka.Message{
		Key:     m.Key,
		Value:   m.Value,
		Headers: m.Headers,
	}
	if m.Topic != "" {
		msg.Topic = m.Topic
	}
	return msg
}

// ConsumeConfig 消费配置
type ConsumeConfig struct {
	Topic          string
	GroupID        string
	Partition      int
	StartOffset    int64
	MinBytes       int
	MaxBytes       int
	MaxWait        time.Duration
	CommitInterval time.Duration
}

func (c *ConsumeConfig) SetTopic(topic string) *ConsumeConfig {
	c.Topic = topic
	return c
}

func (c *ConsumeConfig) SetGroupID(groupID string) *ConsumeConfig {
	c.GroupID = groupID
	return c
}

func (c *ConsumeConfig) SetPartition(partition int) *ConsumeConfig {
	c.Partition = partition
	return c
}

func (c *ConsumeConfig) SetStartOffset(offset int64) *ConsumeConfig {
	c.StartOffset = offset
	return c
}

func (c *ConsumeConfig) SetMinBytes(n int) *ConsumeConfig {
	c.MinBytes = n
	return c
}

func (c *ConsumeConfig) SetMaxBytes(n int) *ConsumeConfig {
	c.MaxBytes = n
	return c
}

func (c *ConsumeConfig) SetMaxWait(d time.Duration) *ConsumeConfig {
	c.MaxWait = d
	return c
}

func (c *ConsumeConfig) SetCommitInterval(d time.Duration) *ConsumeConfig {
	c.CommitInterval = d
	return c
}
