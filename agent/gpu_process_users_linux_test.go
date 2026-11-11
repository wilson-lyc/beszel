package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/shirou/gopsutil/v4/process"
	"github.com/stretchr/testify/require"
)

func TestGPUProcessOwnerFromHostProc(t *testing.T) {
	proc, etc := t.TempDir(), t.TempDir()
	t.Setenv("HOST_PROC", proc)
	t.Setenv("HOST_ETC", etc)
	require.NoError(t, os.Mkdir(filepath.Join(proc, "42"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(proc, "42", "status"), []byte("Name:\tpython\nUid:\t12345\t0\t0\t0\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(etc, "passwd"), []byte("alice:x:12345:12345::/:/bin/sh\n"), 0600))
	p := &process.Process{Pid: 42}
	require.Equal(t, "alice", newGPUProcessUsers().username(context.Background(), p))
}
