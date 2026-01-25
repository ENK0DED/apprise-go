package cli

import (
	"bytes"
	"os"
	"testing"
	"time"
)

func TestRunNoArgsDoesNotReadStdin(t *testing.T) {
	oldStdin := os.Stdin
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdin = reader
	defer func() {
		os.Stdin = oldStdin
		_ = reader.Close()
		_ = writer.Close()
	}()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	done := make(chan int, 1)
	go func() {
		done <- Run([]string{}, stdout, stderr)
	}()

	select {
	case code := <-done:
		if code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
	case <-time.After(500 * time.Millisecond):
		_ = writer.Close()
		select {
		case code := <-done:
			t.Fatalf("Run blocked on stdin (exit code %d)", code)
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("Run blocked on stdin after closing pipe")
		}
	}
}
