package obs

import (
	"golang.org/x/sys/unix"
)

const (
	// maxPollEvents is the maximum number of events a call to epoll_wait() can
	// return.
	maxPollEvents = 32
)

type epoll struct {
	fd     int
	nFds   int
	events [maxPollEvents]unix.EpollEvent
}

func (ep *epoll) init() (err error) {
	ep.fd, err = unix.EpollCreate1(0)
	return
}

func (ep *epoll) addFd(fd int, events uint32) error {
	ev := unix.EpollEvent{
		Events: events,
		Fd:     int32(fd),
	}

	return unix.EpollCtl(ep.fd, unix.EPOLL_CTL_ADD, fd, &ev)
}

func (ep *epoll) poll(timeout int) (int, error) {
	nFds, err := unix.EpollWait(ep.fd, ep.events[0:], timeout)
	if err != nil {
		return 0, err
	}

	ep.nFds = nFds

	return nFds, nil
}

func (ep *epoll) close() {
	unix.Close(ep.fd)
}
