package util

import (
	"errors"
	"testing"
)

func TestWrapError(t *testing.T) {
	origErr := errors.New("original error")

	tests := []struct {
		name    string
		msg     string
		err     error
		details []interface{}
		wantMsg string
	}{
		{
			name:    "only message",
			msg:     "msg",
			err:     nil,
			details: nil,
			wantMsg: "msg",
		},
		{
			name:    "with original error",
			msg:     "msg",
			err:     origErr,
			details: nil,
			wantMsg: "msg: original error",
		},
		{
			name:    "with even number of details",
			msg:     "msg",
			err:     origErr,
			details: []interface{}{"key", "val"},
			wantMsg: "msg: original error，key: val",
		},
		{
			name:    "with odd number of details",
			msg:     "msg",
			err:     origErr,
			details: []interface{}{"key"},
			wantMsg: "msg: original error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WrapError(tt.msg, tt.err, tt.details...)
			if got.Error() != tt.wantMsg {
				t.Errorf("WrapError() Error() = %q, want %q", got.Error(), tt.wantMsg)
			}

			// Verify it's a *ParseError and holds the original error
			var parseErr *ParseError
			if !errors.As(got, &parseErr) {
				t.Fatalf("WrapError() returned error is not of type *ParseError")
			}
			if parseErr.original != tt.err {
				t.Errorf("WrapError() original error = %v, want %v", parseErr.original, tt.err)
			}
		})
	}
}

func TestParseError(t *testing.T) {
	origErr := errors.New("original error")
	msg := "test error message"
	err := &ParseError{
		message:  msg,
		original: origErr,
	}

	t.Run("Error() returns correct message", func(t *testing.T) {
		if err.Error() != msg {
			t.Errorf("ParseError.Error() = %q, want %q", err.Error(), msg)
		}
	})

	t.Run("Unwrap() returns original error", func(t *testing.T) {
		// Using errors.Is to verify Unwrap functionality
		if !errors.Is(err, origErr) {
			t.Errorf("ParseError.Unwrap() via errors.Is failed: expected to wrap %v", origErr)
		}
		// Direct check
		if err.Unwrap() != origErr {
			t.Errorf("ParseError.Unwrap() = %v, want %v", err.Unwrap(), origErr)
		}
	})

	t.Run("Error() with no original error", func(t *testing.T) {
		errNoOrig := &ParseError{message: "no original"}
		if errNoOrig.Error() != "no original" {
			t.Errorf("ParseError.Error() = %q, want %q", errNoOrig.Error(), "no original")
		}
		if errNoOrig.Unwrap() != nil {
			t.Errorf("ParseError.Unwrap() = %v, want nil", errNoOrig.Unwrap())
		}
	})
}
