package obs

import (
	"io/ioutil"
	"strconv"
	"strings"
)

const (
	onlineCPUs = "/sys/devices/system/cpu/online"
)

func parseOnlineCPUs(s string) ([]int, error) {
	var cpus []int

	parts := strings.Split(s, ",")
	for _, part := range parts {
		hyphen := strings.IndexByte(part, '-')
		if hyphen == -1 {
			// part is just a cpu index: "3"
			cpu, err := strconv.Atoi(part)
			if err != nil {
				return nil, err
			}
			cpus = append(cpus, cpu)
			continue
		}

		// part is a range: "0-3"
		a, err := strconv.Atoi(part[:hyphen])
		if err != nil {
			return nil, err
		}
		b, err := strconv.Atoi(part[hyphen+1:])
		if err != nil {
			return nil, err
		}
		for cpu := a; cpu <= b; cpu++ {
			cpus = append(cpus, cpu)
		}
	}

	return cpus, nil
}

// getOnlineCPUs returns the exploded list of online CPUs. Each element of that
// list is a cpu index that can be given to perf_event_open()
func getOnlineCPUs() ([]int, error) {
	online, err := ioutil.ReadFile(onlineCPUs)
	if err != nil {
		return nil, err
	}

	return parseOnlineCPUs(string(online))
}
