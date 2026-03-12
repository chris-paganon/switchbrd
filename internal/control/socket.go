package control

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

func SocketPath() string {
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		return filepath.Join(runtimeDir, "switchbrd.sock")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("switchbrd-%s.sock", strconv.Itoa(os.Getuid())))
}
