package notion

import (
	"net/http"
	"net/url"
	gosync "sync"
	"testing"
)

// TestBatchClient_GetClientConcurrent is a regression test for the
// unsynchronised round-robin counter; run with -race.
func TestBatchClient_GetClientConcurrent(t *testing.T) {
	bc := NewBatchClient("token", 3)

	const workers, perWorker = 30, 100
	counts := make(map[Client]int)
	var mu gosync.Mutex
	var wg gosync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := make(map[Client]int)
			for i := 0; i < perWorker; i++ {
				local[bc.GetClient()]++
			}
			mu.Lock()
			for c, n := range local {
				counts[c] += n
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	if len(counts) != 3 {
		t.Fatalf("expected all 3 clients to be used, got %d", len(counts))
	}
	for _, n := range counts {
		if n != workers*perWorker/3 {
			t.Errorf("uneven round-robin distribution: %v", counts)
			break
		}
	}
}

func TestBatchClient_ClientCountBounds(t *testing.T) {
	tests := []struct {
		requested int
		want      int
	}{
		{0, 1},
		{-5, 1},
		{4, 4},
		{50, 10},
	}
	for _, tt := range tests {
		if got := len(NewBatchClient("token", tt.requested).clients); got != tt.want {
			t.Errorf("NewBatchClient(%d) has %d clients, want %d", tt.requested, got, tt.want)
		}
	}
}

// TestTunedClients_UseProxyFromEnvironment is a regression test: the custom
// transports ignored HTTPS_PROXY.
func TestTunedClients_UseProxyFromEnvironment(t *testing.T) {
	tests := []struct {
		name   string
		client Client
	}{
		{"optimized", NewOptimizedClient("token")},
		{"burst", NewBurstClient("token")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, ok := tt.client.(*client)
			if !ok {
				t.Fatalf("unexpected client type %T", tt.client)
			}
			transport, ok := c.httpClient.Transport.(*http.Transport)
			if !ok {
				t.Fatalf("unexpected transport type %T", c.httpClient.Transport)
			}
			if transport.Proxy == nil {
				t.Fatal("transport has no proxy function")
			}

			// The transport must resolve proxies exactly like
			// http.ProxyFromEnvironment (which caches the environment on
			// first use, so compare results rather than set variables).
			req := &http.Request{URL: &url.URL{Scheme: "https", Host: "api.notion.com"}}
			want, wantErr := http.ProxyFromEnvironment(req)
			got, gotErr := transport.Proxy(req)
			if (wantErr == nil) != (gotErr == nil) {
				t.Fatalf("Proxy() error = %v, want %v", gotErr, wantErr)
			}
			// Either may be nil (no proxy configured).
			if urlString(want) != urlString(got) {
				t.Errorf("Proxy() = %q, want %q", urlString(got), urlString(want))
			}
			if transport.TLSClientConfig != nil && transport.TLSClientConfig.InsecureSkipVerify {
				t.Error("TLS verification must stay enabled")
			}
		})
	}
}

func urlString(u *url.URL) string {
	if u == nil {
		return ""
	}
	return u.String()
}
