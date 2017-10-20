# Linux Observability Primitives

Very grand title for a humble start. This repository contains my experiments
with low level Linux observability, in Go.

If you are interested in the domain, you should probably check out [bcc](
https://github.com/iovisor/bcc) and [gobpf]( https://github.com/iovisor/gobpf).
They support [eBPF](http://www.brendangregg.com/ebpf.html). I may integrate
eBPF support at some point, either through gobpf of directly, but that's a
whole different story.

## Tracepoints

Tracepoints are static probes placed at critical points in the Linux kernel.
There are some tracepoints for many interesting events:

- Scheduler: when does a process starts & exits, context switches.
- I/O: follow block requests.
- Filesystem: intimate details about filesystem operations (ext4, btrfs, ... are instrumented).
- KVM: watch what KVM does, eg. VM exits.
- Drivers: eg. on can follow Intel GPU execution requests and display events (i915 driver).
- Syscall entry/exit.
- Many other events and Various drivers.

To see the full list of tracepoints:

```shell
$ sudo perf list tracepoint
[...]
  sched:sched_kthread_stop                           [Tracepoint event]
  sched:sched_kthread_stop_ret                       [Tracepoint event]
  sched:sched_migrate_task                           [Tracepoint event]
  sched:sched_move_numa                              [Tracepoint event]
  sched:sched_process_exec                           [Tracepoint event]
  sched:sched_process_exit                           [Tracepoint event]
  sched:sched_process_fork                           [Tracepoint event]
  sched:sched_process_free                           [Tracepoint event]
  sched:sched_process_hang                           [Tracepoint event]
  sched:sched_process_wait                           [Tracepoint event]
[...
```

### Example

```go
// Listen for the sched_process_exec, triggered when a process does an exec(),
// and display the process pid and the path of binary being exec'ed. Error
// handling is omitted.
observer := obs.NewObserver()
observer.AddTracepoint("sched:sched_process_exec")
observer.Open()

for {
  event, _ := observer.ReadEvent()
  tp := event.(*obs.TracepointEvent)
  fmt.Printf("exec\t%d\t%s\n", tp.GetInt("pid"), tp.GetString("filename"))
}
```

The output below is the result of running `docker run busybox sleep 3600` and
shows the (surprisingly big) list programs that end up being executed up to
`runc` and the `sleep` process in the container.

```
exec	7364	/usr/bin/docker
exec	7370	/sbin/auplink
exec	7371	/sbin/auplink
exec	7378	/lib/udev/ifupdown-hotplug
exec	7380	/bin/grep
exec	7381	/sbin/ifquery
exec	7379	/sbin/ifquery
exec	7382	/lib/udev/ifupdown-hotplug
exec	7383	/lib/systemd/systemd-sysctl
exec	7385	/bin/grep
exec	7386	/sbin/ifquery
exec	7384	/sbin/ifquery
exec	7387	/lib/systemd/systemd-sysctl
exec	7389	/usr/bin/docker-containerd-shim
exec	7397	/usr/bin/docker-runc
exec	7404	/proc/self/exe
exec	7413	/usr/bin/dockerd
exec	7426	/proc/self/exe
exec	7433	/lib/udev/ifupdown-hotplug
exec	7434	/usr/bin/docker-runc
exec	7407	/bin/sleep
```
