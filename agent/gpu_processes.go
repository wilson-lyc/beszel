package agent

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/henrygd/beszel/internal/entities/system"
	"github.com/shirou/gopsutil/v4/process"
)

type processCPUSample struct {
	created int64
	total   float64
	at      time.Time
}

type gpuProcessCollector struct {
	sync.RWMutex
	snapshot *system.GPUProcessSnapshot
	cpu      map[int32]processCPUSample
}

// GetProcessData starts a separate sampler so commands never block metrics requests.
// Published snapshots are immutable and can safely be shared by cache intervals.
func (gm *GPUManager) GetProcessData() *system.GPUProcessSnapshot {
	gm.processOnce.Do(func() {
		gm.processCollector = &gpuProcessCollector{cpu: make(map[int32]processCPUSample)}
		go gm.processCollector.run()
	})
	c := gm.processCollector
	c.RLock()
	defer c.RUnlock()
	return c.snapshot
}

func (c *gpuProcessCollector) run() {
	if runtime.GOOS != "linux" {
		c.publish(&system.GPUProcessSnapshot{Status: "unsupported", Updated: time.Now().UnixMilli()})
		return
	}
	for {
		c.publish(c.collect())
		time.Sleep(5 * time.Second)
	}
}

func (c *gpuProcessCollector) publish(snapshot *system.GPUProcessSnapshot) {
	c.Lock()
	c.snapshot = snapshot
	c.Unlock()
}

// Bound command output as well as execution time, including broken drivers.
type processOutput struct{ bytes.Buffer }

func (b *processOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 2*1024*1024 {
		return 0, fmt.Errorf("GPU process output exceeds limit")
	}
	return b.Buffer.Write(p)
}

func (c *gpuProcessCollector) collect() *system.GPUProcessSnapshot {
	snapshot := &system.GPUProcessSnapshot{Status: "unavailable", Updated: time.Now().UnixMilli()}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, nvidiaSmiCmd, "pmon", "-s", "um", "-c", "1")
	var output processOutput
	cmd.Stdout = &output
	if err := cmd.Run(); err != nil {
		c.cpu = make(map[int32]processCPUSample)
		return snapshot
	}
	rows, err := parseGPUProcesses(output.String())
	if err != nil {
		c.cpu = make(map[int32]processCPUSample)
		return snapshot
	}
	now := time.Now()
	nextCPU := make(map[int32]processCPUSample)
	// A process using several GPUs has one CPU/RSS sample, repeated on its GPU rows.
	metadata := make(map[int32]system.GPUProcess)
	users := newGPUProcessUsers()
	for i := range rows {
		row := &rows[i]
		if cached, ok := metadata[row.PID]; ok {
			row.User, row.Name, row.CPU, row.RSS = cached.User, cached.Name, cached.CPU, cached.RSS
			continue
		}
		if p, err := process.NewProcessWithContext(ctx, row.PID); err == nil {
			row.User = users.username(ctx, p)
			if name, err := p.NameWithContext(ctx); err == nil && name != "" {
				row.Name = name
			}
			if memory, err := p.MemoryInfoWithContext(ctx); err == nil {
				row.RSS = &memory.RSS
			}
			created, createErr := p.CreateTimeWithContext(ctx)
			if times, err := p.TimesWithContext(ctx); err == nil && createErr == nil {
				total := times.User + times.System
				if prev, ok := c.cpu[row.PID]; ok && prev.created == created && total >= prev.total && now.After(prev.at) {
					usage := (total - prev.total) / now.Sub(prev.at).Seconds() * 100
					row.CPU = &usage
				}
				nextCPU[row.PID] = processCPUSample{created: created, total: total, at: now}
			}
		}
		metadata[row.PID] = *row
	}
	c.cpu = nextCPU
	snapshot.Status, snapshot.Updated, snapshot.Processes = "ok", now.UnixMilli(), rows
	return snapshot
}

// Parse header names rather than fixed offsets: driver versions add jpg/ofa/ccpm columns.
func parseGPUProcesses(output string) ([]system.GPUProcess, error) {
	rows := make([]system.GPUProcess, 0)
	var header map[string]int
	seen := make(map[string]bool)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "#" {
			fields = fields[1:]
			if len(fields) > 1 && fields[0] == "gpu" && fields[1] == "pid" {
				header = make(map[string]int)
				for i, name := range fields {
					header[name] = i
				}
			}
			continue
		}
		if header == nil {
			return nil, fmt.Errorf("missing pmon header")
		}
		get := func(name string) string {
			if i, ok := header[name]; ok && i < len(fields) {
				return fields[i]
			}
			return ""
		}
		if get("pid") == "-" {
			continue
		}
		pid, err := strconv.ParseInt(get("pid"), 10, 32)
		if err != nil || pid <= 0 {
			return nil, fmt.Errorf("invalid pmon PID")
		}
		gpu := get("gpu")
		if _, err := strconv.ParseUint(gpu, 10, 32); err != nil {
			return nil, fmt.Errorf("invalid pmon GPU")
		}
		key := gpu + ":" + get("pid")
		if seen[key] {
			continue
		}
		seen[key] = true
		row := system.GPUProcess{GPU: gpu, PID: int32(pid), Name: get("command")}
		if row.Name == "" {
			return nil, fmt.Errorf("missing pmon command")
		}
		if usage, err := strconv.ParseFloat(get("sm"), 64); err == nil && usage >= 0 && usage <= 100 {
			row.Usage = &usage
		}
		if memory, err := strconv.ParseUint(get("fb"), 10, 40); err == nil {
			value := memory * 1024 * 1024
			row.Memory = &value
		}
		rows = append(rows, row)
	}
	if header == nil {
		return nil, fmt.Errorf("missing pmon header")
	}
	return rows, nil
}
