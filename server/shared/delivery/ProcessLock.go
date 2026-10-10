import (
	"os"
	"path/filepath"
	"syscall"
)

// ProcessLock serializes one kind of read-then-write across the server's
// processes. Tachyon runs every request in its own process, so the
// JavaScript server's in-process promise chains (one writer serializing its
// own ticks and events) become an exclusive lock on a file named after the
// FYLO root and the lock's Name.
type ProcessLock struct {
	Name string
}

// Held runs work while holding the lock. When the lock file cannot be used
// the work still runs, unserialized, as it would have on another host.
func (lock ProcessLock) Held(work func() error) error {
	file, err := os.OpenFile(lock.path(), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return work()
	}
	defer file.Close()
	if syscall.Flock(int(file.Fd()), syscall.LOCK_EX) != nil {
		return work()
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	return work()
}

func (lock ProcessLock) path() string {
	root := Security{}.Sha256Hex(Settings{}.Value("FYLO_ROOT"))
	return filepath.Join(os.TempDir(), "caduceus-"+root[:16]+"-"+lock.Name+".lock")
}
