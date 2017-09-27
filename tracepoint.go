package obs

import (
	"io/ioutil"
	"runtime"
	"strconv"
	"strings"
)

const (
	traceRoot = "/sys/kernel/debug/tracing/events"
)

// Tracepoint is a linux tracepoint, a static probe placed at compile time in
// the linux kernel.
type Tracepoint struct {
	// Name is the name of the tracepoint.
	Name string

	// underlying perf events
	perf *perfSystemEvent
}

// NewTracepoint creates a Tracepoint. Name is the tracepoint name as listed by:
//
//   $ sudo perf list tracepoint
func NewTracepoint(name string) *Tracepoint {
	return &Tracepoint{
		Name: name,
	}
}

func (tp *Tracepoint) open() error {
	var err error

	// Start by retrieving the event id.
	idBytes, err := ioutil.ReadFile(traceRoot + "/" + strings.Replace(tp.Name, ":", "/", 1) + "/id")
	if err != nil {
		return err
	}
	// Convert to int, don't forget to strip the final '\n'
	id, err := strconv.Atoi(string(idBytes[0 : len(idBytes)-1]))
	if err != nil {
		return err
	}

	config := perfEventConfig{
		eventType:  perfTypeTracePoint,
		sampleType: perfSampleRaw,
		config:     id,

		// TODO(damien): Use online CPUs. System event should fill that for us.
		nCpus: runtime.NumCPU(),
		// TODO(damien): the user should be able to ignore the config params below and
		// get sensible defaults.
		nPages:       8,
		wakeupEvents: 1,
	}
	tp.perf, err = newPerfSystemEvent(&config)

	return err
}

func (tp *Tracepoint) close() {
	if tp.perf != nil {
		tp.perf.close()
	}
}
