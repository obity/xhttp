package xhttp

import (
	"net/http"
	"testing"
	"time"
)

// DummyHeaderSetter implements HeaderSetter for testing.
type DummyHeaderSetter struct{}

func (d *DummyHeaderSetter) SetHeaders(req *http.Request) {}

func TestClientCopy(t *testing.T) {
	setter := &DummyHeaderSetter{}
	original := NewClient(setter)
	original.successCodes[http.StatusAccepted] = struct{}{} // Add a custom success code

	// Set a custom transport to test cloning
	customTransport := &http.Transport{
		MaxIdleConns: 10,
	}
	// Use the returned client because WithTransport returns a new instance
	clientWithTransport := original.WithTransport(customTransport)

	copied := clientWithTransport.copy()

	// 1. Verify httpClient is a new instance
	if copied.httpClient == clientWithTransport.httpClient {
		t.Error("expected httpClient to be a new instance, but it's the same")
	}

	// 2. Verify successCodes is a deep copy
	if len(copied.successCodes) != len(clientWithTransport.successCodes) {
		t.Errorf("expected successCodes length %d, got %d", len(clientWithTransport.successCodes), len(copied.successCodes))
	}

	// Modify original successCodes
	clientWithTransport.successCodes[http.StatusNoContent] = struct{}{}

	if _, ok := copied.successCodes[http.StatusNoContent]; ok {
		t.Error("expected copied successCodes not to be affected by modification to original")
	}

	if _, ok := clientWithTransport.successCodes[http.StatusAccepted]; !ok {
		t.Error("expected clientWithTransport successCodes to still contain StatusAccepted")
	}

	// 3. Verify Transport is correctly cloned
	origT, ok := clientWithTransport.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected original transport to be *http.Transport")
	}

	copyT, ok := copied.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected copied transport to be *http.Transport")
	}

	if origT == copyT {
		t.Error("expected Transport to be a clone, but it's the same instance")
	}

	if copyT.MaxIdleConns != origT.MaxIdleConns {
		t.Errorf("expected MaxIdleConns %d, got %d", origT.MaxIdleConns, copyT.MaxIdleConns)
	}
}

func TestClientTransport(t *testing.T) {
	setter := &DummyHeaderSetter{}

	t.Run("Lazy Loading", func(t *testing.T) {
		client := NewClient(setter)
		if client.httpClient.Transport != nil {
			t.Error("expected Transport to be nil initially")
		}

		tr := client.transport()
		if tr == nil {
			t.Fatal("expected transport to be initialized, but got nil")
		}

		if client.httpClient.Transport != tr {
			t.Error("expected httpClient.Transport to be the initialized transport")
		}
	})

	t.Run("Non-http-Transport Handling", func(t *testing.T) {
		client := NewClient(setter)
		// Set a custom RoundTripper that is NOT *http.Transport
		customRT := &mockRoundTripper{
			roundTrip: func(req *http.Request) (*http.Response, error) {
				return nil, nil
			},
		}
		// Use the returned client
		clientWithCustomRT := client.WithTransport(customRT)

		tr := clientWithCustomRT.transport()
		if tr != nil {
			t.Errorf("expected transport to be nil when Transport is not *http.Transport, got %v", tr)
		}
	})
}

func TestTransportOptions(t *testing.T) {
	setter := &DummyHeaderSetter{}
	client := NewClient(setter)

	// Trigger lazy loading
	_ = client.transport()

	t.Run("WithMaxIdleConns", func(t *testing.T) {
		newClient := client.WithMaxIdleConns(50)
		tr := newClient.transport()
		if tr.MaxIdleConns != 50 {
			t.Errorf("expected MaxIdleConns 50, got %d", tr.MaxIdleConns)
		}

		// Verify original client is not affected
		origTr := client.transport()
		if origTr.MaxIdleConns == 50 {
			t.Error("original client's MaxIdleConns was modified")
		}
	})

	t.Run("WithMaxIdleConnsPerHost", func(t *testing.T) {
		newClient := client.WithMaxIdleConnsPerHost(10)
		tr := newClient.transport()
		if tr.MaxIdleConnsPerHost != 10 {
			t.Errorf("expected MaxIdleConnsPerHost 10, got %d", tr.MaxIdleConnsPerHost)
		}
	})

	t.Run("WithIdleConnTimeout", func(t *testing.T) {
		timeout := 30 * time.Second
		newClient := client.WithIdleConnTimeout(timeout)
		tr := newClient.transport()
		if tr.IdleConnTimeout != timeout {
			t.Errorf("expected IdleConnTimeout %v, got %v", timeout, tr.IdleConnTimeout)
		}
	})
}

// mockRoundTripper is used for testing.
type mockRoundTripper struct {
	roundTrip func(*http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTrip(req)
}
