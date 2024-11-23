package jwtx

import (
	"fmt"
	"github.com/jasonlabz/potato/log"
	"testing"
	"time"
)

func TestName(t *testing.T) {
	jwtToken, err := GenerateJWTToken(&User{
		ID:       "1024",
		Name:     "lucas",
		Audience: "web",
		ExtraInfo: map[string]string{
			"hello": "world",
		},
	}, 10*time.Second)
	if err != nil {
		log.GetLogger().Fatal(err.Error())
	}
	user, err := ParseJWTToken(jwtToken)
	fmt.Println(user)
	fmt.Println(jwtToken)
}
