package obs

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
)

/*
#include <stdio.h>
#include <string.h>
#include <stdint.h>
#include <linux/unistd.h>
#include <linux/bpf.h>
#include <linux/perf_event.h>
#include <sys/resource.h>
#include <stdlib.h>

void create_perf_event_attr(int type, int config, int sample_type,
			    int wakeup_events, void *attr)
{
	struct perf_event_attr *ptr = (struct perf_event_attr *) attr;

	memset(ptr, 0, sizeof(*ptr));

	ptr->type = type;
	ptr->size = sizeof(*ptr);
	ptr->config = config;
	ptr->sample_type = sample_type;
	ptr->sample_period = 1;
	ptr->wakeup_events = wakeup_events;
}

static void dump_data(uint8_t *data, size_t size, int cpu)
{
	int i;

	printf("event on cpu%d: ", cpu);
	for (i = 0; i < size; i++)
		printf("%02x ", data[i]);
	printf("\n");
}

struct event_sample {
	struct perf_event_header header;
	uint32_t size;
	uint8_t data[];
};

struct read_state {
	size_t raw_size;
	void *base, *begin, *end, *head;
};

int perf_event_read_init(int page_count, int page_size, void *_header, void *_state)
{
	volatile struct perf_event_mmap_page *header = _header;
	struct read_state *state = _state;
	uint64_t data_tail = header->data_tail;
	uint64_t data_head = *((volatile uint64_t *) &header->data_head);

	__sync_synchronize();
	if (data_head == data_tail)
		return 0;

	state->head = (void *) data_head;
	state->raw_size = page_count * page_size;
	state->base  = ((uint8_t *)header) + page_size;
	state->begin = state->base + data_tail % state->raw_size;
	state->end   = state->base + data_head % state->raw_size;

	return state->begin != state->end;
}

int perf_event_read(void *_state, void *buf, void *_msg)
{
	void **msg = (void **) _msg;
	struct read_state *state = _state;
	struct event_sample *e = state->begin;

	if (state->begin == state->end)
		return 0;

	if (state->begin + e->header.size > state->base + state->raw_size) {
		uint64_t len = state->base + state->raw_size - state->begin;

		memcpy(buf, state->begin, len);
		memcpy((char *) buf + len, state->base, e->header.size - len);

		e = buf;
		state->begin = state->base + e->header.size - len;
	} else if (state->begin + e->header.size == state->base + state->raw_size) {
		state->begin = state->base;
	} else {
		state->begin += e->header.size;
	}

	*msg = e;

	return 1;
}

void perf_event_read_finish(void *_header, void *_state)
{
	volatile struct perf_event_mmap_page *header = _header;
	struct read_state *state = _state;

	__sync_synchronize();
	header->data_tail = (uint64_t) state->head;
}

void cast(void *ptr, void *_dst)
{
	void **dst = (void **) _dst;
	*dst = ptr;
}

*/
import "C"

// perfType is the event we want from perf.
type perfType uint32

// These constants are linux ABI, defined as PERF_TYPE_* in <linux/perf_event.h>
const (
	perfTypeHardware perfType = iota
	perfTypeSoftware
	perfTypeTracePoint
	perfTypeHWCache
	perfTypeRaw
	perfTypeBreakpoint
)

// perfSample are the fields we want in the samples.
type perfSample uint64

const (
	perfSampleIP perfSample = 1 << iota
	perfSampleTID
	perfSampleTime
	perfSampleAddr
	perfSampleRead
	perfSampleCallchain
	perfSampleID
	perfSampleCPU
	perfSamplePeriod
	perfSampleStreamID
	perfSampleRaw
	perfSampleBranchStack
	perfSampleRegsUser
	perfSampleStackUser
	perfSampleWeight
	perfSampleDataSrc
	perfSampleIdentifier
	perfSampleTransaction
	perfSampleRegsIntr
)

// perfEventHeader is ABI, struct perf_event_header in <linux/perf_event.h>.
type perfEventHeader struct {
	kind      uint32
	misc      uint16
	totalSize uint16
}

// perfEventSample is ABI, struct perf_event_sample in kernel sources.
type perfEventSample struct {
	perfEventHeader
	size uint32
	data byte // First byte of data blob of size bytes
}

func (e *perfEventSample) DataDirect() []byte {
	// http://stackoverflow.com/questions/27532523/how-to-convert-1024c-char-to-1024byte
	return (*[1 << 30]byte)(unsafe.Pointer(&e.data))[:int(e.size):int(e.size)]
}

func (e *perfEventSample) DataCopy() []byte {
	return C.GoBytes(unsafe.Pointer(&e.data), C.int(e.size))
}

// perfEventLost is ABI, struct perf_event_lost in kernel sources.
type perfEventLost struct {
	perfEventHeader
	id   uint64
	lost uint64
}

type perfEventConfig struct {
	nCpus        int
	nPages       int
	eventType    perfType
	config       int
	sampleType   perfSample
	wakeupEvents int
}

type perfEvent struct {
	cpu      int
	fd       int
	pageSize int
	nPages   int
	lost     uint64
	unknown  uint64
	data     []byte
}

type perfReceiveFunc func(msg *perfEventSample, cpu int)
type perfLostFunc func(msg *perfEventLost, cpu int)

func perfEventOpen(config *perfEventConfig, pid int, cpu int, groupFD int, flags int) (*perfEvent, error) {
	attr := C.struct_perf_event_attr{}

	C.create_perf_event_attr(
		C.int(config.eventType),
		C.int(config.config),
		C.int(config.sampleType),
		C.int(config.wakeupEvents),
		unsafe.Pointer(&attr),
	)

	ret, _, err := unix.Syscall6(
		unix.SYS_PERF_EVENT_OPEN,
		uintptr(unsafe.Pointer(&attr)),
		uintptr(pid),
		uintptr(cpu),
		uintptr(groupFD),
		uintptr(flags), 0)

	if int(ret) > 0 && err == 0 {
		return &perfEvent{
			cpu: cpu,
			fd:  int(ret),
		}, nil
	}
	return nil, fmt.Errorf("Unable to open perf event: %s", err)
}

func (e *perfEvent) mmap(pageSize int, nPages int) error {
	size := pageSize * (nPages + 1)
	data, err := unix.Mmap(e.fd,
		0,
		size,
		unix.PROT_READ|unix.PROT_WRITE,
		unix.MAP_SHARED)

	if err != nil {
		return fmt.Errorf("Unable to mmap perf event: %s", err)
	}

	e.pageSize = pageSize
	e.nPages = nPages
	e.data = data

	return nil
}

func (e *perfEvent) enable() error {
	if err := unix.IoctlSetInt(e.fd, unix.PERF_EVENT_IOC_ENABLE, 0); err != nil {
		return fmt.Errorf("Unable to enable perf event: %v", err)
	}

	return nil
}

func (e *perfEvent) disable() error {
	if e == nil {
		return nil
	}

	if err := unix.IoctlSetInt(e.fd, unix.PERF_EVENT_IOC_DISABLE, 0); err != nil {
		return fmt.Errorf("Unable to disable perf event: %v", err)
	}

	return nil
}

func (e *perfEvent) read(receive perfReceiveFunc, lostFn perfLostFunc) {
	buf := make([]byte, 256)
	state := C.malloc(C.size_t(unsafe.Sizeof(C.struct_read_state{})))

	// Prepare for reading and check if events are available
	available := C.perf_event_read_init(C.int(e.nPages), C.int(e.pageSize),
		unsafe.Pointer(&e.data[0]), unsafe.Pointer(state))

	// Poll false positive
	if available == 0 {
		return
	}

	for {
		var msg *perfEventHeader

		if ok := C.perf_event_read(unsafe.Pointer(state),
			unsafe.Pointer(&buf[0]), unsafe.Pointer(&msg)); ok == 0 {
			break
		}

		if msg.kind == C.PERF_RECORD_SAMPLE {
			var sample *perfEventSample
			C.cast(unsafe.Pointer(msg), unsafe.Pointer(&sample))
			receive(sample, e.cpu)
		} else if msg.kind == C.PERF_RECORD_LOST {
			var lost *perfEventLost
			C.cast(unsafe.Pointer(msg), unsafe.Pointer(&lost))
			e.lost += lost.lost
			if lostFn != nil {
				lostFn(lost, e.cpu)
			}
		} else {
			e.unknown++
		}
	}

	// Move ring buffer tail pointer
	C.perf_event_read_finish(unsafe.Pointer(&e.data[0]), unsafe.Pointer(state))
	C.free(state)
}

func (e *perfEvent) close() {
	unix.Close(e.fd)
}
