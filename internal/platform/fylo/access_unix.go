//go:build !windows

package fylo

import "os"

func currentAccessIDs() (int, int) {
	return os.Getuid(), os.Getgid()
}
