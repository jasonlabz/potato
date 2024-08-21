package idgen

import (
	"github.com/yitter/idgenerator-go/idgen"
	"math/rand"
)

func init() {
	SetIDGenerator(WithSeqBitLength(8), WithMethod(1))
	// 以上过程只需全局一次，且应在生成ID之前完成。
}

func GenUID() int64 {
	return idgen.NextId()
}

func SetIDGenerator(opts ...Option) {
	param := &GenParam{}
	for _, option := range opts {
		option(param)
	}
	var workID uint16
	if param.workID == nil {
		workID = uint16(rand.Intn(64))
	} else {
		workID = *param.workID
	}
	// 创建 IdGeneratorOptions 对象，可在构造函数中输入 WorkerId：
	var options = idgen.NewIdGeneratorOptions(workID)
	if param.baseTime != nil {
		options.BaseTime = *param.baseTime
	}
	if param.seqBitLength != nil {
		options.SeqBitLength = *param.seqBitLength
	}
	if param.seqBitLength != nil {
		options.SeqBitLength = *param.seqBitLength
	}
	if param.method != nil {
		options.Method = *param.method
	}
	if param.workerIdBitLength != nil {
		options.WorkerIdBitLength = *param.workerIdBitLength
	}
	if param.maxSeqNumber != nil {
		options.MaxSeqNumber = *param.maxSeqNumber
	}
	if param.minSeqNumber != nil {
		options.MinSeqNumber = *param.minSeqNumber
	}
	if param.topOverCostCount != nil {
		options.TopOverCostCount = *param.topOverCostCount
	}

	// 保存参数（务必调用，否则参数设置不生效）：
	idgen.SetIdGenerator(options)
}

type GenParam struct {
	workID            *uint16
	seqBitLength      *byte
	baseTime          *int64
	method            *uint16
	workerIdBitLength *byte
	maxSeqNumber      *uint32
	minSeqNumber      *uint32
	topOverCostCount  *uint32
}

type Option func(p *GenParam)

func WithWorkID(workID uint16) Option {
	return func(p *GenParam) {
		p.workID = &workID
	}
}

func WithSeqBitLength(seqBitLength byte) Option {
	return func(p *GenParam) {
		p.seqBitLength = &seqBitLength
	}
}

func WithBaseTime(baseTime int64) Option {
	return func(p *GenParam) {
		p.baseTime = &baseTime
	}
}

func WithMethod(method uint16) Option {
	return func(p *GenParam) {
		p.method = &method
	}
}

func WithWorkerIdBitLength(workerIdBitLength byte) Option {
	return func(p *GenParam) {
		p.workerIdBitLength = &workerIdBitLength
	}
}

func WithMaxSeqNumber(maxSeqNumber uint32) Option {
	return func(p *GenParam) {
		p.maxSeqNumber = &maxSeqNumber
	}
}

func WithMinSeqNumber(minSeqNumber uint32) Option {
	return func(p *GenParam) {
		p.minSeqNumber = &minSeqNumber
	}
}

func WithTopOverCostCount(topOverCostCount uint32) Option {
	return func(p *GenParam) {
		p.topOverCostCount = &topOverCostCount
	}
}
