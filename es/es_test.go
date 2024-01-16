package es

import (
	"fmt"
	"testing"
)

func TestES(t *testing.T) {
	cli, err := NewESClient(&Config{
		Endpoints: []string{"127.0.0.1:9200"},
		Username:  "elastic",
		Password:  "openthedoor",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(cli)
}
