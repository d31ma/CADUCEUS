package channels

import (
	"errors"
	"testing"
)

func TestMemoryDirectoryKnowsOnlyTheMembersItWasGiven(t *testing.T) {
	fake := NewMemoryDirectory()
	fake.Knows("bo")
	bo, _ := fake.MemberInDomain("Bearer ana", "bo")
	eve, _ := fake.MemberInDomain("Bearer ana", "eve")
	if !bo || eve {
		t.Fatalf("want bo known and eve unknown, got %v %v", bo, eve)
	}
}

func TestMemoryDirectoryClaimsReleasesAndRefusesTakenAddresses(t *testing.T) {
	fake := NewMemoryDirectory()
	fake.Take("bo@example.com")
	if !errors.As(fake.ClaimChannelAddress("Bearer ana", "bo@example.com", "ch_1"), &AddressTaken{}) {
		t.Fatal("a taken address was claimed")
	}
	if err := fake.ClaimChannelAddress("Bearer ana", "news@example.com", "ch_1"); err != nil || fake.ClaimedBy("news@example.com") != "ch_1" {
		t.Fatalf("claim failed: %v", err)
	}
	if err := fake.ReleaseChannelAddress("Bearer ana", "news@example.com"); err != nil || fake.ClaimedBy("news@example.com") != "" || len(fake.Released()) != 1 {
		t.Fatalf("release failed: %v", err)
	}
}

func TestMemoryDirectoryFailsEveryCallAfterFail(t *testing.T) {
	fake := NewMemoryDirectory()
	fake.Fail(errors.New("down"))
	if _, err := fake.MemberInDomain("Bearer ana", "bo"); err == nil {
		t.Fatal("member lookup did not fail")
	}
	if fake.ClaimChannelAddress("Bearer ana", "x@example.com", "ch_1") == nil || fake.ReleaseChannelAddress("Bearer ana", "x@example.com") == nil {
		t.Fatal("address calls did not fail")
	}
}
