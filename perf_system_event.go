package obs

import (
	"os"

	"golang.org/x/sys/unix"
)

// perfSystemEvent is a system-wide event. perf doesn't allow a single event,
// one opened with perf_event_open(), to be both: for all pids and for all cpus.
// perfSystemEvent abstract that detail away, creating a perfEvent listening for
// all PIDs for each online CPU.
type perfSystemEvent struct {
	cpus      int
	nPages    int
	pageSize  int
	fdToEvent map[int]*perfEvent
	epoll     epoll
}

func newPerfSystemEvent(config *perfEventConfig) (*perfSystemEvent, error) {
	var err error

	e := &perfSystemEvent{
		cpus:      config.nCpus,
		nPages:    config.nPages,
		pageSize:  os.Getpagesize(),
		fdToEvent: make(map[int]*perfEvent),
	}

	defer func() {
		if err != nil {
			e.close()
		}
	}()

	if err = e.epoll.init(); err != nil {
		return nil, err
	}

	for cpu := int(0); cpu < e.cpus; cpu++ {
		event, err := perfEventOpen(config, -1, cpu, -1, 0)
		if err != nil {
			return nil, err
		}
		e.fdToEvent[event.fd] = event

		if err = e.epoll.addFd(event.fd, unix.EPOLLIN); err != nil {
			return nil, err
		}

		if err = event.mmap(e.pageSize, e.nPages); err != nil {
			return nil, err
		}

		if err = event.enable(); err != nil {
			return nil, err
		}
	}

	return e, nil
}

func (e *perfSystemEvent) poll(timeout int) (int, error) {
	return e.epoll.poll(timeout)
}

func (e *perfSystemEvent) read(receive perfReceiveFunc, lost perfLostFunc) error {
	for i := 0; i < e.epoll.nFds; i++ {
		fd := int(e.epoll.events[i].Fd)
		if event, ok := e.fdToEvent[fd]; ok {
			event.read(receive, lost)
		}
	}

	return nil
}

func (e *perfSystemEvent) stats() (uint64, uint64) {
	var lost, unknown uint64

	for _, event := range e.fdToEvent {
		lost += event.lost
		unknown += event.unknown
	}

	return lost, unknown
}

func (e *perfSystemEvent) close() error {
	var retErr error

	e.epoll.close()

	for _, event := range e.fdToEvent {
		if err := event.disable(); err != nil {
			retErr = err
		}

		event.close()
	}

	return retErr
}
