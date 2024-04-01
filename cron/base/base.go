package base

type JobBase interface {
	GetJobName() string
	Run()
}
