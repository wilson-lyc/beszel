package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type gpuUserFixture struct {
	uids []uint32
	name string
	err  error
}

func (p gpuUserFixture) UidsWithContext(context.Context) ([]uint32, error)   { return p.uids, p.err }
func (p gpuUserFixture) UsernameWithContext(context.Context) (string, error) { return p.name, p.err }

func TestGPUProcessUsersHostMapping(t *testing.T) {
	etc := t.TempDir()
	t.Setenv("HOST_ETC", etc)
	require.NoError(t, os.WriteFile(filepath.Join(etc, "passwd"), []byte("# ignored\nbroken\nalice:x:1000:1000::/home/alice:/bin/sh\nroot:x:0:0::/root:/bin/sh\nbad:x:invalid:1::/:/bin/sh\n"), 0600))
	users := newGPUProcessUsers()
	ctx := context.Background()
	// Host ownership wins over a conflicting container account and effective UID.
	require.Equal(t, "alice", users.username(ctx, gpuUserFixture{uids: []uint32{1000, 0}, name: "container-user"}))
	require.Equal(t, "root", users.username(ctx, gpuUserFixture{uids: []uint32{0}}))
	require.Equal(t, "UID 1001", users.username(ctx, gpuUserFixture{uids: []uint32{1001}, name: "wrong-container-user"}))
	// Refresh account mappings on the next snapshot.
	require.NoError(t, os.WriteFile(filepath.Join(etc, "passwd"), []byte("bob:x:1000:1000::/:/bin/sh\n"), 0600))
	require.Equal(t, "bob", newGPUProcessUsers().username(ctx, gpuUserFixture{uids: []uint32{1000}}))
}

func TestGPUProcessUsersFallback(t *testing.T) {
	t.Setenv("HOST_ETC", "")
	users := newGPUProcessUsers()
	ctx := context.Background()
	require.Equal(t, "alice", users.username(ctx, gpuUserFixture{uids: []uint32{1000}, name: "alice"}))
	require.Equal(t, "UID 1001", users.username(ctx, gpuUserFixture{uids: []uint32{1001}}))
	require.Empty(t, users.username(ctx, gpuUserFixture{}))
	require.Empty(t, users.username(ctx, gpuUserFixture{err: errors.New("process disappeared")}))
	t.Setenv("HOST_ETC", t.TempDir())
	require.Equal(t, "UID 1000", newGPUProcessUsers().username(ctx, gpuUserFixture{uids: []uint32{1000}, name: "wrong-container-user"}))
}
