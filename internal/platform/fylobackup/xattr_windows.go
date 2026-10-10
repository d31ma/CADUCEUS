//go:build windows

package fylobackup

import "errors"

func readFYLOAttributes(string) (map[string]string, error) {
	return nil, errors.New("FYLO local backup is not supported on Windows in Phase 2")
}

func writeFYLOAttributes(string, map[string]string) error {
	return errors.New("FYLO local restore is not supported on Windows in Phase 2")
}
