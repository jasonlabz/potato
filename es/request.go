package es

import (
	"context"

	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
)

type XRequest struct {
	request *search.Request
}

func RequestBuilder(ctx context.Context, opts ...Option) *XRequest {
	optionConfig := &OptionConfig{}
	for _, opt := range opts {
		opt(optionConfig)
	}
	if optionConfig.l == nil {
	}

	return &XRequest{
		request: &search.Request{},
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
func (x *XRequest) QueryMatchAll() *XRequest {
	x.request.Query = &types.Query{
		MatchAll: &types.MatchAllQuery{},
	}
	return x
}

func (x *XRequest) QueryMatchAllBoost(boost float32) *XRequest {
	x.request.Query = &types.Query{
		MatchAll: &types.MatchAllQuery{
			Boost: &boost,
		},
	}
	return x
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
func (x *XRequest) QueryMatch(field, text string) *XRequest {
	x.request.Query = &types.Query{
		Match: map[string]types.MatchQuery{
			field: {Query: text},
		},
	}
	return x
}

func (x *XRequest) QueryMatchBoost(field, text string, boost float32) *XRequest {
	x.request.Query = &types.Query{
		Match: map[string]types.MatchQuery{
			field: {
				Query: text,
				Boost: &boost,
			},
		},
	}
	return x
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
func (x *XRequest) QueryMultiMatch(fields []string, text string) *XRequest {
	x.request.Query = &types.Query{
		MultiMatch: &types.MultiMatchQuery{
			Query:  text,
			Fields: fields,
		},
	}
	return x
}

func (x *XRequest) QueryMultiMatchBoost(fields []string, text string, boost float32) *XRequest {
	x.request.Query = &types.Query{
		MultiMatch: &types.MultiMatchQuery{
			Query:  text,
			Fields: fields,
			Boost:  &boost,
		},
	}
	return x
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
func (x *XRequest) QueryTerm(field string, value any) *XRequest {
	x.request.Query = &types.Query{
		Term: map[string]types.TermQuery{
			field: {Value: value},
		},
	}
	return x
}

func (x *XRequest) QueryTermBoost(field string, value any, boost float32) *XRequest {
	x.request.Query = &types.Query{
		Term: map[string]types.TermQuery{
			field: {
				Value: value,
				Boost: &boost,
			},
		},
	}
	return x
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
func (x *XRequest) QueryTerms(field string, values []any) *XRequest {
	x.request.Query = &types.Query{
		Terms: &types.TermsQuery{
			TermsQuery: map[string]types.TermsQueryField{
				field: values,
			},
		},
	}
	return x
}

func (x *XRequest) QueryTermsBoost(field string, values []any, boost float32) *XRequest {
	x.request.Query = &types.Query{
		Terms: &types.TermsQuery{
			TermsQuery: map[string]types.TermsQueryField{
				field: values,
			},
			Boost: &boost,
		},
	}
	return x
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
func (x *XRequest) QueryRawJson(body string) *XRequest {
	tempQuery := types.NewQuery()
	err := tempQuery.UnmarshalJSON([]byte(body))
	if err != nil {
		return x
	}
	x.request.Query = tempQuery
	return x
}

// SetQuery 设置查询对象
func (x *XRequest) SetQuery(query *types.Query) *XRequest {
	x.request.Query = query
	return x
}

// PageQuery 分页查询
/** 语法：
GET /{索引名}/_search
{
  "from": 0,
  "size": 20
}
*/
func (x *XRequest) PageQuery(pageNo, pageSize int) *XRequest {
	from := (pageNo - 1) * pageSize
	x.request.From = &from
	x.request.Size = &pageSize
	return x
}

// SetFrom 设置起始位置
func (x *XRequest) SetFrom(from int) *XRequest {
	x.request.From = &from
	return x
}

// SetSize 设置返回大小
func (x *XRequest) SetSize(size int) *XRequest {
	x.request.Size = &size
	return x
}

// SetSort 设置排序
/** 语法：
GET /{索引名}/_search
{
  "sort": [
    { "field1": { "order": "desc" } },
    { "field2": { "order": "asc" } }
  ]
}
*/
func (x *XRequest) SetSort(sorters ...types.SortCombinations) *XRequest {
	x.request.Sort = sorters
	return x
}

// SetSource 设置返回字段
/** 语法：
GET /{索引名}/_search
{
  "_source": {
    "includes": ["field1", "field2"],
    "excludes": ["field3"]
  }
}
*/
func (x *XRequest) SetSource(includes []string, excludes []string) *XRequest {
	x.request.Source_ = &types.SourceFilter{
		Includes: includes,
		Excludes: excludes,
	}
	return x
}

// SetAggregations 设置聚合
/** 语法：
GET /{索引名}/_search
{
  "aggs": {
    "agg_name": {
      "terms": { "field": "field_name" }
    }
  }
}
*/
func (x *XRequest) SetAggregations(aggs map[string]types.Aggregations) *XRequest {
	x.request.Aggregations = aggs
	return x
}

// SetHighlight 设置高亮
/** 语法：
GET /{索引名}/_search
{
  "highlight": {
    "fields": {
      "content": {}
    }
  }
}
*/
func (x *XRequest) SetHighlight(highlight *types.Highlight) *XRequest {
	x.request.Highlight = highlight
	return x
}

// SetTimeout 设置超时
func (x *XRequest) SetTimeout(timeout string) *XRequest {
	x.request.Timeout = &timeout
	return x
}

func (x *XRequest) Build() *search.Request {
	return x.request
}
