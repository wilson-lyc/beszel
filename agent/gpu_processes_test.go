package agent

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGPUProcessParsing(t *testing.T) {
	rows, err := parseGPUProcesses(`# gpu pid type sm mem enc dec jpg ofa fb ccpm command
# Idx # C/G % % % % % % MB MB name
0 42 C 75 12 - - - - 2048 0 python
1 42 C 0 0 - - - - 0 0 python
0 99 G - - - - - - - - Xorg
0 42 C 75 12 - - - - 2048 0 python
2 - - - - - - - - - - -
`)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.EqualValues(t, 42, rows[0].PID)
	assert.Equal(t, "0", rows[0].GPU)
	require.NotNil(t, rows[0].Memory)
	assert.EqualValues(t, 2048*1024*1024, *rows[0].Memory)
	require.NotNil(t, rows[0].Usage)
	assert.Equal(t, 75.0, *rows[0].Usage)
	require.NotNil(t, rows[1].Memory)
	assert.Zero(t, *rows[1].Memory)
	require.NotNil(t, rows[1].Usage)
	assert.Zero(t, *rows[1].Usage)
	assert.Nil(t, rows[2].Memory)
	assert.Nil(t, rows[2].Usage)
}

func TestGPUProcessOlderDriverAndEmpty(t *testing.T) {
	rows, err := parseGPUProcesses("# gpu pid type sm mem enc dec fb command\n0 123 C 40 2 0 0 512 python\n")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.EqualValues(t, 512*1024*1024, *rows[0].Memory)
	rows, err = parseGPUProcesses("# gpu pid type sm mem enc dec fb command\n0 - - - - - - - -\n")
	require.NoError(t, err)
	assert.Empty(t, rows)
	for _, input := range []string{"", "Not supported", "# gpu pid type sm mem fb command\n0 bad C 1 1 1 x", "# gpu pid type sm mem fb command\n0 12 C 1"} {
		_, err := parseGPUProcesses(input)
		require.Error(t, err)
	}
}
