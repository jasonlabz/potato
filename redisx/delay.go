package redisx

import "context"

// PublishBody 消息结构体
type PublishBody struct {
	MsgID     string `msgpack:"1"` // 消息id
	Topic     string `msgpack:"2"` // 消息名
	Delay     int64  `msgpack:"3"` // 延迟时间
	PlayLoad  []byte `msgpack:"4"` // 消息体
	Timestamp int64  `msgpack:"5"` // 消息投递时间
	Extra     string `msgpack:"6"` // 辅助消息
}

// PushDelayQueue 往延迟队列推送消息
func (op *RedisOperator) PushDelayQueue(ctx context.Context, key string, data *PublishBody) (err error) {

	return
}
