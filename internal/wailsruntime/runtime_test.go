package wailsruntime

import (
	"errors"
	"fmt"
	"testing"
)

func TestNormalizeDialogSelection(t *testing.T) {
	t.Run("keeps selected value", func(t *testing.T) {
		const selection = `D:\Games`

		got, err := normalizeDialogSelection(selection, nil)
		if err != nil {
			t.Fatalf("normalizeDialogSelection() error = %v", err)
		}
		if got != selection {
			t.Fatalf("normalizeDialogSelection() = %q, want %q", got, selection)
		}
	})

	t.Run("converts cancellation to empty selection", func(t *testing.T) {
		got, err := normalizeDialogSelection("", errors.New(wailsDialogCancelledMessage))
		if err != nil {
			t.Fatalf("normalizeDialogSelection() error = %v", err)
		}
		if got != "" {
			t.Fatalf("normalizeDialogSelection() = %q, want empty selection", got)
		}
	})

	t.Run("converts wrapped cancellation to empty selection", func(t *testing.T) {
		cancelled := errors.New(wailsDialogCancelledMessage)

		got, err := normalizeDialogSelection("", fmt.Errorf("open dialog: %w", cancelled))
		if err != nil {
			t.Fatalf("normalizeDialogSelection() error = %v", err)
		}
		if got != "" {
			t.Fatalf("normalizeDialogSelection() = %q, want empty selection", got)
		}
	})

	t.Run("preserves other errors", func(t *testing.T) {
		dialogErr := errors.New("dialog unavailable")

		got, err := normalizeDialogSelection("ignored", dialogErr)
		if got != "" {
			t.Fatalf("normalizeDialogSelection() = %q, want empty selection", got)
		}
		if !errors.Is(err, dialogErr) {
			t.Fatalf("normalizeDialogSelection() error = %v, want %v", err, dialogErr)
		}
	})
}
