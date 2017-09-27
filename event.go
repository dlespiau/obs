package obs

// Event is system event.
type Event interface {
}

type baseEvent struct {
	timestamp uint64
}

// TracepointEvent is fired when a Tracepoint is hit.
type TracepointEvent struct {
	baseEvent
	data []byte
}

// Data returns the tracepoint raw data.
func (e *TracepointEvent) Data() []byte {
	return e.data
}
