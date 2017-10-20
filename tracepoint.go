package obs

import (
	"io/ioutil"
	"runtime"
	"strconv"
	"strings"
)

// TODO(damien): find where debugfs is mounted

const (
	tracingRoot = "/sys/kernel/debug/tracing"
)

// tracepoint is a linux tracepoint, a static probe placed at compile time in
// the linux kernel.
type tracepoint struct {
	// Name is the name of the tracepoint.
	Name string

	// raw data format, from tracingRoot/events/**/**/format. This is used to
	// decode the raw data incoming from perf events.
	format format

	// underlying perf events
	perf *perfSystemEvent
}

// newTracepoint creates a Tracepoint. Name is the tracepoint name as listed by:
//
//   $ sudo perf list tracepoint
func newTracepoint(name string) *tracepoint {
	return &tracepoint{
		Name: name,
	}
}

func (tp *tracepoint) open() error {
	var err error

	tpPath := tracingRoot + "/events/" + strings.Replace(tp.Name, ":", "/", 1)

	// Start by retrieving the event id.
	idBytes, err := ioutil.ReadFile(tpPath + "/id")
	if err != nil {
		return err
	}
	// Convert to int, don't forget to strip the final '\n'
	id, err := strconv.Atoi(string(idBytes[0 : len(idBytes)-1]))
	if err != nil {
		return err
	}

	// Grab the event format.
	if err := tp.format.initFromFile(tpPath + "/format"); err != nil {
		return err
	}

	// Finally, configure perf to receive events.
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

func (tp *tracepoint) close() {
	if tp.perf != nil {
		tp.perf.close()
	}
}
