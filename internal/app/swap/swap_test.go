package swap

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func body(h http.Handler) (int, string) {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	return rec.Code, rec.Body.String()
}

func constant(code int, s string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
		_, _ = w.Write([]byte(s))
	})
}

func TestZeroValueIsUnavailable(t *testing.T) {
	var h Handler
	if code, _ := body(&h); code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", code)
	}
}

func TestSetSwitchesHandler(t *testing.T) {
	var h Handler
	h.Set(constant(200, "recovery"))
	if _, b := body(&h); b != "recovery" {
		t.Fatalf("body = %q", b)
	}
	h.Set(constant(200, "normal"))
	if _, b := body(&h); b != "normal" {
		t.Fatalf("body = %q", b)
	}
	h.Set(nil)
	if code, _ := body(&h); code != http.StatusServiceUnavailable {
		t.Fatalf("code after Set(nil) = %d, want 503", code)
	}
}

// 差し替えと配信が同時に走っても、リクエストは必ず「差し替え前」か「差し替え後」のどちらかで完結する（-race で検査）。
func TestConcurrentSwapAndServe(t *testing.T) {
	var h Handler
	h.Set(constant(200, "old"))

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 200 {
				code, b := body(&h)
				if code != 200 || (b != "old" && b != "new") {
					t.Errorf("unexpected response: %d %q", code, b)
					return
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range 200 {
			if i%2 == 0 {
				h.Set(constant(200, "new"))
			} else {
				h.Set(constant(200, "old"))
			}
		}
		h.Set(constant(200, "new"))
	}()
	wg.Wait()

	if _, b := body(&h); b != "new" {
		t.Fatalf("final body = %q, want new", b)
	}
}

func TestInFlightRequestCompletesOnOldHandler(t *testing.T) {
	var h Handler
	entered, release := make(chan struct{}), make(chan struct{})
	h.Set(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		_, _ = w.Write([]byte("old"))
	}))
	done := make(chan string)
	go func() { _, b := body(&h); done <- b }()
	<-entered
	h.Set(constant(200, "new"))
	close(release)
	if b := <-done; b != "old" {
		t.Fatalf("in-flight response = %q, want old", b)
	}
	if _, b := body(&h); b != "new" {
		t.Fatalf("next response = %q, want new", b)
	}
}
