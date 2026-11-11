package main

import "testing"

func TestWithRegionAppendsToAuthoredStyles(t *testing.T) {
	got, err := withRegion(".a {\n  gap: 0;\n}\n", ".grid {}")
	want := ".a {\n  gap: 0;\n}\n\n" + regionStart + "\n.grid {}\n" + regionEnd + "\n"
	if err != nil || got != want {
		t.Fatalf("got %q, %v; want %q", got, err, want)
	}
}

func TestWithRegionCreatesNewFile(t *testing.T) {
	got, err := withRegion("", ".grid {}")
	want := regionStart + "\n.grid {}\n" + regionEnd + "\n"
	if err != nil || got != want {
		t.Fatalf("got %q, %v; want %q", got, err, want)
	}
}

func TestWithRegionReplacesOldRegionAndKeepsAuthoredText(t *testing.T) {
	original := ".a {}\n\n" + regionStart + "\n.old {}\n" + regionEnd + "\n"
	got, err := withRegion(original, ".new {}")
	want := ".a {}\n\n" + regionStart + "\n.new {}\n" + regionEnd + "\n"
	if err != nil || got != want {
		t.Fatalf("got %q, %v; want %q", got, err, want)
	}
}

func TestWithRegionMovesMiddleRegionToTheEnd(t *testing.T) {
	original := ".a {}\n" + regionStart + "\n.old {}\n" + regionEnd + "\n.b {}\n"
	got, err := withRegion(original, ".new {}")
	want := ".a {}\n\n.b {}\n\n" + regionStart + "\n.new {}\n" + regionEnd + "\n"
	if err != nil || got != want {
		t.Fatalf("got %q, %v; want %q", got, err, want)
	}
}

func TestWithRegionIsStableWhenRunTwice(t *testing.T) {
	first, _ := withRegion(".a {}\n", ".grid {}")
	second, err := withRegion(first, ".grid {}")
	if err != nil || second != first {
		t.Fatalf("second run changed the file: %q, %v", second, err)
	}
}

func TestWithRegionRemovesRegionWhenNothingIsGenerated(t *testing.T) {
	original := ".a {}\n\n" + regionStart + "\n.old {}\n" + regionEnd + "\n"
	got, err := withRegion(original, "")
	if err != nil || got != ".a {}\n" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestWithRegionLeavesFileWithoutRegionUntouchedWhenNothingIsGenerated(t *testing.T) {
	original := ".a {}\n\n\n"
	got, err := withRegion(original, "")
	if err != nil || got != original {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestWithRegionEmptiesFileThatHeldOnlyARegion(t *testing.T) {
	got, err := withRegion(regionStart+"\n.old {}\n"+regionEnd+"\n", "")
	if err != nil || got != "" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestWithRegionRejectsMissingEnd(t *testing.T) {
	if _, err := withRegion(".a {}\n"+regionStart+"\n.old {}\n", ".new {}"); err == nil {
		t.Fatal("expected an error for a region without an end marker")
	}
}

func TestWithRegionRejectsMissingStart(t *testing.T) {
	if _, err := withRegion(".a {}\n.old {}\n"+regionEnd+"\n", ".new {}"); err == nil {
		t.Fatal("expected an error for a region without a start marker")
	}
}

func TestWithRegionRejectsEndBeforeStart(t *testing.T) {
	if _, err := withRegion(regionEnd+"\n.old {}\n"+regionStart+"\n", ".new {}"); err == nil {
		t.Fatal("expected an error for markers in the wrong order")
	}
}

func TestWithRegionRejectsTwoRegions(t *testing.T) {
	region := regionStart + "\n.old {}\n" + regionEnd + "\n"
	if _, err := withRegion(region+region, ".new {}"); err == nil {
		t.Fatal("expected an error for a repeated region")
	}
}
