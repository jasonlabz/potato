package es

import (
	"context"
	"fmt"
	"testing"
)

func TestES(t *testing.T) {
	ctx := context.Background()
	cli, err := NewElasticSearchOperator(&Config{
		Endpoints: []string{"127.0.0.1:9200"},
		Username:  "elastic",
		Password:  "openthedoor",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(cli)
	//err = cli.CreateIndex(ctx, "test1", `{"mappings":{"_doc":{"_all":{"enabled":false},"properties":{"uuid":{"type":"text","copy_to":"_search_all","fields":{"keyword":{"type":"keyword","ignore_above":150}}},"name":{"type":"text","copy_to":"_search_all","analyzer":"ik_max_word","search_analyzer":"ik_smart","fields":{"keyword":{"type":"keyword","ignore_above":150}}},"dt_from_explode_time":{"type":"date","copy_to":"_search_all","format":"strict_date_optional_time||epoch_millis"},"_search_all":{"type":"text"}},"date_detection":false,"dynamic_templates":[{"strings":{"match_mapping_type":"string","mapping":{"type":"text","copy_to":"_search_all","fields":{"keyword":{"type":"keyword","ignore_above":150}}}}}]}},"settings":{"index":{"number_of_shards":6,"number_of_replicas":1}}}`)
	//if err != nil {
	//	return
	//}
	list, err := cli.GetIndexList(ctx)
	if err != nil {
		return
	}
	response, err := cli.SearchDocuments(ctx, "test1", nil)
	if err != nil {
		return
	}

	fmt.Println(response)
	fmt.Println(list)
}

func TestES1(t *testing.T) {
	body := QueryBuilder()
	match := body.QueryMatchBoost("hello", "world", 1.0)
	fmt.Println(match)
}
