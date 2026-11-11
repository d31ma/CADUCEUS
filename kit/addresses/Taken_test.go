package addresses

import "testing"

func TestTakenNamesTheLocalPart(t *testing.T) {
	assertText(t, Taken{LocalPart: "ana"}.Error(), "the local part ana is already in use")
}
