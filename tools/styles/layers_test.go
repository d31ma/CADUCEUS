package main

import "testing"

func TestWithLayerOrderAddsLineAboveImports(t *testing.T) {
	original := "@import \"../vendor/duvay/duvay.css\" layer(duvay);\n:root {}\n"
	want := layerOrder + "\n\n" + original
	if got := withLayerOrder(original); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWithLayerOrderKeepsCorrectLine(t *testing.T) {
	original := layerOrder + "\n@import \"a.css\" layer(duvay);\n"
	if got := withLayerOrder(original); got != original {
		t.Fatalf("got %q", got)
	}
}

func TestWithLayerOrderReplacesOtherLayerOrder(t *testing.T) {
	original := "@layer duvay, utilities;\n:root {}\n"
	want := layerOrder + "\n:root {}\n"
	if got := withLayerOrder(original); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWithLayerOrderDoesNotTreatLayerBlockAsOrder(t *testing.T) {
	original := "@layer brand { :root {} }\n"
	want := layerOrder + "\n\n" + original
	if got := withLayerOrder(original); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWithLayerOrderIsStableWhenRunTwice(t *testing.T) {
	once := withLayerOrder(":root {}\n")
	if twice := withLayerOrder(once); twice != once {
		t.Fatalf("second run changed the text: %q", twice)
	}
}
