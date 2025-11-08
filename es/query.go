package es

import (
	"context"
	"fmt"
	"strings"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/jasonlabz/potato/internal/log"
	zapx "github.com/jasonlabz/potato/log"
	"github.com/jasonlabz/potato/utils"
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
	*types.Query
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
		Query: types.NewQuery(),
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
	err := q.Query.UnmarshalJSON([]byte(`{"query": {"match_all": {}}}`))
	if err != nil {
		q.l.Error(q.c, "set query match all failure", err)
		q.lastErr = err
	}
	return q
}

func (q *XQuery) QueryMatchAllBoost(boost float32) *XQuery {
	err := q.Query.UnmarshalJSON([]byte(fmt.Sprintf(`{"query": {"match_all": {"boost":%f}}}`, boost)))
	if err != nil {
		q.l.Error(q.c, "set query match all failure", err)
		q.lastErr = err
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
	queryStr := fmt.Sprintf(`"%s":"%s"`, field, text)
	err := q.Query.UnmarshalJSON([]byte(fmt.Sprintf(`{"query": {"match": {%s}}}`, queryStr)))
	if err != nil {
		q.l.Error(q.c, "set query match failure", err)
		q.lastErr = err
	}
	return q
}

func (q *XQuery) QueryMatchBoost(field, text string, boost float32) *XQuery {
	err := q.Query.UnmarshalJSON([]byte(fmt.Sprintf(`{"query": {"match": {"%s":{"query":"%s","boost":%f}}}}`, field, text, boost)))
	if err != nil {
		q.l.Error(q.c, "set query match failure", err)
	}
	return q
}

// QueryMultiMatch 匹配单个字段,通过match实现全文搜索
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
	err := q.Query.UnmarshalJSON([]byte(fmt.Sprintf(`{"query":{"multi_match":{"query":"%s","fields":[%s]}}}`, text, strings.Join(fields, ","))))
	if err != nil {
		q.l.Error(q.c, "set query multi match failure", err)
		q.lastErr = err
	}
	return q
}

func (q *XQuery) QueryMultiMatchBoost(fields []string, text string, boost float32) *XQuery {
	err := q.Query.UnmarshalJSON([]byte(fmt.Sprintf(`{"query":{"multi_match":{"query":"%s","fields":[%s],"boost":%f}}}`, text, strings.Join(fields, ","), boost)))
	if err != nil {
		q.l.Error(q.c, "set query multi match failure", err)
		q.lastErr = err
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
	queryStr := fmt.Sprintf(`"%s":%s`, field, utils.JSONMarshal(value))
	err := q.Query.UnmarshalJSON([]byte(fmt.Sprintf(`{"query": {"term": {%s}}}`, queryStr)))
	if err != nil {
		q.l.Error(q.c, "set query term failure", err)
		q.lastErr = err
	}
	return q
}

func (q *XQuery) QueryTermBoost(field string, value any, boost float32) *XQuery {
	err := q.Query.UnmarshalJSON([]byte(fmt.Sprintf(`{"query": {"term": {"%s":{"value":%v,"boost":%f}}}}`, field, utils.JSONMarshal(value), boost)))
	if err != nil {
		q.l.Error(q.c, "set query term failure", err)
		q.lastErr = err
	}
	return q
}

// QueryTerms 精确匹配单个字段
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
func (q *XQuery) QueryTerms(field string, values []any) *XQuery {
	queryStr := fmt.Sprintf(`"%s":%s`, field, utils.StringValue(values))
	err := q.Query.UnmarshalJSON([]byte(fmt.Sprintf(`{"query": {"term": {%s}}}`, queryStr)))
	if err != nil {
		q.l.Error(q.c, "set query terms failure", err)
		q.lastErr = err
	}
	return q
}

func (q *XQuery) QueryTermsBoost(field string, values []any, boost float32) *XQuery {
	queryStr := fmt.Sprintf(`"%s":%s`, field, utils.StringValue(values))
	err := q.Query.UnmarshalJSON([]byte(fmt.Sprintf(`{"query": {"term": {%s}}}`, queryStr)))
	if err != nil {
		q.l.Error(q.c, "set query terms failure", err)
		q.lastErr = err
	}
	q.Query.Terms.Boost = &boost
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
	queryStr := fmt.Sprintf(`"%s": {"gte": %v,"lte": %v}`, field, start, end)
	err := q.Query.UnmarshalJSON([]byte(fmt.Sprintf(`{"query": {"range": {%s}}}`, queryStr)))
	if err != nil {
		q.l.Error(q.c, "set query terms failure", err)
		q.lastErr = err
	}
	return q
}

func (q *XQuery) QueryRawJson(body string) *XQuery {
	err := q.Query.UnmarshalJSON([]byte(body))
	if err != nil {
		q.l.Error(q.c, "set query terms failure", err)
		q.lastErr = err
	}
	return q
}

func (q *XQuery) Build() (*types.Query, error) {
	return q.Query, q.lastErr
}
