package imapwire

import "testing"

func TestMailboxModifiedUTF7RoundTrip(t *testing.T) {
	tests := map[string]string{
		"Inbox":          "Inbox",
		"R&D":            "R&-D",
		"日本語":            "&ZeVnLIqe-",
		"台北/日本語":         "&U,BTFw-/&ZeVnLIqe-",
		"Projects/Crème": "Projects/Cr&AOg-me",
	}
	for plain, wire := range tests {
		if got := EncodeMailbox(plain); got != wire {
			t.Errorf("EncodeMailbox(%q) = %q, want %q", plain, got, wire)
		}
		got, err := DecodeMailbox(wire)
		if err != nil || got != plain {
			t.Errorf("DecodeMailbox(%q) = %q, %v, want %q", wire, got, err, plain)
		}
	}
}

func TestDecodeMailboxRejectsMalformedSequences(t *testing.T) {
	for _, value := range []string{"broken&", "&A-", "&***-", "raw-é"} {
		if _, err := DecodeMailbox(value); err == nil {
			t.Errorf("DecodeMailbox(%q) accepted malformed input", value)
		}
	}
}
