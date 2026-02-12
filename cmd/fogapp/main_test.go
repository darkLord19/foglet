package main

import "testing"

func TestUnsupportedBuildMessage(t *testing.T) {
	msg := unsupportedBuildMessage()
	if msg == "" {
		t.Fatal("expected unsupported build message")
	}
}
