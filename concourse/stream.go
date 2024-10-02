package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/hashicorp/go-multierror"
)

type Keyword = string

type Object[T any] map[Keyword]T

func (obj Object[T]) Concat(other Object[T]) Object[T] {
	clone := obj.Clone()
	for k, v := range other {
		clone[k] = v
		// ov, found := obj[k]
		// if !found {
		// 	clone[k] = v
		// 	continue
		// }
		//
		// if reflect.TypeOf(v) != reflect.TypeOf(ov) {
		// 	// TODO: return false?
		// 	continue
		// }

		// sub, ok := v.(Object)
		// if ok {
		// 	clone[k] = ov.(Object).Concat(sub)
		// } else {
		// 	clone[k] = v
		// }
	}

	return clone
}

func (o Object[T]) Clone() Object[T] {
	res := Object[T]{}
	for k, v := range o {
		res[k] = v
	}
	return res
}

type Stream[T any] interface {
	Next(context.Context) (T, error)
	Close(context.Context) error
}

type Emitter[T any] interface {
	Emit(context.Context, T)
}

type Subscribable[T any] interface {
	Subscribe() Stream[T]
}

var ErrEndOfStream = errors.New("end of stream")
var ErrStreamInterrupted = errors.New("stream interrupted")

type Broadcast[T any] struct {
	subscribers []chan<- T
}

func NewBroadcast[T any]() *Broadcast[T] {
	return &Broadcast[T]{}
}

func (stream *Broadcast[T]) Subscribe() Stream[T] {
	ch := make(chan T)
	stream.subscribers = append(stream.subscribers, ch)
	return &subscription[T]{
		queue: ch,
	}
}

func (stream *Broadcast[T]) Close() {
	for _, sub := range stream.subscribers {
		close(sub)
	}
}

func (stream *Broadcast[T]) Emit(ctx context.Context, obj T) {
	slog.Info("broadcasting", "obj", obj)

	done := ctx.Done()

	for _, sub := range stream.subscribers {
		select {
		case sub <- obj:
		case <-done:
			return
		}
	}
}

type subscription[T any] struct {
	queue chan T
}

func (sub *subscription[T]) Emit(ctx context.Context, obj T) {
	slog.Info("emitting to subscription", "obj", obj)
	sub.queue <- obj
}

func (sub *subscription[T]) Next(ctx context.Context) (T, error) {
	var zero T
	select {
	case obj, ok := <-sub.queue:
		if !ok {
			return zero, ErrEndOfStream
		}

		return obj, nil
	case <-ctx.Done():
		return zero, ErrStreamInterrupted
	}
}

func (sub *subscription[T]) Close(ctx context.Context) error {
	// TODO: unsubscribe
	return nil
}

type Intersection[T any] struct {
	streams []Stream[Object[T]]

	intersection chan Object[T]
	errs         chan error

	live int32
	dead chan struct{}

	candidates  []*candidate[T]
	candidatesL sync.Mutex
}

type candidate[T any] struct {
	value    Object[T]
	streams  int
	vouchers map[Keyword]int
}

func Intersect[T any](ctx context.Context, streams ...Stream[Object[T]]) Stream[Object[T]] {
	if len(streams) == 1 {
		// optimization: prevent overhead; should be equivalent to single stream
		return streams[0]
	}

	inter := &Intersection[T]{
		streams:      streams,
		intersection: make(chan Object[T]),
		errs:         make(chan error, len(streams)),
		live:         int32(len(streams)),
		dead:         make(chan struct{}),
	}

	for _, stream := range streams {
		go inter.spawn(ctx, stream)
	}

	return inter
}

func (inter *Intersection[T]) Next(ctx context.Context) (Object[T], error) {
	select {
	case obj := <-inter.intersection:
		return obj, nil

	case <-ctx.Done():
		return nil, ErrStreamInterrupted

	case <-inter.dead:
		return nil, ErrEndOfStream

	case err := <-inter.errs:
		return nil, err
	}
}

func (inter *Intersection[T]) Close(ctx context.Context) error {
	var err error
	for i, stream := range inter.streams {
		err := stream.Close(ctx)
		if err != nil {
			err = multierror.Append(err, fmt.Errorf("close stream %d: %w", i, err))
		}
	}

	// XXX: possible panic write to closed channel
	close(inter.intersection)

	return err
}

func (inter *Intersection[T]) spawn(ctx context.Context, stream Stream[Object[T]]) {
	for {
		obj, err := stream.Next(ctx)
		if err != nil {
			if errors.Is(err, ErrEndOfStream) {
				res := atomic.AddInt32(&inter.live, -1)
				if res == 0 {
					close(inter.dead)
				}

				break
			}

			inter.errs <- err
			return
		}

		inter.emit(obj)
	}
}

func (inter *Intersection[T]) emit(obj Object[T]) {
	slog.Info("emitting to intersection", "obj", obj)

	inter.candidatesL.Lock()
	defer inter.candidatesL.Unlock()

	anyCompatible := false
	for _, candidate := range inter.candidates {
		co := candidate.value

		compatible := true
		for k, v := range co {
			objV, found := obj[k]
			if !found {
				continue
			}

			if !reflect.DeepEqual(objV, v) {
				compatible = false
				break
			}
		}
		// } else {
		// 	compatible = candidate.value == val
		// }

		if !compatible {
			continue
		}

		anyCompatible = true

		candidate.streams++

		// deep merge
		candidate.value = co.Concat(obj)

		for k := range co {
			_, ok := candidate.vouchers[k]
			if !ok {
				// other streams implicitly vouch for unknown keys
				candidate.vouchers[k] = candidate.streams
			} else {
				candidate.vouchers[k]++
			}
		}
	}

	if !anyCompatible {
		vouchers := map[Keyword]int{}
		for k := range obj {
			vouchers[k] = 1
		}

		inter.candidates = append(inter.candidates, &candidate[T]{
			value:    obj,
			streams:  1,
			vouchers: vouchers,
		})
	}

	for i, candidate := range inter.candidates {
		allVouched := true
		for _, vouched := range candidate.vouchers {
			if vouched < len(inter.streams) {
				allVouched = false
			}
		}

		if allVouched {
			inter.intersection <- candidate.value

			if len(inter.candidates) > i+1 { // TODO: probably an off-by-one somewhere in here
				inter.candidates = inter.candidates[i+1:]
			} else {
				inter.candidates = nil
			}

			break
		}
	}
}

type Chained[T any] struct {
	Stream   Stream[T]
	Continue func(T) (Stream[T], error)

	last T
}

func Chain[T any](stream Stream[T], cont func(T) (Stream[T], error)) Stream[T] {
	return &Chained[T]{
		Stream:   stream,
		Continue: cont,
	}
}

func (cat *Chained[T]) Next(ctx context.Context) (T, error) {
	var zero T
	for {
		obj, err := cat.Stream.Next(ctx)
		if err == nil {
			cat.last = obj
			return obj, nil
		}

		if errors.Is(err, ErrEndOfStream) {
			cont, err := cat.Continue(cat.last)
			if err != nil {
				return zero, fmt.Errorf("continue: %w", err)
			}

			err = cat.Stream.Close(ctx)
			if err != nil {
				return zero, fmt.Errorf("close previous stream: %w", err)
			}

			// skip first object, as it should be equal to the last
			//
			// TODO: this could perhaps check that it's actually equal, but it might
			// be better to just enforce this convention for infinite streams
			skipped, err := cont.Next(ctx)
			if err != nil {
				return zero, fmt.Errorf("skip first object: %w", err)
			}

			slog.Info("skipped first object", "obj", skipped)

			cat.Stream = cont
			continue
		}

		return zero, err
	}
}

func (cat *Chained[T]) Close(ctx context.Context) error {
	slog.Info("closing chained stream")
	return cat.Stream.Close(ctx)
}

type Aggregated[T any] struct {
	streams map[Keyword]Stream[T]
	cases   []reflect.SelectCase
	chans   map[Keyword]<-chan T
	keyword map[int]Keyword

	next Object[T]
}

func Aggregate[T any](ctx context.Context, streams map[Keyword]Stream[T]) Stream[Object[T]] {
	agg := &Aggregated[T]{
		streams: streams,
		cases:   make([]reflect.SelectCase, len(streams)),
		chans:   make(map[Keyword]<-chan T, len(streams)),
		keyword: make(map[int]Keyword, len(streams)),
		next:    map[string]T{},
	}

	i := 0
	for name, stream := range streams {
		agg.keyword[i] = name

		ch := make(chan T)
		agg.chans[name] = ch

		agg.cases[i] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ch),
		}

		go agg.update(ctx, stream, ch)

		i++
	}

	return agg
}

func (aggregated *Aggregated[T]) update(ctx context.Context, stream Stream[T], ch chan<- T) {
	for {
		obj, err := stream.Next(ctx)
		if err != nil {
			return
		}

		select {
		case ch <- obj:
		case <-ctx.Done():
			return
		}
	}
}

func (aggregated *Aggregated[T]) ready() bool {
	return len(aggregated.next) == len(aggregated.streams)
}

func (aggregated *Aggregated[T]) Next(ctx context.Context) (Object[T], error) {
	for name, ch := range aggregated.chans {
		_, has := aggregated.next[name]
		if has {
			continue
		}

		select {
		case obj := <-ch:
			next := aggregated.next.Clone()
			next[name] = obj
			aggregated.next = next

			if aggregated.ready() {
				return aggregated.next, nil
			}

		case <-ctx.Done():
			return nil, ErrStreamInterrupted
		}
	}

	cases := aggregated.cases

	doneIdx := len(cases)
	cases = append(cases, reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(ctx.Done()),
	})

	defaultIdx := len(cases)
	cases = append(cases, reflect.SelectCase{
		Dir: reflect.SelectDefault,
	})

	hasDefault := true

	var hasNew bool
	for {
		idx, val, _ := reflect.Select(cases)
		if idx == doneIdx { // ctx.Done()
			return nil, ErrStreamInterrupted
		}

		if hasDefault && idx == defaultIdx {
			if hasNew {
				return aggregated.next, nil
			}

			// nothing new, remove the default so we block on an update instead
			cases = cases[0 : len(cases)-2]
			hasDefault = false
			continue
		}

		name := aggregated.keyword[idx]
		obj := val.Interface().(T)

		next := aggregated.next.Clone()
		next[name] = obj
		aggregated.next = next

		if hasDefault {
			hasNew = true
		} else {
			return aggregated.next, nil
		}
	}
}

func (aggregated *Aggregated[T]) Close(ctx context.Context) error {
	var multiErr error
	for i, stream := range aggregated.streams {
		err := stream.Close(ctx)
		if err != nil {
			multiErr = multierror.Append(
				multiErr,
				fmt.Errorf("close stream %s: %w", i, err),
			)
		}
	}

	return multiErr
}

type SliceStream[T any] struct {
	load     func(context.Context) ([]T, error)
	loadOnce sync.Once
	loadErr  error
	values   []T
	offset   int
	closed   bool
}

func (s *SliceStream[T]) Next(ctx context.Context) (T, error) {
	var zero T
	if s.load != nil {
		s.loadOnce.Do(func() {
			s.values, s.loadErr = s.load(ctx)
		})
		if s.loadErr != nil {
			return zero, s.loadErr
		}
	}
	if s.offset >= len(s.values) {
		return zero, ErrEndOfStream
	}
	value := s.values[s.offset]
	s.offset++
	return value, nil
}

func (s *SliceStream[T]) Close(ctx context.Context) error {
	s.closed = true
	return nil
}
