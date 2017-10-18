package obs

// EventSource uniquely identifies the source of an event.
type EventSource uint32

// Event is system event.
type Event interface {
	GetSource() EventSource
}

type baseEvent struct {
	source EventSource
	// XXX hole!
	timestamp uint64
}

// GetSource returns the source ID of the object that has emitted e.
func (e baseEvent) GetSource() EventSource {
	return e.source
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
