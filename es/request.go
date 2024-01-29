package es

import (
	"fmt"
	"strings"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/jasonlabz/potato/core/utils"
	"github.com/jasonlabz/potato/log"
)

type XRequest struct {
	*search.Request
}

func RequestBuilder() *XRequest {
	request := search.NewRequest()
	request.Query = types.NewQuery()
	return &XRequest{
		request,
	}
}

//QueryMatchAll
/** 语法：
GET /{索引名}/_search
{
    "query": {
        "match_all": {}
    }
}
*/
func (x *XRequest) QueryMatchAll() *XRequest {
	err := x.Query.UnmarshalJSON([]byte(`{"query": {"match_all": {}}}`))
	log.DefaultLogger().WithError(err).Error("set query match all failure")
	return x
}

func (x *XRequest) QueryMatchAllBoost(boost float32) *XRequest {
	err := x.Query.UnmarshalJSON([]byte(fmt.Sprintf(`{"query": {"match_all": {"boost":%f}}}`, boost)))
	log.DefaultLogger().WithError(err).Error("set query match all failure")
	return x
}

//QueryMatch 匹配单个字段,通过match实现全文搜索
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
	queryStr := fmt.Sprintf(`"%s":"%s"`, field, text)
	err := x.Query.UnmarshalJSON([]byte(fmt.Sprintf(`{"query": {"match": {%s}}}`, queryStr)))
	if err != nil {
		log.DefaultLogger().WithError(err).Error("set query match failure")
	}
	return x
}
func (x *XRequest) QueryMatchBoost(field, text string, boost float32) *XRequest {
	err := x.Query.UnmarshalJSON([]byte(fmt.Sprintf(`{"query": {"match": {"%s":{"query":"%s","boost":%f}}}}`, field, text, boost)))
	if err != nil {
		log.DefaultLogger().WithError(err).Error("set query match failure")
	}
	return x
}

//QueryMultiMatch 匹配单个字段,通过match实现全文搜索
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
	err := x.Query.UnmarshalJSON([]byte(fmt.Sprintf(`{"query":{"multi_match":{"query":"%s","fields":[%s]}}}`, text, strings.Join(fields, ","))))
	if err != nil {
		log.DefaultLogger().WithError(err).Error("set query multi match failure")
	}
	return x
}

func (x *XRequest) QueryMultiMatchBoost(fields []string, text string, boost float32) *XRequest {
	err := x.Query.UnmarshalJSON([]byte(fmt.Sprintf(`{"query":{"multi_match":{"query":"%s","fields":[%s],"boost":%f}}}`, text, strings.Join(fields, ","), boost)))
	if err != nil {
		log.DefaultLogger().WithError(err).Error("set query multi match failure")
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
	queryStr := fmt.Sprintf(`"%s":%s`, field, utils.JSONMarshal(value))
	err := x.Query.UnmarshalJSON([]byte(fmt.Sprintf(`{"query": {"term": {%s}}}`, queryStr)))
	if err != nil {
		log.DefaultLogger().WithError(err).Error("set query term failure")
	}
	return x
}

func (x *XRequest) QueryTermBoost(field string, value any, boost float32) *XRequest {
	err := x.Query.UnmarshalJSON([]byte(fmt.Sprintf(`{"query": {"term": {"%s":{"value":%v,"boost":%f}}}}`, field, utils.JSONMarshal(value), boost)))
	if err != nil {
		log.DefaultLogger().WithError(err).Error("set query term failure")
	}
	return x
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
func (x *XRequest) QueryTerms(field string, values []any) *XRequest {
	queryStr := fmt.Sprintf(`"%s":%s`, field, utils.JSONMarshal(values))
	err := x.Query.UnmarshalJSON([]byte(fmt.Sprintf(`{"query": {"term": {%s}}}`, queryStr)))
	if err != nil {
		log.DefaultLogger().WithError(err).Error("set query terms failure")
	}
	return x
}

func (x *XRequest) QueryTermsBoost(field string, values []any, boost float32) *XRequest {
	queryStr := fmt.Sprintf(`"%s":%s`, field, utils.JSONMarshal(values))
	err := x.Query.UnmarshalJSON([]byte(fmt.Sprintf(`{"query": {"term": {%s}}}`, queryStr)))
	if err != nil {
		log.DefaultLogger().WithError(err).Error("set query terms failure")
	}
	x.Query.Terms.Boost = &boost
	return x
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
func (x *XRequest) QueryRange(field string, start, end any) *XRequest {
	queryStr := fmt.Sprintf(`"%s": {"gte": %v,"lte": %v}`, field, start, end)
	err := x.Query.UnmarshalJSON([]byte(fmt.Sprintf(`{"query": {"range": {%s}}}`, queryStr)))
	if err != nil {
		log.DefaultLogger().WithError(err).Error("set query terms failure")
	}
	return x
}

func (x *XRequest) QueryRawJson(body string) *XRequest {
	err := x.Query.UnmarshalJSON([]byte(body))
	if err != nil {
		log.DefaultLogger().WithError(err).Error("set query terms failure")
	}
	return x
}

func (x *XRequest) PageQuery(pageNo, pageSize int) *XRequest {
	from := (pageNo - 1) * pageSize
	x.Request.From = &from
	x.Request.Size = &pageSize
	return x
}

func (x *XRequest) Build() *search.Request {
	return x.Request
}
