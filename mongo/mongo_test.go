// Package mongo
//
//   _ __ ___   __ _ _ __  _   _| |_
//  | '_ ` _ \ / _` | '_ \| | | | __|
//  | | | | | | (_| | | | | |_| | |_
//  |_| |_| |_|\__,_|_| |_|\__,_|\__|
//
//  Buddha bless, no bugs forever!
//
//  Author:    lucas
//  Email:     1783022886@qq.com
//  Created:   2026/1/31 21:59
//  Version:   v1.0.0

package mongo

import (
	"context"
	"testing"
)

func TestName(t *testing.T) {
	ctx := context.Background()
	mongoOperator, err := NewMongoOperator(ctx, &Config{
		URI:         "mongodb://192.168.5.30:27017",
		MaxPoolSize: 100,
		MinPoolSize: 5,
		Auth: &AuthConfig{
			Username: "admin",
			Password: "openthedoor",
		},
	})
	if err != nil {
		t.Error(err)
	}
	err = mongoOperator.Ping(ctx)
	if err != nil {
		t.Error(err)
	}
	// NewRepository(mongoOperator)
}
