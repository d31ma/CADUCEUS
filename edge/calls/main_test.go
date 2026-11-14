package main

import (
	"context"
	"errors"
	"testing"
)

func TestRunStopsWithoutTheKeysUrl(t *testing.T) {
	t.Setenv("MESAIL_CALLS_KEYS_URL", "")
	if (Program{}).Run() == nil {
		t.Fatal("the program started without MESAIL_CALLS_KEYS_URL")
	}
}

func TestRunStopsWhenTheMediaPortIsTaken(t *testing.T) {
	t.Setenv("MESAIL_CALLS_KEYS_URL", "http://realtime/keys")
	t.Setenv("MESAIL_CALLS_MEDIA_LISTEN", "256.0.0.1:1")
	if (Program{}).Run() == nil {
		t.Fatal("the program started without a media port")
	}
}

func TestWaitForEndReportsAServerFailure(t *testing.T) {
	failures := make(chan error, 1)
	failures <- errors.New("port in use")
	if (Program{}).waitForEnd(context.Background(), failures) == nil {
		t.Fatal("the failure was lost")
	}
	ended, cancel := context.WithCancel(context.Background())
	cancel()
	if (Program{}).waitForEnd(ended, make(chan error)) != nil {
		t.Fatal("a normal stop gave an error")
	}
}
