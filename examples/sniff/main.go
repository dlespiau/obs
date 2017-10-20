package main

import (
	"fmt"
	"os"

	"github.com/dlespiau/obs"
)

func die(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func main() {
	observer := obs.NewObserver()
	exec := observer.AddTracepoint("sched:sched_process_exec")

	err := observer.Open()
	if err != nil {
		die(err)
	}

	for {
		event, err := observer.ReadEvent()
		if err != nil {
			die(err)
		}

		switch source := event.GetSource(); source {
		case exec:
			tp := event.(*obs.TracepointEvent)
			fmt.Printf("exec\t%d\n", tp.GetInt("pid"))
		default:
			fmt.Fprintf(os.Stderr, "Unknown event source: %d", source)
		}

	}
}
