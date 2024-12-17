package xcron

import (
	"fmt"
	"strings"

	"github.com/jasonlabz/potato/times"
	"github.com/jasonlabz/potato/utils"
)

type CronTaskBuilder struct {
	JobType    JobType  `json:"job_type"` //  1|每天；2|每月；3|每周；4|间隔（每隔2个小时，每隔30分钟）
	Seconds    *string  `json:"seconds"`
	Minutes    *string  `json:"minutes"`
	Hours      *string  `json:"hours"`
	DayOfMonth []string `json:"day_of_month"`
	Month      *string  `json:"month"`
	DayOfWeek  []string `json:"day_of_week"`
	Spec       string   `json:"spec"`
}

func (b *CronTaskBuilder) SetSpec(spec string) *CronTaskBuilder {
	b.Spec = spec
	return b
}

func (b *CronTaskBuilder) SetSeconds(seconds string) *CronTaskBuilder {
	b.Seconds = &seconds
	return b
}

func (b *CronTaskBuilder) SetMinutes(minutes string) *CronTaskBuilder {
	b.Minutes = &minutes
	return b
}

func (b *CronTaskBuilder) SetHours(hours string) *CronTaskBuilder {
	b.Hours = &hours
	return b
}

func (b *CronTaskBuilder) SetDayOfMonth(dayOfMonth ...string) *CronTaskBuilder {
	b.DayOfMonth = dayOfMonth
	return b
}

func (b *CronTaskBuilder) SetMonth(month string) *CronTaskBuilder {
	b.Month = &month
	return b
}

func (b *CronTaskBuilder) SetDayOfWeek(dayOfWeek ...string) *CronTaskBuilder {
	b.DayOfWeek = dayOfWeek
	return b
}

func (b *CronTaskBuilder) BuildSpec() string {
	if b.Spec != "" {
		return b.Spec
	}
	seconds := utils.IsTrueOrNot(*b.Seconds == "", "*", *b.Seconds)
	minutes := utils.IsTrueOrNot(*b.Minutes == "", "*", *b.Minutes)
	hours := utils.IsTrueOrNot(*b.Hours == "", "*", *b.Hours)
	dayOfMonth := utils.IsTrueOrNot(len(b.DayOfMonth) == 0, "*", strings.Join(b.DayOfMonth, ","))
	month := utils.IsTrueOrNot(*b.Month == "", "*", *b.Month)
	dayOfWeek := utils.IsTrueOrNot(len(b.DayOfWeek) == 0, "*", strings.Join(b.DayOfWeek, ","))
	return fmt.Sprintf("%s %s %s %s %s %s", seconds, minutes, hours, dayOfMonth, month, dayOfWeek)
}

func (b *CronTaskBuilder) Validate() error {
	splits := strings.Split(b.BuildSpec(), " ")
	var err error
	switch len(splits) {
	case 5:
		_, err = standardParser.Parse(b.Spec)
	case 6:
		_, err = secondParser.Parse(b.Spec)
	}
	if err != nil {
		return fmt.Errorf("invalid crontab spec")
	}
	return nil
}

func (b *CronTaskBuilder) Reset() {
	b.DayOfWeek = nil
	b.Seconds = nil
	b.Minutes = nil
	b.Hours = nil
	b.Month = nil
	b.DayOfMonth = nil
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
