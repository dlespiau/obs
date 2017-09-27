package main

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/dlespiau/obs"
)

func die(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func main() {
	exec := obs.NewTracepoint("sched:sched_process_exec")

	observer := obs.NewObserver()
	observer.AddTracepoint(exec)

	err := observer.Open()
	if err != nil {
		die(err)
	}

	for {
		event, err := observer.ReadEvent()
		if err != nil {
			die(err)
		}

		tp := event.(*obs.TracepointEvent)
		fmt.Println(hex.Dump(tp.Data()))
	}
}
