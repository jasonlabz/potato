package xcron

import (
	"fmt"
	"strings"
	"time"

	"github.com/jasonlabz/potato/utils"
)

type CronTaskBuilder struct {
	JobType    JobType  `json:"job_type"` // 1|每天；2|每月；3|每周；4|间隔（每隔2个小时，每隔30分钟）
	Seconds    *string  `json:"seconds"`
	Minutes    *string  `json:"minutes"`
	Hours      *string  `json:"hours"`
	DayOfMonth []string `json:"day_of_month"`
	Month      *string  `json:"month"`
	DayOfWeek  []string `json:"day_of_week"`
	Spec       string   `json:"spec"`
	Interval   string   `json:"interval"` // 用于 @every 表达式
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

func (b *CronTaskBuilder) SetInterval(interval string) *CronTaskBuilder {
	b.Interval = interval
	return b
}

func (b *CronTaskBuilder) BuildSpec() string {
	if b.Spec != "" {
		return b.Spec
	}

	// 如果是 IntervalJob，直接返回 @every 表达式
	if b.JobType == IntervalJob {
		return fmt.Sprintf("@every %s", b.Interval)
	}

	// 其他任务类型，生成标准的 Cron 表达式
	seconds := utils.IsTrueOrNot(b.Seconds == nil || *b.Seconds == "", "*", *b.Seconds)
	minutes := utils.IsTrueOrNot(b.Minutes == nil || *b.Minutes == "", "*", *b.Minutes)
	hours := utils.IsTrueOrNot(b.Hours == nil || *b.Hours == "", "*", *b.Hours)
	dayOfMonth := utils.IsTrueOrNot(len(b.DayOfMonth) == 0, "*", strings.Join(b.DayOfMonth, ","))
	month := utils.IsTrueOrNot(b.Month == nil || *b.Month == "", "*", *b.Month)
	dayOfWeek := utils.IsTrueOrNot(len(b.DayOfWeek) == 0, "*", strings.Join(b.DayOfWeek, ","))

	return fmt.Sprintf("%s %s %s %s %s %s", seconds, minutes, hours, dayOfMonth, month, dayOfWeek)
}

func (b *CronTaskBuilder) Validate() error {
	spec := b.BuildSpec()

	// 如果是 @every 表达式，直接返回
	if strings.HasPrefix(spec, "@every") {
		return nil
	}

	// 否则，验证标准的 Cron 表达式
	splits := strings.Split(spec, " ")
	var err error
	switch len(splits) {
	case 5:
		_, err = standardParser.Parse(spec)
	case 6:
		_, err = secondParser.Parse(spec)
	}
	if err != nil {
		return fmt.Errorf("invalid crontab spec: %v", err)
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
	b.Interval = ""
}

type JobType int

const (
	HourJob     JobType = 0 // 每小时执行
	DayJob      JobType = 1 // 每天执行
	MonthJob    JobType = 2 // 每月执行
	WeekJob     JobType = 3 // 每周执行
	IntervalJob JobType = 4 // 间隔任务（@every）
	FixedJob    JobType = 5 // 固定时间任务（如 0 12 * * *）
)

// GenCrontabStr 生成 Cron 表达式
func (b *CronTaskBuilder) GenCrontabStr(jobType JobType) string {
	b.JobType = jobType

	switch jobType {
	case HourJob:
		after := time.Now()
		return fmt.Sprintf("%d %d * * * *", after.Second(), after.Minute())
	case DayJob:
		after := time.Now()
		return fmt.Sprintf("%d %d %d * * *", after.Second(), after.Minute(), after.Hour())
	case MonthJob:
		after := time.Now()
		return fmt.Sprintf("%d %d %d %d * *", after.Second(), after.Minute(), after.Hour(), after.Day())
	case WeekJob:
		after := time.Now()
		return fmt.Sprintf("%d %d %d * * %d", after.Second(), after.Minute(), after.Hour(), after.Weekday())
	case IntervalJob:
		return fmt.Sprintf("@every %s", b.Interval)
	case FixedJob:
		return b.BuildSpec()
	default:
		return fmt.Sprintf("%d %d %d * * *", time.Now().Second(), time.Now().Minute(), time.Now().Hour())
	}
}
