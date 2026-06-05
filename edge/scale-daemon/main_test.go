package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

func init() {
	// httpClient is initialised in main(); tests must set it up themselves.
	httpClient = &http.Client{Timeout: 5 * time.Second}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// collectingServer starts a test HTTP server that records every weight payload
// it receives. Call the returned getter after the test to inspect the results.
func collectingServer(t *testing.T) (*httptest.Server, func() []weightPayload) {
	t.Helper()
	var mu sync.Mutex
	var received []weightPayload

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var p weightPayload
		if err := json.Unmarshal(body, &p); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		received = append(received, p)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))

	return srv, func() []weightPayload {
		mu.Lock()
		defer mu.Unlock()
		cp := make([]weightPayload, len(received))
		copy(cp, received)
		return cp
	}
}

// startSender launches runSender in a goroutine and returns the weights channel,
// a cancel function, and a done channel that closes when the sender exits.
func startSender(cfg *config) (chan<- int, context.CancelFunc, <-chan struct{}) {
	ctx, cancel := context.WithCancel(context.Background())
	weights := make(chan int, 64)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runSender(ctx, cfg, weights)
	}()
	return weights, cancel, done
}

func cfgWith(url string, debounceMs, minWeightKg int) *config {
	return &config{
		IngestorURL: url,
		DebounceMs:  debounceMs,
		MinWeightKg: minWeightKg,
	}
}

// ── regex ─────────────────────────────────────────────────────────────────────

func TestWeightRegex(t *testing.T) {
	cases := []struct {
		input string
		want  int
		match bool
	}{
		{"  39780 kg ", 39780, true},
		{"weight: 1234 kg\r\n", 1234, true},
		{"0 kg", 0, true},
		{"ST,GS,  16800 kg", 16800, true},
		{"no weight here", 0, false},
		{" kg only", 0, false},
		{"", 0, false},
	}
	for _, tc := range cases {
		m := weightRE.FindStringSubmatch(tc.input)
		if tc.match && m == nil {
			t.Errorf("input %q: expected match", tc.input)
			continue
		}
		if !tc.match && m != nil {
			t.Errorf("input %q: expected no match, got %v", tc.input, m)
			continue
		}
		if tc.match {
			w, _ := strconv.Atoi(m[1])
			if w != tc.want {
				t.Errorf("input %q: got %d, want %d", tc.input, w, tc.want)
			}
		}
	}
}

// ── config file ───────────────────────────────────────────────────────────────

func TestReadConfigFile_ValidFile(t *testing.T) {
	f, err := os.CreateTemp("", "scale-cfg-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString(`{"scale_host":"10.0.0.1","scale_port":"9999","debounce_ms":7000,"min_weight_kg":500}`)
	f.Close()

	fc := readConfigFile(f.Name())

	if fc.ScaleHost == nil || *fc.ScaleHost != "10.0.0.1" {
		t.Errorf("scale_host: got %v, want 10.0.0.1", fc.ScaleHost)
	}
	if fc.ScalePort == nil || *fc.ScalePort != "9999" {
		t.Errorf("scale_port: got %v, want 9999", fc.ScalePort)
	}
	if fc.DebounceMs == nil || *fc.DebounceMs != 7000 {
		t.Errorf("debounce_ms: got %v, want 7000", fc.DebounceMs)
	}
	if fc.MinWeightKg == nil || *fc.MinWeightKg != 500 {
		t.Errorf("min_weight_kg: got %v, want 500", fc.MinWeightKg)
	}
	// Absent field must remain nil so it doesn't override defaults.
	if fc.APIKey != nil {
		t.Errorf("api_key should be nil for absent field")
	}
}

func TestReadConfigFile_MissingFile(t *testing.T) {
	fc := readConfigFile("/tmp/scale-daemon-definitely-does-not-exist-xyz.json")
	if fc.ScaleHost != nil || fc.DebounceMs != nil {
		t.Error("expected all nil fields when file is absent")
	}
}

func TestReadConfigFile_EmptyObject(t *testing.T) {
	f, _ := os.CreateTemp("", "scale-cfg-*.json")
	defer os.Remove(f.Name())
	f.WriteString(`{}`)
	f.Close()

	fc := readConfigFile(f.Name())
	if fc.ScaleHost != nil || fc.DebounceMs != nil {
		t.Error("empty JSON object should leave all fields nil")
	}
}

// ── sender: peak tracking ─────────────────────────────────────────────────────

// Peak of all readings in the window is dispatched, not just the last one.
func TestSender_SendsPeak(t *testing.T) {
	srv, payloads := collectingServer(t)
	defer srv.Close()

	weights, cancel, done := startSender(cfgWith(srv.URL, 80, 0))

	weights <- 30000
	weights <- 35000
	weights <- 39780 // highest

	time.Sleep(160 * time.Millisecond) // let timer fire
	cancel()
	<-done

	got := payloads()
	if len(got) != 1 {
		t.Fatalf("expected 1 send, got %d", len(got))
	}
	if got[0].WeightKg != 39780 {
		t.Errorf("weight_kg: got %d, want 39780", got[0].WeightKg)
	}
}

// ── sender: timer starts once ─────────────────────────────────────────────────

// A reading that arrives after the timer has started must NOT restart it.
// If it did, the send would be delayed and only 1 reading would arrive before
// the cancel — making it impossible to distinguish restart vs no-restart.
// Instead we verify that readings arriving mid-window are included in history.
func TestSender_TimerStartsOnce(t *testing.T) {
	srv, payloads := collectingServer(t)
	defer srv.Close()

	const debounce = 100 * time.Millisecond
	weights, cancel, done := startSender(cfgWith(srv.URL, int(debounce.Milliseconds()), 0))

	weights <- 10000                          // t=0 → timer starts (100 ms)
	time.Sleep(50 * time.Millisecond)
	weights <- 20000                          // t=50ms → timer must NOT restart
	time.Sleep(100 * time.Millisecond)        // t=150ms → timer fired at t=100ms

	cancel()
	<-done

	got := payloads()
	if len(got) != 1 {
		t.Fatalf("expected 1 send, got %d; timer may have restarted", len(got))
	}
	if got[0].WeightKg != 20000 {
		t.Errorf("weight_kg: got %d, want 20000 (peak)", got[0].WeightKg)
	}
	if len(got[0].HistoryScale) != 2 {
		t.Errorf("history entries: got %d, want 2", len(got[0].HistoryScale))
	}
}

// ── sender: history ───────────────────────────────────────────────────────────

func TestSender_HistoryContainsAllReadings(t *testing.T) {
	srv, payloads := collectingServer(t)
	defer srv.Close()

	weights, cancel, done := startSender(cfgWith(srv.URL, 80, 0))

	readings := []int{11000, 22000, 33000, 44000}
	for _, w := range readings {
		weights <- w
	}

	time.Sleep(160 * time.Millisecond)
	cancel()
	<-done

	got := payloads()
	if len(got) != 1 {
		t.Fatalf("expected 1 send, got %d", len(got))
	}
	if len(got[0].HistoryScale) != len(readings) {
		t.Errorf("history entries: got %d, want %d", len(got[0].HistoryScale), len(readings))
	}
	// Verify peak is the highest reading.
	if got[0].WeightKg != 44000 {
		t.Errorf("weight_kg: got %d, want 44000", got[0].WeightKg)
	}
}

// ── sender: reset after send ──────────────────────────────────────────────────

// After a send, the window resets. A new weight change must trigger a fresh send.
func TestSender_ResetAfterSend(t *testing.T) {
	srv, payloads := collectingServer(t)
	defer srv.Close()

	weights, cancel, done := startSender(cfgWith(srv.URL, 80, 0))

	// First window.
	weights <- 39780
	time.Sleep(160 * time.Millisecond)

	// Second window (different truck, lighter load).
	weights <- 25000
	time.Sleep(160 * time.Millisecond)

	cancel()
	<-done

	got := payloads()
	if len(got) != 2 {
		t.Fatalf("expected 2 sends, got %d", len(got))
	}
	if got[0].WeightKg != 39780 {
		t.Errorf("first send: got %d, want 39780", got[0].WeightKg)
	}
	if got[1].WeightKg != 25000 {
		t.Errorf("second send: got %d, want 25000", got[1].WeightKg)
	}
}

// ── sender: min weight threshold ──────────────────────────────────────────────

func TestSender_BelowThresholdNotSent(t *testing.T) {
	srv, payloads := collectingServer(t)
	defer srv.Close()

	weights, cancel, done := startSender(cfgWith(srv.URL, 80, 1000))

	weights <- 500  // below min_weight_kg=1000
	weights <- 200

	time.Sleep(160 * time.Millisecond)
	cancel()
	<-done

	if got := payloads(); len(got) != 0 {
		t.Errorf("expected 0 sends for below-threshold readings, got %d", len(got))
	}
}

func TestSender_AboveThresholdSent(t *testing.T) {
	srv, payloads := collectingServer(t)
	defer srv.Close()

	weights, cancel, done := startSender(cfgWith(srv.URL, 80, 1000))

	weights <- 500   // below — ignored
	weights <- 5000  // above — starts window, peak=5000

	time.Sleep(160 * time.Millisecond)
	cancel()
	<-done

	got := payloads()
	if len(got) != 1 {
		t.Fatalf("expected 1 send, got %d", len(got))
	}
	if got[0].WeightKg != 5000 {
		t.Errorf("weight_kg: got %d, want 5000", got[0].WeightKg)
	}
}

// ── sender: duplicate readings ignored ───────────────────────────────────────

// Sending the same value twice must not create duplicate history entries.
func TestSender_DuplicateReadingsIgnored(t *testing.T) {
	srv, payloads := collectingServer(t)
	defer srv.Close()

	weights, cancel, done := startSender(cfgWith(srv.URL, 80, 0))

	weights <- 39780
	weights <- 39780 // duplicate — should be ignored
	weights <- 39780

	time.Sleep(160 * time.Millisecond)
	cancel()
	<-done

	got := payloads()
	if len(got) != 1 {
		t.Fatalf("expected 1 send, got %d", len(got))
	}
	if len(got[0].HistoryScale) != 1 {
		t.Errorf("history entries: got %d, want 1 (duplicates should be ignored)", len(got[0].HistoryScale))
	}
}
