package httputils

import (
	"io"
	"strings"
	"testing"
)

func TestProgressReadSeekerReportsReadsAndRewind(t *testing.T) {
	updates := make([]TransferProgress, 0, 3)
	reader := NewProgressReadSeeker(strings.NewReader("abcdef"), 6, func(progress TransferProgress) {
		updates = append(updates, progress)
	})

	buffer := make([]byte, 3)
	if _, err := io.ReadFull(reader, buffer); err != nil {
		t.Fatalf("read first part: %v", err)
	}
	if string(buffer) != "abc" {
		t.Fatalf("first read = %q, want abc", buffer)
	}
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("rewind reader: %v", err)
	}
	if _, err := io.ReadFull(reader, buffer); err != nil {
		t.Fatalf("read after rewind: %v", err)
	}

	want := []TransferProgress{
		{Current: 3, Total: 6},
		{Current: 0, Total: 6},
		{Current: 3, Total: 6},
	}
	if len(updates) != len(want) {
		t.Fatalf("progress updates = %v, want %v", updates, want)
	}
	for i := range want {
		if updates[i] != want[i] {
			t.Fatalf("progress update %d = %+v, want %+v", i, updates[i], want[i])
		}
	}
}

func TestNewProgressReadSeekerHandlesNilAndNegativeTotal(t *testing.T) {
	if got := NewProgressReadSeeker(nil, 1, nil); got != nil {
		t.Fatal("nil source should produce nil reader")
	}

	var update TransferProgress
	reader := NewProgressReadSeeker(strings.NewReader("x"), -1, func(progress TransferProgress) {
		update = progress
	})
	if _, err := io.ReadAll(reader); err != nil {
		t.Fatalf("read progress reader: %v", err)
	}
	if update != (TransferProgress{Current: 1, Total: 0}) {
		t.Fatalf("progress update = %+v", update)
	}
}
