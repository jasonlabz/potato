package options

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type ListOptionFunc func(options *metav1.ListOptions)

func WithLabelSelector(label string) ListOptionFunc {
	return func(options *metav1.ListOptions) {
		options.LabelSelector = label
	}
}

func WithFieldSelector(field string) ListOptionFunc {
	return func(options *metav1.ListOptions) {
		options.FieldSelector = field
	}
}
func WithLimitCount(limit int64) ListOptionFunc {
	return func(options *metav1.ListOptions) {
		options.Limit = limit
	}
}

func WithContinues(continues string) ListOptionFunc {
	return func(options *metav1.ListOptions) {
		options.Continue = continues
	}
}

func WithWatch(watch bool) ListOptionFunc {
	return func(options *metav1.ListOptions) {
		options.Watch = watch
	}
}

func WithResourceVersion(version string) ListOptionFunc {
	return func(options *metav1.ListOptions) {
		options.ResourceVersion = version
	}
}

func WithSendInitialEvents(sendInitialEvents bool) ListOptionFunc {
	return func(options *metav1.ListOptions) {
		options.SendInitialEvents = &sendInitialEvents
	}
}
