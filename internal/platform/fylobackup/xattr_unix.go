//go:build !windows

package fylobackup

import (
	"bytes"
	"errors"
	"strings"

	"golang.org/x/sys/unix"
)

func readFYLOAttributes(path string) (map[string]string, error) {
	size, err := unix.Listxattr(path, nil)
	if err != nil {
		if errors.Is(err, unix.ENOTSUP) {
			return nil, nil
		}
		return nil, err
	}
	if size == 0 {
		return nil, nil
	}
	names := make([]byte, size)
	size, err = unix.Listxattr(path, names)
	if err != nil {
		return nil, err
	}
	attributes := map[string]string{}
	for _, rawName := range bytes.Split(names[:size], []byte{0}) {
		name := string(rawName)
		if name == "" || !strings.HasPrefix(name, "user.fylo.") {
			continue
		}
		valueSize, err := unix.Getxattr(path, name, nil)
		if err != nil {
			return nil, err
		}
		value := make([]byte, valueSize)
		if valueSize > 0 {
			if _, err := unix.Getxattr(path, name, value); err != nil {
				return nil, err
			}
		}
		attributes[name] = encodeAttribute(value)
	}
	if len(attributes) == 0 {
		return nil, nil
	}
	return attributes, nil
}

func writeFYLOAttributes(path string, attributes map[string]string) error {
	for name, encoded := range attributes {
		if !strings.HasPrefix(name, "user.fylo.") {
			return errors.New("backup contains a non-FYLO extended attribute")
		}
		value, err := decodeAttribute(encoded)
		if err != nil {
			return err
		}
		if err := unix.Setxattr(path, name, value, 0); err != nil {
			return err
		}
	}
	return nil
}
