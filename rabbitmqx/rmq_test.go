package rabbitmqx

import (
	"strings"
	"sync"
	"testing"
)

func TestMQConfigAddrEscapesCredentials(t *testing.T) {
	c := &MQConfig{Host: "127.0.0.1", Port: 5672, Username: "u@x", Password: "p:w/d#1"}
	if addr := c.addr(); addr != "amqp://u%40x:p%3Aw%2Fd%231@127.0.0.1:5672/%2F" {
		t.Fatalf("addr = %q", addr)
	}
	if logAddr := c.logAddr(); strings.Contains(logAddr, "p:w/d#1") || strings.Contains(logAddr, "u@x") {
		t.Fatalf("logAddr leaks credentials: %q", logAddr)
	}
}

func TestCancelQueueTwiceDoesNotPanic(t *testing.T) {
	r := &RabbitMQOperator{client: &Client{}}
	ch := make(chan bool)
	r.client.cancelChan.Store("q", ch)
	go func() {
		<-ch
		r.client.cancelChan.CompareAndDelete("q", ch)
	}()

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.CancelQueue("q")
		}()
	}
	wg.Wait()
	_ = r.CancelQueue("q")
}
