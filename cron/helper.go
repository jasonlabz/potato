package xcron

import (
	"fmt"
	"strings"

	"github.com/jasonlabz/potato/times"
	"github.com/jasonlabz/potato/utils"
)

type CronTaskBuilder struct {
	jobType    JobType  `json:"job_type"` //  1|每天；2|每月；3|每周；4|间隔（每隔2个小时，每隔30分钟）
	seconds    *string  `json:"seconds"`
	minutes    *string  `json:"minutes"`
	hours      *string  `json:"hours"`
	dayOfMonth []string `json:"day_of_month"`
	month      *string  `json:"month"`
	dayOfWeek  []string `json:"day_of_week"`
	spec       string   `json:"spec"`
}

func (b *CronTaskBuilder) Spec(spec string) *CronTaskBuilder {
	b.spec = spec
	return b
}

func (b *CronTaskBuilder) Seconds(seconds string) *CronTaskBuilder {
	b.seconds = &seconds
	return b
}

func (b *CronTaskBuilder) Minutes(minutes string) *CronTaskBuilder {
	b.minutes = &minutes
	return b
}

func (b *CronTaskBuilder) Hours(hours string) *CronTaskBuilder {
	b.hours = &hours
	return b
}

func (b *CronTaskBuilder) DayOfMonth(dayOfMonth ...string) *CronTaskBuilder {
	b.dayOfMonth = dayOfMonth
	return b
}

func (b *CronTaskBuilder) Month(month string) *CronTaskBuilder {
	b.month = &month
	return b
}

func (b *CronTaskBuilder) DayOfWeek(dayOfWeek ...string) *CronTaskBuilder {
	b.dayOfWeek = dayOfWeek
	return b
}

func (b *CronTaskBuilder) BuildSpec() string {
	if b.spec != "" {
		return b.spec
	}
	seconds := utils.IsTrueOrNot(*b.seconds == "", "*", *b.seconds)
	minutes := utils.IsTrueOrNot(*b.minutes == "", "*", *b.minutes)
	hours := utils.IsTrueOrNot(*b.hours == "", "*", *b.hours)
	dayOfMonth := utils.IsTrueOrNot(len(b.dayOfMonth) == 0, "*", strings.Join(b.dayOfMonth, ","))
	month := utils.IsTrueOrNot(*b.month == "", "*", *b.month)
	dayOfWeek := utils.IsTrueOrNot(len(b.dayOfWeek) == 0, "*", strings.Join(b.dayOfWeek, ","))
	return fmt.Sprintf("%s %s %s %s %s %s", seconds, minutes, hours, dayOfMonth, month, dayOfWeek)
}

func (b *CronTaskBuilder) Validate() error {
	splits := strings.Split(b.BuildSpec(), " ")
	var err error
	switch len(splits) {
	case 5:
		_, err = standardParser.Parse(b.spec)
	case 6:
		_, err = secondParser.Parse(b.spec)
	}
	if err != nil {
		return fmt.Errorf("invalid crontab spec")
	}
	return nil
}

func (b *CronTaskBuilder) Reset() {
	b.dayOfWeek = nil
	b.seconds = nil
	b.minutes = nil
	b.hours = nil
	b.month = nil
	b.dayOfMonth = nil
}

type JobType int

var (
	HourJob  JobType = 0 // 每天执行
	DayJob   JobType = 1 // 每天执行
	MonthJob JobType = 2 // 每月执行
	WeekJob  JobType = 3 // 每周执行
)

// GenCrontabStr 生成6位crontab表达式，仅支持固定间隔时间 https://blog.csdn.net/Michael_lcf/article/details/118784383
// 1、 Seconds （秒）
// 2、 Minutes（分）
// 3、 Hours（小时）
// 4、 Day-of-Month （天）
// 5、 Month（月）
// 6、 Day-of-Week （周）
func (b *CronTaskBuilder) GenCrontabStr(jobType JobType) (spec string) {
	after := times.Now()
	switch jobType {
	case DayJob:
		spec = fmt.Sprintf("%d %d %d * * *", after.Second(), after.Minute(), after.Hour())
	case MonthJob:
		spec = fmt.Sprintf("%d %d %d %d * *", after.Second(), after.Minute(), after.Hour(), after.Day())
	case WeekJob:
		spec = fmt.Sprintf("%d %d %d %d %d *", after.Second(), after.Minute(), after.Hour(), after.Day(), after.Month())
	case HourJob:
		spec = fmt.Sprintf("%d %d * * * *", after.Second(), after.Minute())
	default:
		spec = fmt.Sprintf("%d %d %d * * *", after.Second(), after.Minute(), after.Hour())
	}
	return
}

//type Options struct {
//	writeFile bool
//	logFormat string
//}
//
//type Option func(o *Options)
//
//func WithLevel(level string) Option {
//	return func(o *Options) {
//		o.logLevel = level
//	}
//}
//
//func WithBasePath(basePath string) Option {
//	return func(o *Options) {
//		o.basePath = basePath
//	}
//}
