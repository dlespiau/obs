package obs

import (
	"sync"
	"sync/atomic"
)

// Observer is the object that will observe the system. An observer is first
// configured to listen to various events such as tracepoints or kprobes being
// hit. Then events can be read from the observer when they occur.
type Observer struct {
	nextEventSource uint32
	tracepoints     []tracepointData
	close           chan interface{}
	events          chan Event
	wg              sync.WaitGroup
}

// tracepointData is the per-tracepoint data the observer keeps around.
type tracepointData struct {
	source EventSource
	tp     *tracepoint
}

// NewObserver creates an Observer.
func NewObserver() *Observer {
	return &Observer{
		close:  make(chan interface{}),
		events: make(chan Event),
	}
}

// AddTracepoint adds a tracepoint to watch for.
func (o *Observer) AddTracepoint(name string) EventSource {
	source := atomic.AddUint32(&o.nextEventSource, 1)
	o.tracepoints = append(o.tracepoints, tracepointData{
		source: EventSource(source),
		tp:     newTracepoint(name),
	})
	return EventSource(source)
}

// Open finish initializing the observer. From then on, events can be received
// with ReadEvent().
func (o *Observer) Open() error {
	var err error

	defer func() {
		if err != nil {
			o.Close()
		}
	}()

	for _, data := range o.tracepoints {
		tp := data.tp
		source := data.source

		if err = tp.open(); err != nil {
			return err
		}
		o.wg.Add(1)
		go func(tp *tracepoint, source EventSource) {
			// TODO(damien): should we hide the implementation details into the
			// tracepoint object and have it provide a channel?
			for {
				var nFds int
				nFds, err = tp.perf.poll(-1)
				if err != nil {
					break
				}
				if nFds == 0 {
					break
				}
				tp.perf.read(func(msg *perfEventSample, cpu int) {
					event := &TracepointEvent{
						baseEvent: baseEvent{
							source: source,
						},
						data: msg.DataCopy(),
					}
					o.events <- event
				}, nil)
			}
			o.wg.Done()
		}(tp, source)

	}

	return nil
}

// ReadEvent returns one event. This call blocks until an event is received.
func (o *Observer) ReadEvent() (Event, error) {
	select {
	case <-o.close:
		return nil, nil
	case event := <-o.events:
		return event, nil
	}
}

// Close frees precious resources acquired during Open.
func (o *Observer) Close() {
	close(o.close)
	for _, data := range o.tracepoints {
		data.tp.close()
	}
	o.wg.Wait()
}
