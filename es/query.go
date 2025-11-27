package es

import (
	"context"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"

	"github.com/jasonlabz/potato/internal/log"
	zapx "github.com/jasonlabz/potato/log"
)

type OptionConfig struct {
	l log.Logger
}

type Option func(opt *OptionConfig)

func WithLogger(logger log.Logger) Option {
	return func(opt *OptionConfig) {
		opt.l = logger
	}
}

type XQuery struct {
	query   *types.Query
	c       context.Context
	lastErr error
	l       log.Logger
}

func QueryBuilder(ctx context.Context, opts ...Option) *XQuery {
	optionConfig := &OptionConfig{}
	for _, opt := range opts {
		opt(optionConfig)
	}
	if optionConfig.l == nil {
		optionConfig.l = zapx.GetLogger()
	}
	return &XQuery{
		query: types.NewQuery(),
		c:     ctx,
		l:     optionConfig.l,
	}
}

// QueryMatchAll
/** 语法：
GET /{索引名}/_search
{
    "query": {
        "match_all": {}
    }
}
*/
func (q *XQuery) QueryMatchAll() *XQuery {
	q.query.MatchAll = &types.MatchAllQuery{}
	return q
}

func (q *XQuery) QueryMatchAllBoost(boost float32) *XQuery {
	q.query.MatchAll = &types.MatchAllQuery{
		Boost: &boost,
	}
	return q
}

// QueryMatch 匹配单个字段,通过match实现全文搜索
/** 语法：
GET /{索引名}/_search
{
  "query": {
	"match": {
	  "{FIELD}": "{TEXT}"
	}
  }
}
*/
func (q *XQuery) QueryMatch(field, text string) *XQuery {
	q.query.Match = map[string]types.MatchQuery{
		field: {Query: text},
	}
	return q
}

func (q *XQuery) QueryMatchBoost(field, text string, boost float32) *XQuery {
	q.query.Match = map[string]types.MatchQuery{
		field: {
			Query: text,
			Boost: &boost,
		},
	}
	return q
}

// QueryMultiMatch 匹配多个字段,通过multi_match实现全文搜索
/** 语法：
GET /{索引名}/_search
{
  "query": {
    "multi_match" : {
      "query":  "{TEXT}",
      "fields": ["{FIELD1}", "{FIELD2}"]
    }
  }
}
*/
func (q *XQuery) QueryMultiMatch(fields []string, text string) *XQuery {
	q.query.MultiMatch = &types.MultiMatchQuery{
		Query:  text,
		Fields: fields,
	}
	return q
}

func (q *XQuery) QueryMultiMatchBoost(fields []string, text string, boost float32) *XQuery {
	q.query.MultiMatch = &types.MultiMatchQuery{
		Query:  text,
		Fields: fields,
		Boost:  &boost,
	}
	return q
}

// QueryTerm 精确匹配单个字段
/** 语法：
GET /{索引名}/_search
	{
	  "query": {
	    "term": {
	      "{FIELD}": "{VALUE}"
	    }
	  }
	}
*/
func (q *XQuery) QueryTerm(field string, value any) *XQuery {
	q.query.Term = map[string]types.TermQuery{
		field: {Value: value},
	}
	return q
}

func (q *XQuery) QueryTermBoost(field string, value any, boost float32) *XQuery {
	q.query.Term = map[string]types.TermQuery{
		field: {
			Value: value,
			Boost: &boost,
		},
	}
	return q
}

// QueryTerms 精确匹配单个字段的多个值
/** 语法：
GET /{索引名}/_search
	{
	  "query": {
	    "terms": {
	      "{FIELD}": ["{VALUE1}", "{VALUE2}"]
	    }
	  }
	}
*/
func (q *XQuery) QueryTerms(field string, values []any) *XQuery {
	q.query.Terms = &types.TermsQuery{
		TermsQuery: map[string]types.TermsQueryField{
			field: values,
		},
	}
	return q
}

func (q *XQuery) QueryTermsBoost(field string, values []any, boost float32) *XQuery {
	q.query.Terms = &types.TermsQuery{
		TermsQuery: map[string]types.TermsQueryField{
			field: values,
		},
		Boost: &boost,
	}
	return q
}

// QueryRange 通过range实现范围查询，类似SQL语句中的>, >=, <, <=表达式。
/** 语法：
GET /{索引名}/_search
{
  "query": {
    "range": {
      "{FIELD}": {
        "gte": 10,
        "lte": 20
      }
    }
  }
}
*/
func (q *XQuery) QueryRange(field string, start, end any) *XQuery {
	// q.QueryRangeBoost()
	return q
}

// func (q *XQuery) QueryRangeBoost(field string, start, end any, boost float32) *XQuery {
//	var rangeQuery types.RangeQuery
//	switch start.(type) {
//	case float32, float64, int, int8, int16, int32, int64:
//
//		rangeQuery = types.NumberRangeQuery{
//		Boost: &boost,
//		Gte: float64(start),
//		}
//		q.query.Range = map[string]
//	case string:
//	case bool:
//	case time.Time:
//	}
//	q.query.Range = map[string]types.NumberRangeQuery{
//		field:
//	}
//
//	queryStr := fmt.Sprintf(`"%s": {"gte": %v,"lte": %v}`, field, start, end)
//	err := q.query.UnmarshalJSON([]byte(fmt.Sprintf(`{"query": {"range": {%s}}}`, queryStr)))
//	if err != nil {
//		q.l.Error(q.c, "set query terms failure", err)
//		q.lastErr = err
//	}
//	return q
// }

// XBoolQuery 布尔查询构建器
type XBoolQuery struct {
	parent *XQuery
	boolQ  *types.BoolQuery
}

// Must 必须匹配的条件
func (b *XBoolQuery) Must(query *types.Query) *XBoolQuery {
	if b.boolQ.Must == nil {
		b.boolQ.Must = []types.Query{}
	}
	b.boolQ.Must = append(b.boolQ.Must, *query)
	return b
}

// Should 应该匹配的条件
func (b *XBoolQuery) Should(query *types.Query) *XBoolQuery {
	if b.boolQ.Should == nil {
		b.boolQ.Should = []types.Query{}
	}
	b.boolQ.Should = append(b.boolQ.Should, *query)
	return b
}

// MustNot 必须不匹配的条件
func (b *XBoolQuery) MustNot(query *types.Query) *XBoolQuery {
	if b.boolQ.MustNot == nil {
		b.boolQ.MustNot = []types.Query{}
	}
	b.boolQ.MustNot = append(b.boolQ.MustNot, *query)
	return b
}

// Filter 过滤条件
func (b *XBoolQuery) Filter(query *types.Query) *XBoolQuery {
	if b.boolQ.Filter == nil {
		b.boolQ.Filter = []types.Query{}
	}
	b.boolQ.Filter = append(b.boolQ.Filter, *query)
	return b
}

// MinimumShouldMatch 设置最小应该匹配的数量
func (b *XBoolQuery) MinimumShouldMatch(minimumShouldMatch string) *XBoolQuery {
	b.boolQ.MinimumShouldMatch = &minimumShouldMatch
	return b
}

// Boost 设置权重
func (b *XBoolQuery) Boost(boost float32) *XBoolQuery {
	b.boolQ.Boost = &boost
	return b
}

// EndBool 结束布尔查询构建，返回父查询
func (b *XBoolQuery) EndBool() *XQuery {
	b.parent.query.Bool = b.boolQ
	return b.parent
}

// QueryBool 布尔查询
/** 语法：
GET /{索引名}/_search
{
  "query": {
    "bool": {
      "must": [
        { "match": { "title": "Search" } },
        { "match": { "content": "Elasticsearch" } }
      ],
      "filter": [
        { "term": { "status": "published" } }
      ]
    }
  }
}
*/
func (q *XQuery) QueryBool() *XBoolQuery {
	return &XBoolQuery{
		parent: q,
		boolQ:  &types.BoolQuery{},
	}
}

// QueryRawJson 原始JSON查询
/** 语法：
GET /{索引名}/_search
{
  "query": {
    // 任意合法的ES查询JSON
  }
}
*/
func (q *XQuery) QueryRawJson(body string) *XQuery {
	// 保留原有的JSON解析方式作为备选
	tempQuery := types.NewQuery()
	err := tempQuery.UnmarshalJSON([]byte(body))
	if err != nil {
		q.l.Error(q.c, "set query raw json failure", err)
		q.lastErr = err
		return q
	}
	q.query = tempQuery
	return q
}

func (q *XQuery) Build() (*types.Query, error) {
	return q.query, q.lastErr
}
