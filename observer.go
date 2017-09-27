package obs

import (
	"sync"
)

// Observer is the object that will observe the system. An observer is first
// configured to listen to various events such as tracepoints or kprobes being
// hit. Then events can be read from the observer when they occur.
type Observer struct {
	tracepoints []*Tracepoint
	close       chan interface{}
	events      chan Event
	wg          sync.WaitGroup
}

// NewObserver creates an Observer.
func NewObserver() *Observer {
	return &Observer{
		close:  make(chan interface{}),
		events: make(chan Event),
	}
}

// AddTracepoint adds a tracepoint to watch for.
func (o *Observer) AddTracepoint(tp *Tracepoint) {
	o.tracepoints = append(o.tracepoints, tp)
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

	for _, tp := range o.tracepoints {
		if err = tp.open(); err != nil {
			return err
		}
		o.wg.Add(1)
		go func(tp *Tracepoint) {
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
						data: msg.DataCopy(),
					}
					o.events <- event
				}, nil)
			}
			o.wg.Done()
		}(tp)

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
	for _, tp := range o.tracepoints {
		tp.close()
	}
	o.wg.Wait()
}
