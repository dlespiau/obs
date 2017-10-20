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
	tp   *tracepoint
	data []byte
}

// Data returns the tracepoint raw data.
func (e *TracepointEvent) Data() []byte {
	return e.data
}

// GetInt retrieves an integer from the field 'name' from the tracepoint data.
// If 'name' isn't a valid field name, GetInt returns -1.
//
// One can consult the list of per-event fields in its format file:
//
//    # cat /sys/kernel/debug/tracing/events/sched/sched_process_exec/format
//    name: sched_process_exec
//    ID: 266
//    format:
//       field:unsigned short common_type;	offset:0;	size:2;	signed:0;
//       field:unsigned char common_flags;	offset:2;	size:1;	signed:0;
//       field:unsigned char common_preempt_count;	offset:3;	size:1;	signed:0;
//       field:int common_pid;	offset:4;	size:4;	signed:1;
//
//       field:__data_loc char[] filename;	offset:8;	size:4;	signed:1;
//       field:pid_t pid;	offset:12;	size:4;	signed:1;
//       field:pid_t old_pid;	offset:16;	size:4;	signed:1;
func (e *TracepointEvent) GetInt(name string) int {
	v, _ := e.tp.format.decodeInt(e.data, name)
	return v
}
