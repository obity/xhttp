package xhttp

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"strings"
	"testing"
)

// errorReader is a helper to simulate read errors.
type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("simulated read error")
}

func TestMultipartBody(t *testing.T) {
	// 1. Setup data
	fieldName := "name"
	fieldValue := "val"
	fileFieldName := "file"
	fileName := "test.txt"
	fileContent := []byte("content")

	// 2. Build MultipartBody
	mw := NewMultipartBody()
	opt := mw.
		AddField(fieldName, fieldValue).
		AddFile(fileFieldName, fileName, bytes.NewReader(fileContent)).
		Option()

	// 3. Apply Option to a requestConfig
	cfg := &requestConfig{}
	opt(cfg)

	// 4. Verify no error occurred during construction
	if cfg.bodyErr != nil {
		t.Fatalf("expected no error, got: %v", cfg.bodyErr)
	}

	// 5. Verify Content-Type
	if !strings.HasPrefix(cfg.contentType, "multipart/form-data; boundary=") {
		t.Errorf("expected Content-Type to start with 'multipart/form-data; boundary=', got: %s", cfg.contentType)
	}

	// Extract boundary from Content-Type
	parts := strings.Split(cfg.contentType, "boundary=")
	if len(parts) != 2 {
		t.Fatalf("failed to parse boundary from Content-Type: %s", cfg.contentType)
	}
	boundary := parts[1]

	// 6. Verify Body content
	reader := multipart.NewReader(bytes.NewReader(cfg.body), boundary)
	foundField := false
	foundFile := false

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("error reading multipart part: %v", err)
		}

		if part.FormName() == fieldName {
			foundField = true
			buf := new(bytes.Buffer)
			if _, err := io.Copy(buf, part); err != nil {
				t.Fatalf("error reading field value: %v", err)
			}
			if buf.String() != fieldValue {
				t.Errorf("expected field value %s, got %s", fieldValue, buf.String())
			}
		} else if part.FormName() == fileFieldName {
			foundFile = true
			if part.FileName() != fileName {
				t.Errorf("expected filename %s, got %s", fileName, part.FileName())
			}
			buf := new(bytes.Buffer)
			if _, err := io.Copy(buf, part); err != nil {
				t.Fatalf("error reading file content: %v", err)
			}
			if !bytes.Equal(buf.Bytes(), fileContent) {
				t.Errorf("expected file content %s, got %s", string(fileContent), buf.String())
			}
		}
	}

	if !foundField {
		t.Errorf("field %s not found in multipart body", fieldName)
	}
	if !foundFile {
		t.Errorf("file %s not found in multipart body", fileFieldName)
	}
}

func TestMultipartError(t *testing.T) {
	// 1. Setup error reader
	errR := &errorReader{}

	// 2. Build MultipartBody with error-prone reader
	mw := NewMultipartBody()
	opt := mw.AddFile("file", "test.txt", errR).Option()

	// 3. Apply Option to a requestConfig
	cfg := &requestConfig{}
	opt(cfg)

	// 4. Verify error was captured
	if cfg.bodyErr == nil {
		t.Fatal("expected an error in cfg.bodyErr, but got nil")
	}

	if !strings.Contains(cfg.bodyErr.Error(), "simulated read error") {
		t.Errorf("expected error message to contain 'simulated read error', got: %v", cfg.bodyErr)
	}
}
