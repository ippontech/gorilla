package goshin

import (
	"fmt"
	"strings"
	"time"
)
import linuxproc "github.com/c9s/goprocinfo/linux"
import "github.com/tjgq/broadcast"

type DiskStats struct {
	last, actual           map[string]linuxproc.DiskStat
	lastTime, actualTime   time.Time
	ignoreDevices, devices map[string]bool
}

func (n *DiskStats) Store() {

	n.lastTime = n.actualTime

	// no copy for map on "=" with GO
	for k, v := range n.actual {
		n.last[k] = v
	}

	diskStats, _ := linuxproc.ReadDiskStats("/proc/diskstats")

	for _, diskStat := range diskStats {

		device := diskStat.Name

		if strings.HasPrefix(device, "ram") {
			continue
		}
		if strings.HasPrefix(device, "loop") {
			continue
		}
		// store new value
		n.actual[device] = diskStat
	}

	n.actualTime = time.Now()

}

func (n *DiskStats) buildMetric(device string, name string, value float64) *Metric {
	metric := NewMetric()
	metric.Service = fmt.Sprintf("diskstats %s %s", device, name)
	metric.Value = value

	return metric
}

func (n *DiskStats) candidateDevices() []string {

	keys := make([]string, 0, len(n.actual))

	for k, _ := range n.actual {
		_, include := n.devices[k]
		_, exclude := n.ignoreDevices[k]

		if len(n.devices) != 0 {
			if include && !exclude {
				keys = append(keys, k)
			}
		} else {
			if !exclude {
				keys = append(keys, k)
			}
		}

	}

	return keys
}
func (n *DiskStats) Collect(queue chan *Metric, listener *broadcast.Listener) {
	for {
		<-listener.Ch

		n.Store()

		// first run or
		// no interface
		if len(n.last) == 0 {
			continue
		}

		interval := float64(n.actualTime.Sub(n.lastTime).Seconds())

		for _, deviceName := range n.candidateDevices() {

			lastStat := n.last[deviceName]
			actualStat := n.actual[deviceName]

			queue <- n.buildMetric(deviceName, "reads reqs", computeRate(actualStat.ReadIOs, lastStat.ReadIOs, interval))
			queue <- n.buildMetric(deviceName, "reads merged", computeRate(actualStat.ReadMerges, lastStat.ReadMerges, interval))
			queue <- n.buildMetric(deviceName, "reads sector", computeRate(actualStat.ReadSectors, lastStat.ReadSectors, interval))
			queue <- n.buildMetric(deviceName, "reads time", computeRate(actualStat.ReadTicks, lastStat.ReadTicks, interval))
			queue <- n.buildMetric(deviceName, "writes reqs", computeRate(actualStat.WriteIOs, lastStat.WriteIOs, interval))
			queue <- n.buildMetric(deviceName, "writes merged", computeRate(actualStat.WriteMerges, lastStat.WriteMerges, interval))
			queue <- n.buildMetric(deviceName, "writes sector", computeRate(actualStat.WriteSectors, lastStat.WriteSectors, interval))
			queue <- n.buildMetric(deviceName, "writes time", computeRate(actualStat.WriteTicks, lastStat.WriteTicks, interval))
			queue <- n.buildMetric(deviceName, "io reqs", float64(actualStat.InFlight))
			queue <- n.buildMetric(deviceName, "io time", computeRate(actualStat.IOTicks, lastStat.IOTicks, interval))
			queue <- n.buildMetric(deviceName, "io weighted", computeRate(actualStat.TimeInQueue, lastStat.TimeInQueue, interval))
		}
	}
}

func computeRate(actual uint64, last uint64, interval float64) float64 {
	diff := int64(actual - last)

	if diff > 0 {
		return float64(diff) / interval
	} else {
		return float64(-diff) / interval
	}
}

// Act as constructor
func NewDiskStats(devices, ignoreDevices map[string]bool) *DiskStats {
	return &DiskStats{
		last:          make(map[string]linuxproc.DiskStat),
		actual:        make(map[string]linuxproc.DiskStat),
		ignoreDevices: ignoreDevices,
		devices:       devices,
	}
}
