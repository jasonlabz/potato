package streams

import (
	"runtime"
	"sort"
	"sync"
)

const worker = 1

type Optional[T any] struct {
	v *T
}

func (o Optional[T]) IsPresent() bool {
	return o.v != nil
}

func (o Optional[T]) Get() (v T, ok bool) {
	if o.v == nil {
		return *new(T), false
	}
	return *o.v, true
}
func (o Optional[T]) IfPresent(fn func(T)) {
	if o.v != nil {
		fn(*o.v)
	}
}

type Stream[T any] struct {
	parallel bool
	source   <-chan T
}

/*#################### 创建阶段 ####################*/

// Of 通过参数创建Stream对象
func Of[T any](items ...T) Stream[T] {
	source := make(chan T, len(items))
	defer close(source)
	for _, item := range items {
		source <- item
	}
	return convertToStream(source, false)
}

// OfParallel 通过参数创建Parallel Stream对象,
func OfParallel[T any](items ...T) Stream[T] {
	source := make(chan T, len(items))
	defer close(source)
	for _, item := range items {
		source <- item
	}
	return convertToStream(source, true)
}

// Call 通过函数创建Stream对象
func Call[T any](fn func(source <-chan T)) Stream[T] {
	source := make(chan T)
	go func() {
		defer close(source)
		fn(source)
	}()

	return convertToStream(source, false)
}

// CallParallel 通过函数创建Parallel Stream对象
func CallParallel[T any](fn func(source <-chan T)) Stream[T] {
	source := make(chan T)
	go func() {
		defer close(source)
		fn(source)
	}()

	return convertToStream(source, true)
}

// convertToStream 转换通道数据为Stream
func convertToStream[T any](source chan T, parallel bool) Stream[T] {
	return Stream[T]{
		source:   source,
		parallel: parallel,
	}
}

// clear 清空channel
func clear[T any](source <-chan T) {
	for range source {
	}
}

/*#################### 加工处理阶段 ####################*/

// Concat 拼接多个Stream对象
func (s Stream[T]) Concat(others ...Stream[T]) Stream[T] {
	source := make(chan T)
	go func() {
		defer close(source)
		for item := range s.source {
			source <- item
		}
		for _, each := range others {
			for item := range each.source {
				source <- item
			}
		}
	}()
	return convertToStream(source, s.parallel)
}

func (s Stream[T]) Filter(fn func(item T) bool) Stream[T] {
	return s.Walk(func(item T, pipe chan<- T) {
		if fn(item) {
			pipe <- item
		}
	})
}

func (s Stream[T]) Count() (count int) {
	for range s.source {
		count++
	}
	return
}

func (s Stream[T]) Peek(fn func(item *T)) Stream[T] {
	return s.Walk(func(item T, pipe chan<- T) {
		fn(&item)
		pipe <- item
	})
}

func (s Stream[T]) Limit(maxSize int64) Stream[T] {
	if maxSize < 0 {
		panic("n must not be negative")
	}
	source := make(chan T)
	go func() {
		defer close(source)
		var n int64 = 0
		for item := range s.source {
			if n < maxSize {
				source <- item
				n++
			} else {
				break
			}
		}
	}()
	return Range(source, s.parallel)
}

func (s Stream[T]) Skip(n int64) Stream[T] {
	if n < 0 {
		panic("n must not be negative")
	}
	if n == 0 {
		return s
	}
	source := make(chan T)
	go func() {
		for item := range s.source {
			n--
			if n >= 0 {
				continue
			} else {
				source <- item
			}
		}
		close(source)
	}()

	return Range(source, s.parallel)
}

func (s Stream[T]) Distinct(fn func(item T) any) Stream[T] {
	source := make(chan T)
	GoSafe(func() {
		defer close(source)

		keys := make(map[any]struct{})
		for item := range s.source {
			key := fn(item)
			if _, ok := keys[key]; !ok {
				source <- item
				keys[key] = struct{}{}
			}
		}
	})
	return Range(source, s.parallel)
}

func (s Stream[T]) Sorted(less func(a, b T) bool) Stream[T] {
	var items []T
	for item := range s.source {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return less(items[i], items[j])
	})
	return Of(items...)
}

func (s Stream[T]) Reverse() Stream[T] {
	var items []T
	for item := range s.source {
		items = append(items, item)
	}
	for i := len(items)/2 - 1; i >= 0; i-- {
		opp := len(items) - 1 - i
		items[i], items[opp] = items[opp], items[i]
	}

	return Of(items...)
}

func (s Stream[T]) Max(comparator func(T, T) int) Optional[T] {
	max, ok := s.FindFirst().Get()
	if !ok {
		return Optional[T]{v: nil}
	}
	s.ForEach(func(t T) {
		if comparator(t, max) > 0 {
			max = t
		}
	})
	return Optional[T]{v: &max}
}

func (s Stream[T]) Min(comparator func(T, T) int) Optional[T] {

	//min := s.FindFirst()
	min, ok := s.FindFirst().Get()
	if !ok {
		return Optional[T]{v: nil}
	}
	s.ForEach(func(t T) {
		if comparator(t, min) < 0 {
			min = t
		}
	})
	return Optional[T]{v: &min}
}

func (s Stream[T]) ForEach(fn func(item T)) {
	var workers = 1
	if s.parallel {
		workers = runtime.NumCPU() * 2
	}
	//go func() {
	var wg sync.WaitGroup
	// 这里是个占位类型
	pool := make(chan struct{}, workers)
	for item := range s.source {
		val := item
		// 这里是个占位类型值
		pool <- struct{}{}
		wg.Add(1)
		GoSafe(func() {
			defer func() {
				wg.Done()
				<-pool
			}()
			fn(val)
		})
	}
	wg.Wait()
	close(pool)
}

// Walk 让调用者处理每个Item，调用者可以根据给定的Item编写零个、一个或多个项目
func (s Stream[T]) Walk(fn func(item T, pipe chan<- T)) Stream[T] {
	return s.walkLimited(fn)
}

// walkLimited 遍历工作的协程个数限制
func (s Stream[T]) walkLimited(fn func(item T, pipe chan<- T)) Stream[T] {
	var workers = 1
	if s.parallel {
		workers = runtime.NumCPU() * 2
	}
	pipe := make(chan T, workers)
	go func() {
		var wg sync.WaitGroup
		// 这里是个占位类型
		pool := make(chan struct{}, workers)
		for item := range s.source {
			val := item
			// 这里是个占位类型值
			pool <- struct{}{}
			wg.Add(1)
			GoSafe(func() {
				defer func() {
					wg.Done()
					<-pool
				}()
				fn(val, pipe)
			})
		}
		wg.Wait()
		close(pipe)
	}()
	return Range(pipe, s.parallel)
}

// AllMatch 返回此流中是否全都满足条件
func (s Stream[T]) AllMatch(predicate func(T) bool) bool {
	// 非缓冲通道
	flag := make(chan bool)
	GoSafe(func() {
		tempFlag := true
		for item := range s.source {
			if !predicate(item) {
				go clear(s.source)
				tempFlag = false
				break
			}
		}
		flag <- tempFlag
	})
	return <-flag
}

// AnyMatch 返回此流中是否存在元素满足所提供的条件
func (s Stream[T]) AnyMatch(predicate func(T) bool) bool {
	flag := make(chan bool)
	GoSafe(func() {
		tempFlag := false
		for item := range s.source {
			if predicate(item) {
				go clear(s.source)
				tempFlag = true
				break
			}
		}
		flag <- tempFlag
	})

	return <-flag
}

// NoneMatch 返回此流中是否全都不满足条件
func (s Stream[T]) NoneMatch(predicate func(T) bool) bool {
	flag := make(chan bool)
	GoSafe(func() {
		tempFlag := true
		for item := range s.source {
			if predicate(item) {
				go clear(s.source)
				tempFlag = false
				break
			}
		}
		flag <- tempFlag
	})

	return <-flag
}

func (s Stream[T]) FindFirst() Optional[T] {
	for item := range s.source {
		go clear(s.source)
		return Optional[T]{v: &item}
	}
	return Optional[T]{v: nil}
}

func (s Stream[T]) FindLast() Optional[T] {
	tempStream := s.Reverse()
	for item := range tempStream.source {
		go clear(tempStream.source)
		return Optional[T]{v: &item}
	}
	return Optional[T]{v: nil}
}

func (s Stream[T]) Reduce(accumulator func(T, T) T) Optional[T] {
	var cnt = 0
	var res T
	for item := range s.source {
		if cnt == 0 {
			cnt++
			res = item
			continue
		}
		cnt++
		res = accumulator(res, item)
	}
	if cnt == 0 {
		return Optional[T]{v: nil}
	}
	return Optional[T]{v: &res}
}

func (s Stream[T]) Map(fn func(item T) any) Stream[any] {
	return Map[T](s, fn)
}

func (s Stream[T]) MapToInt(mapper func(T) int64) Stream[int64] {
	return Map[T](s, mapper)
}

func (s Stream[T]) MapToDouble(mapper func(T) float64) Stream[float64] {
	return Map[T](s, mapper)
}

func (s Stream[T]) FlatMap(mapper func(T) Stream[any]) Stream[any] {
	return FlatMap[T](s, mapper)
}

func (s Stream[T]) FlatMapToInt(mapper func(T) Stream[any]) Stream[any] {
	return FlatMap[T](s, mapper)
}
func (s Stream[T]) FlatMapToDouble(mapper func(T) Stream[float64]) Stream[float64] {
	return FlatMap[T](s, mapper)
}

func (s Stream[T]) ToSlice() []T {
	r := make([]T, 0)
	for item := range s.source {
		r = append(r, item)
	}
	return r
}

func (s Stream[T]) Collect(collector Collector[T, T, any]) any {
	temp := collector.Supplier()()
	for item := range s.source {
		temp = collector.Accumulator()(item, temp)
	}
	return collector.Finisher()(temp)
}

/*#################### 结果汇总阶段 ####################*/
