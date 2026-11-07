package jwtx

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jasonlabz/potato/log"
)

func TestName(t *testing.T) {
	jwtToken, err := GenerateJWTToken(context.Background(), &User{
		ID:       "1024",
		Name:     "lucas",
		Audience: "web",
		ExtraInfo: map[string]string{
			"hello": "world",
		},
	}, 10*time.Second)
	if err != nil {
		log.GetLogger().Fatal(context.Background(), err.Error())
	}
	user, err := ParseJWTToken(jwtToken)
	fmt.Println(user)
	fmt.Println(jwtToken)
}
