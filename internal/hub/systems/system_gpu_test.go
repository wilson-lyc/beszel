//go:build testing

package systems

import (
	"testing"

	"github.com/henrygd/beszel/internal/entities/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateRecordsGPUUtilization(t *testing.T) {
	sys, app := newTestSystemWithHub(t)
	for _, tc := range []struct {
		name  string
		gpu   bool
		usage float64
	}{
		{"no GPU", false, 0},
		{"active GPU", true, 42.5},
		{"idle GPU", true, 0},
		{"GPU removed", false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := &system.CombinedData{Info: system.Info{GpuPct: tc.usage, Cpu: 12.5}}
			if tc.gpu {
				data.Stats.GPUData = map[string]system.GPUData{"0": {Name: "GPU", Usage: tc.usage}}
			}
			_, err := sys.createRecords(data)
			require.NoError(t, err)
			record, err := app.FindRecordById("systems", sys.Id)
			require.NoError(t, err)
			var info map[string]any
			require.NoError(t, record.UnmarshalJSONField("info", &info))
			assert.Equal(t, 12.5, info["cpu"])
			if tc.gpu {
				assert.Equal(t, tc.usage, info["g"])
			} else {
				assert.NotContains(t, info, "g")
			}
		})
	}
}

func TestCreateRecordsGPUMemory(t *testing.T) {
	sys, app := newTestSystemWithHub(t)
	for _, tc := range []struct {
		name string
		gpus map[string]system.GPUData
		want *float64
	}{
		{"no GPU", nil, nil},
		{"unknown capacity", map[string]system.GPUData{"0": {MemoryUsed: 100}}, nil},
		{"active", map[string]system.GPUData{"0": {MemoryUsed: 2048, MemoryTotal: 8192}}, new(25.0)},
		{"idle", map[string]system.GPUData{"0": {MemoryTotal: 8192}}, new(0.0)},
		{"multiple GPUs", map[string]system.GPUData{
			"0": {MemoryUsed: 1024, MemoryTotal: 4096},
			"1": {MemoryUsed: 5120, MemoryTotal: 8192},
			"2": {MemoryUsed: 100},
		}, new(50.0)},
		{"GPU removed", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := &system.CombinedData{Stats: system.Stats{GPUData: tc.gpus}}
			_, err := sys.createRecords(data)
			require.NoError(t, err)
			record, err := app.FindRecordById("systems", sys.Id)
			require.NoError(t, err)
			var info map[string]any
			require.NoError(t, record.UnmarshalJSONField("info", &info))
			if tc.want == nil {
				assert.NotContains(t, info, "gm")
			} else {
				assert.Equal(t, *tc.want, info["gm"])
			}
		})
	}
}

func TestCreateRecordsGPUProcessesLiveOnly(t *testing.T) {
	sys, app := newTestSystemWithHub(t)
	data := &system.CombinedData{Stats: system.Stats{GPUProcesses: &system.GPUProcessSnapshot{
		Status: "ok", Updated: 1234, Processes: []system.GPUProcess{{PID: 42, GPU: "0", User: "alice", Name: "python"}},
	}}}
	_, err := sys.createRecords(data)
	require.NoError(t, err)
	// The caller still owns the live snapshot, including when marshalled for realtime.
	require.NotNil(t, data.Stats.GPUProcesses)
	payload, err := marshalRealtimeData(data)
	require.NoError(t, err)
	assert.Contains(t, string(payload), `"user":"alice"`)
	// Neither the history record nor homepage info may retain process identities.
	record, err := app.FindFirstRecordByFilter("system_stats", "system={:system}", map[string]any{"system": sys.Id})
	require.NoError(t, err)
	var stats map[string]any
	require.NoError(t, record.UnmarshalJSONField("stats", &stats))
	assert.NotContains(t, stats, "gp")
	systemRecord, err := app.FindRecordById("systems", sys.Id)
	require.NoError(t, err)
	var info map[string]any
	require.NoError(t, systemRecord.UnmarshalJSONField("info", &info))
	assert.NotContains(t, info, "gp")
}
