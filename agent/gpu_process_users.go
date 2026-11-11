package agent

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type gpuUserProcess interface {
	UidsWithContext(context.Context) ([]uint32, error)
	UsernameWithContext(context.Context) (string, error)
}

// A host passwd file is authoritative: container accounts can use the same UID
// for a different person. Reload each snapshot to pick up account changes.
type gpuProcessUsers struct {
	host  bool
	names map[uint32]string
}

func newGPUProcessUsers() gpuProcessUsers {
	users := gpuProcessUsers{names: make(map[uint32]string)}
	if etc := os.Getenv("HOST_ETC"); etc != "" {
		users.host = true
		if f, err := os.Open(filepath.Join(etc, "passwd")); err == nil {
			defer f.Close()
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				fields := strings.Split(scanner.Text(), ":")
				if len(fields) != 7 || fields[0] == "" || strings.HasPrefix(fields[0], "#") {
					continue
				}
				if uid, err := strconv.ParseUint(fields[2], 10, 32); err == nil {
					if _, exists := users.names[uint32(uid)]; !exists {
						users.names[uint32(uid)] = fields[0]
					}
				}
			}
		}
	}
	return users
}

func (users gpuProcessUsers) username(ctx context.Context, p gpuUserProcess) string {
	uids, err := p.UidsWithContext(ctx)
	if err != nil || len(uids) == 0 {
		return ""
	}
	// Use the real UID, matching gopsutil's process owner semantics.
	uid := uids[0]
	if name, ok := users.names[uid]; ok {
		return name
	}
	if !users.host {
		if name, err := p.UsernameWithContext(ctx); err == nil && name != "" {
			users.names[uid] = name
			return name
		}
	}
	// Keep unknown accounts identifiable without inventing a username.
	return "UID " + strconv.FormatUint(uint64(uid), 10)
}
