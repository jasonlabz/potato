package streams

import "fmt"

func Range[T any](source <-chan T, isParallel bool) Stream[T] {
	return Stream[T]{
		source:   source,
		parallel: isParallel,
	}
}

// Concat 拼接流
func Concat[T any](s Stream[T], others ...Stream[T]) Stream[T] {
	return s.Concat(others...)
}

func GoSafe(fn func()) {
	go RunSafe(fn)
}

func RunSafe(fn func()) {
	defer Recover()
	fn()
}

func Recover(cleanups ...func()) {
	for _, cleanup := range cleanups {
		cleanup()
	}

	if p := recover(); p != nil {
		fmt.Println(p)
	}
}

func Map[T any, R any](s Stream[T], mapper func(T) R) Stream[R] {
	mapped := make([]R, 0)

	for el := range s.source {
		mapped = append(mapped, mapper(el))
	}
	return Of(mapped...)
}

func FlatMap[T any, R any](s Stream[T], mapper func(T) Stream[R]) Stream[R] {
	streams := make([]Stream[R], 0)
	s.ForEach(func(t T) {
		streams = append(streams, mapper(t))
	})

	newEl := make([]R, 0)
	for _, str := range streams {
		newEl = append(newEl, str.ToSlice()...)
	}

	return Of(newEl...)
}

func ToSlice[T any]() Collector[T, T, any] {
	return &DefaultCollector[T, T, any]{
		supplier: func() any {
			temp := make([]T, 0)
			return temp
		},
		accumulator: func(t T, ts any) any {
			ts = append(ts.([]T), t)
			return ts
		},
		function: func(t any) any {
			return t
		},
	}
}
func ToMap[T any](keyMapper func(T) any, valueMapper func(T) any, opts ...func(oldV, newV any) any) Collector[T, T, any] {
	return &DefaultCollector[T, T, any]{
		supplier: func() any {
			temp := make(map[any]any, 0)
			return temp
		},
		accumulator: func(t T, ts any) any {
			key := keyMapper(t)
			value := valueMapper(t)

			for _, opt := range opts {
				value = MapMerge(key, value, ts.(map[any]any), opt)
				(ts.(map[any]any))[key] = value
				return ts
			}
			(ts.(map[any]any))[key] = value
			return ts
		},
		function: func(t any) any {
			return t
		},
	}
}

func GroupingBy[T any](keyMapper func(T) any, valueMapper func(T) any) Collector[T, T, any] {
	return &DefaultCollector[T, T, any]{
		supplier: func() any {
			temp := make(map[any][]any)
			return temp
		},
		accumulator: func(t T, ts any) any {
			key := keyMapper(t)
			value := valueMapper(t)
			(ts.(map[any][]any))[key] = append((ts.(map[any][]any))[key], value)
			return ts
		},
		function: func(t any) any {
			return t
		},
	}
}
