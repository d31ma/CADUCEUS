package blobs

import (
	"errors"
	"strings"
	"testing"
)

func TestInspectMeasuresHashesAndSniffs(t *testing.T) {
	inspection, err := Inspector{}.Inspect(strings.NewReader("%PDF-1.7 hello"), 100)
	if err != nil || inspection.Size != 14 || inspection.SniffedType != "application/pdf" {
		t.Fatalf("got %+v, %v", inspection, err)
	}
}

func TestInspectHashesTheWholeBody(t *testing.T) {
	inspection, _ := Inspector{}.Inspect(strings.NewReader("abc"), 3)
	if inspection.Sha256 != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("sha %q", inspection.Sha256)
	}
}

func TestInspectRefusesABodyOverTheLimit(t *testing.T) {
	var large TooLarge
	if _, err := (Inspector{}).Inspect(strings.NewReader("abcd"), 3); !errors.As(err, &large) || large.Limit != 3 {
		t.Fatalf("got %v", err)
	}
}

func TestInspectPassesOnReadErrors(t *testing.T) {
	if _, err := (Inspector{}).Inspect(failingReader{}, 3); err == nil || err.Error() != "disk gone" {
		t.Fatalf("got %v", err)
	}
}

func TestHeadBufferKeepsOnlyTheFirstBytes(t *testing.T) {
	head := &HeadBuffer{capacity: 4}
	head.Write([]byte("abc"))
	written, _ := head.Write([]byte("defg"))
	if string(head.Bytes()) != "abcd" || written != 4 {
		t.Fatalf("got %q, %d", head.Bytes(), written)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("disk gone")
}
