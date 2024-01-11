package times

import (
	"time"
)

const (
	MilliTimeFormat string = "2006-01-02 15:04:05.000"
	MicroTimeFormat string = "2006-01-02 15:04:05.000000"
	NanoTimeFormat  string = "2006-01-02 15:04:05.000000000"
	DateTimeFormat  string = "2006-01-02 15:04:05"
	DateFormat      string = "2006-01-02"
	TimeFormat      string = "15:04:05"
)

func init() {
	time.Local, _ = time.LoadLocation("Asia/Shanghai")
}

func Now() time.Time {
	return time.Now().In(time.Local)
}

func FormatTime(t time.Time) string {
	return Format(t, TimeFormat)
}

func FormatDate(t time.Time) string {
	return Format(t, DateFormat)
}

func FormatDateTime(t time.Time) string {
	return Format(t, DateTimeFormat)
}

func Format(t time.Time, format string) string {
	return t.Format(format)
}

func CurrentTime() string {
	return FormatDateTime(Now())
}

func CurrentTimeMillis() int64 {
	return Now().UnixMilli()
}

func CurrentTimeSeconds() int64 {
	return Now().Unix()
}

func ParseTime(timeStr string) (time.Time, error) {
	tm, err := time.ParseInLocation(DateFormat, timeStr, time.Local)
	return tm, err
}

// GetRecentlyDayTime 获取最近n天的开始和结束时间
func GetRecentlyDayTime(n int) (startTime, endTime string) {
	currentTime := Now()
	oldTime := currentTime.AddDate(0, 0, -n)
	endTime = currentTime.Format(DateFormat) + " 00:00:00"
	startTime = oldTime.Format(DateFormat) + " 23:59:59"
	return
}

// GetExpireDayTime 获取过期时间
func GetExpireDayTime(n time.Duration) (expireTime time.Time) {
	currentTime := Now()
	expireTime = currentTime.Add(n)
	return
}
