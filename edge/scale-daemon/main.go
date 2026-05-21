package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

const serviceName = "omnigate-scale-daemon"

// weightRE extracts the integer kilogram value from scale output, e.g. "  39780 kg ".
var weightRE = regexp.MustCompile(`(\d+)\s*kg`)

// config holds all runtime parameters resolved from env vars with flag fallbacks.
type config struct {
	ScaleHost    string
	ScalePort    string
	IngestorURL  string
	DeviceID     string
	APIKey       string
	OTELEndpoint string
	DebounceMs   int
	MinWeightKg  int
	ReconnectSec int
}

func loadConfig() *config {
	cfg := &config{}

	flag.StringVar(&cfg.ScaleHost, "scale-host",
		envOr("SCALE_HOST", "127.0.0.1"), "Scale TCP host")
	flag.StringVar(&cfg.ScalePort, "scale-port",
		envOr("SCALE_PORT", "5001"), "Scale TCP port")
	flag.StringVar(&cfg.IngestorURL, "ingestor-url",
		envOr("INGESTOR_URL", "http://localhost:8090/ingest/event"), "Ingestor endpoint URL")
	flag.StringVar(&cfg.DeviceID, "device-id",
		envOr("DEVICE_ID", ""), "Device identifier (used in logs)")
	flag.StringVar(&cfg.APIKey, "api-key",
		envOr("API_KEY", ""), "API key sent as Bearer token to the gateway")
	flag.StringVar(&cfg.OTELEndpoint, "otel-endpoint",
		envOr("OTEL_ENDPOINT", "localhost:4318"), "OTLP HTTP collector endpoint (host:port)")
	flag.IntVar(&cfg.DebounceMs, "debounce-ms", 2000,
		"Milliseconds the weight must stay constant before sending")
	flag.IntVar(&cfg.MinWeightKg, "min-weight-kg", 0,
		"Minimum weight in kg to report (0 = report all, including zero)")
	flag.IntVar(&cfg.ReconnectSec, "reconnect-sec", 5,
		"Seconds to wait before re-connecting to the scale after a disconnect")
	flag.Parse()

	return cfg
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// initTelemetry wires up OTLP HTTP exporters for logs and traces.
// It returns a shutdown function and is best-effort: if the collector is
// unreachable the daemon continues running with stdout-only logging.
func initTelemetry(ctx context.Context, endpoint string) (func(context.Context), error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceNameKey.String(serviceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("resource: %w", err)
	}

	logExp, err := otlploghttp.New(ctx,
		otlploghttp.WithEndpoint(endpoint),
		otlploghttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("log exporter: %w", err)
	}
	lp := log.NewLoggerProvider(
		log.WithProcessor(log.NewBatchProcessor(logExp)),
		log.WithResource(res),
	)
	global.SetLoggerProvider(lp)

	traceExp, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		_ = lp.Shutdown(ctx)
		return nil, fmt.Errorf("trace exporter: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return func(ctx context.Context) {
		_ = lp.Shutdown(ctx)
		_ = tp.Shutdown(ctx)
	}, nil
}

// newLogger returns a slog.Logger that writes JSON to stdout AND ships logs to
// the OTel collector via the global LoggerProvider.
func newLogger() *slog.Logger {
	stdout := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	otelH := otelslog.NewHandler(serviceName)
	return slog.New(&multiHandler{handlers: []slog.Handler{stdout, otelH}})
}

// multiHandler fans a single slog.Record out to multiple slog.Handlers.
type multiHandler struct {
	handlers []slog.Handler
}

func (m *multiHandler) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		if h.Enabled(ctx, r.Level) {
			_ = h.Handle(ctx, r)
		}
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	hs := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		hs[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: hs}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	hs := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		hs[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: hs}
}

// weightPayload is the JSON body sent to the ingestor.
// The adapter's data_mapping should reference $.weight_kg.
type weightPayload struct {
	WeightKg int `json:"weight_kg"`
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

func sendWeight(ctx context.Context, cfg *config, weightKg int) error {
	body, err := json.Marshal(weightPayload{WeightKg: weightKg})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.IngestorURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("ingestor returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// runSender reads from the weights channel and dispatches stable readings.
//
// A reading is considered "stable" when the value has not changed for
// cfg.DebounceMs milliseconds. Each unique stable value is sent exactly once
// until the weight changes again.
func runSender(ctx context.Context, cfg *config, weights <-chan int) {
	debounce := time.Duration(cfg.DebounceMs) * time.Millisecond

	var lastSeen, lastSent int
	var timerC <-chan time.Time // nil → never fires until first weight arrives

	for {
		select {
		case <-ctx.Done():
			return

		case w, ok := <-weights:
			if !ok {
				return
			}
			if w == lastSeen {
				continue
			}
			lastSeen = w
			// Swap to a fresh timer so the debounce window restarts.
			// The previous timer's channel is simply abandoned; the runtime GCs it.
			t := time.NewTimer(debounce)
			timerC = t.C

		case <-timerC:
			timerC = nil // one-shot; re-armed only on next weight change
			if lastSeen == lastSent {
				continue
			}
			if lastSeen < cfg.MinWeightKg {
				slog.DebugContext(ctx, "weight below threshold, skipping",
					slog.Int("weight_kg", lastSeen),
					slog.Int("min_weight_kg", cfg.MinWeightKg),
				)
				continue
			}
			if err := sendWeight(ctx, cfg, lastSeen); err != nil {
				slog.ErrorContext(ctx, "failed to send weight",
					slog.Int("weight_kg", lastSeen),
					slog.String("error", err.Error()),
				)
			} else {
				lastSent = lastSeen
				slog.InfoContext(ctx, "weight dispatched",
					slog.Int("weight_kg", lastSeen),
					slog.String("device_id", cfg.DeviceID),
				)
			}
		}
	}
}

// runReader maintains a persistent TCP connection to the scale, parses every
// line for a weight value, and sends it to the weights channel.
// On disconnect it waits cfg.ReconnectSec seconds before retrying.
func runReader(ctx context.Context, cfg *config, weights chan<- int) {
	addr := net.JoinHostPort(cfg.ScaleHost, cfg.ScalePort)
	reconnect := time.Duration(cfg.ReconnectSec) * time.Second

	for {
		if ctx.Err() != nil {
			return
		}

		conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
		if err != nil {
			slog.WarnContext(ctx, "scale connect failed",
				slog.String("addr", addr),
				slog.String("error", err.Error()),
				slog.String("retry_in", reconnect.String()),
			)
			select {
			case <-ctx.Done():
				return
			case <-time.After(reconnect):
				continue
			}
		}

		slog.InfoContext(ctx, "connected to scale", slog.String("addr", addr))

		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			line := scanner.Text()
			m := weightRE.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			w, err := strconv.Atoi(m[1])
			if err != nil {
				continue
			}
			select {
			case weights <- w:
			case <-ctx.Done():
				conn.Close()
				return
			default:
				// Channel full: drop stale reading; the next tick will update it.
			}
		}

		if err := scanner.Err(); err != nil {
			slog.WarnContext(ctx, "scale read error",
				slog.String("addr", addr),
				slog.String("error", err.Error()),
			)
		} else {
			slog.WarnContext(ctx, "scale connection closed", slog.String("addr", addr))
		}
		conn.Close()

		select {
		case <-ctx.Done():
			return
		case <-time.After(reconnect):
		}
	}
}

func main() {
	cfg := loadConfig()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Telemetry is best-effort: a missing collector must not prevent data collection.
	shutdown, err := initTelemetry(ctx, cfg.OTELEndpoint)
	if err != nil {
		slog.Warn("telemetry unavailable, continuing with stdout only",
			slog.String("endpoint", cfg.OTELEndpoint),
			slog.String("error", err.Error()),
		)
	}
	if shutdown != nil {
		defer shutdown(context.Background())
	}

	slog.SetDefault(newLogger())

	if cfg.APIKey == "" {
		slog.Warn("API_KEY is not set — requests will be sent without authentication")
	}

	slog.Info("scale-daemon starting",
		slog.String("scale_addr", net.JoinHostPort(cfg.ScaleHost, cfg.ScalePort)),
		slog.String("ingestor_url", cfg.IngestorURL),
		slog.String("device_id", cfg.DeviceID),
		slog.Int("debounce_ms", cfg.DebounceMs),
		slog.Int("min_weight_kg", cfg.MinWeightKg),
	)

	// Buffer allows the reader to stay ahead without blocking on slow sends.
	weights := make(chan int, 32)

	go runReader(ctx, cfg, weights)
	runSender(ctx, cfg, weights) // blocks until ctx is cancelled

	slog.Info("scale-daemon stopped")
}
