package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	stats "github.com/shirou/gopsutil/v4/process"
)

type Process struct {
	processName string
	processPID  int32
	cpuValues   sort.Float64Slice
	memValues   sort.Float64Slice
	process     *stats.Process
}

type ProcessAggregations struct {
	processMap map[string]*Process
	mu         sync.Mutex
}

func InitProcessAggregations(processName []string) *ProcessAggregations {
	fmt.Printf("Starting process aggregations")

	processAggregationsMap := make(map[string]*Process)
	for _, processName := range processName {
		pids, err := findPIDFromExe(processName)
		if err != nil || len(pids) == 0 {
			fmt.Printf("Error getting PID for process %s\n", processName)
			continue
		}

		process, err := stats.NewProcess(pids[0])
		if err != nil {
			fmt.Printf("Error tracking process %s\n", processName)
			continue
		}

		p := Process{
			processName: processName,
			processPID:  pids[0],
			process:     process,
		}

		processAggregationsMap[processName] = &p
	}

	return &ProcessAggregations{
		processMap: processAggregationsMap,
	}
}

func (pa *ProcessAggregations) Run() {
	go pa.CollectStats()
	go pa.SendToAppInsights()
}

func (pa *ProcessAggregations) CollectStats() {
	ticker := time.NewTicker(time.Second * time.Duration(10))
	for ; true; <-ticker.C {
		pa.mu.Lock()

		for _, p := range pa.processMap {
			cpu, err := p.process.Percent(0)
			if err == nil {
				p.cpuValues = append(p.cpuValues, cpu)
				p.cpuValues.Sort()
			}
			mem, err := p.process.MemoryPercent()
			if err == nil {
				p.memValues = append(p.memValues, float64(mem))
				p.memValues.Sort()
			}

			fmt.Printf("cpu: %f, mem: %f\n", cpu, mem)
		}

		pa.mu.Unlock()
	}
}

func (pa *ProcessAggregations) SendToAppInsights() {
	ticker := time.NewTicker(time.Second * time.Duration(300))
	for ; true; <-ticker.C {
		pa.mu.Lock()

		for processName, p := range pa.processMap {
			for _, percentile := range []int{50, 95} {
				if len(p.cpuValues) > 0 {
					cpuMetric := appinsights.NewMetricTelemetry(
						fmt.Sprintf("fluent_%s_cpu_usage_0%d", strings.ToLower(processName), percentile),
						float64(p.cpuValues[int(math.Round(float64(len(p.cpuValues)-1)*float64(percentile)/100.0))]),
					)
					fmt.Printf("cpuMetric: %v\n", cpuMetric)
					fmt.Printf("cpuValues: %v\n", p.cpuValues)
					fmt.Printf("index: %d\n", int(math.Round(float64(len(p.cpuValues)-1)*float64(percentile)/100.0)))
					TelemetryClient.Track(cpuMetric)
				}

				if len(p.memValues) > 0 {
					memMetric := appinsights.NewMetricTelemetry(
						fmt.Sprintf("fluent_%s_memory_rss_0%d", strings.ToLower(processName), percentile),
						float64(p.memValues[int(math.Round(float64(len(p.memValues)-1)*float64(percentile)/100.0))]),
					)
					fmt.Printf("memMetric: %v\n", memMetric)
					fmt.Printf("memValues: %v\n", p.memValues)
					fmt.Printf("index: %d\n", int(math.Round(float64(len(p.memValues)-1)*float64(percentile)/100.0)))
					TelemetryClient.Track(memMetric)
				}
			}

			// Clear values for next aggregation period
			p.cpuValues = sort.Float64Slice{}
			p.memValues = sort.Float64Slice{}
		}

		pa.mu.Unlock()
	}
}
