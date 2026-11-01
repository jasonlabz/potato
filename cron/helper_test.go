package xcron

import "testing"

func TestValidateWithoutInit(t *testing.T) {
	// 未调用 InitCron 时 Validate 也应能工作
	if err := (&CronTaskBuilder{Spec: "0 */5 * * * *"}).Validate(); err != nil {
		t.Fatalf("Validate(6 fields) error = %v", err)
	}
	if err := (&CronTaskBuilder{Spec: "0 99 * * * *"}).Validate(); err == nil {
		t.Fatal("Validate(invalid minute) should fail")
	}
	if err := (&CronTaskBuilder{Spec: "* * *"}).Validate(); err == nil {
		t.Fatal("Validate(3 fields) should fail")
	}
	for _, spec := range []string{"@daily", "@every 1s", "0  0 * * * *", "CRON_TZ=Asia/Shanghai 0 0 0 * * *", "0 0 * * *"} {
		if err := (&CronTaskBuilder{Spec: spec}).Validate(); err != nil {
			t.Fatalf("Validate(%q) error = %v", spec, err)
		}
	}
}

func TestRegisterBeforeInit(t *testing.T) {
	if err := RegisterFunc("@every 1s", func() {}, false); err != ErrCronNotInitialized {
		t.Fatalf("RegisterFunc() error = %v", err)
	}
	<-StopCron().Done()
}
