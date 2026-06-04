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
	"strings"
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

// version is set at build time via -ldflags="-X main.version=v1.2.3".
var version = "dev"

// weightRE extracts the integer kilogram value from scale output, e.g. "  39780 kg ".
var weightRE = regexp.MustCompile(`(\d+)\s*kg`)

// config holds all runtime parameters resolved from (in priority order):
// CLI flags > environment variables > config file > hardcoded defaults.
type config struct {
	ConfigFile     string
	ScaleHost      string
	ScalePort      string
	IngestorURL    string
	DeviceID       string
	APIKey         string
	OTELEndpoint   string
	DebounceMs     int
	MinWeightKg    int
	ReconnectSec   int
	LogLevel       string
	HTTPTimeoutSec int
}

// fileConfig mirrors config with pointer fields so absent keys are distinguishable from zero values.
type fileConfig struct {
	ScaleHost      *string `json:"scale_host"`
	ScalePort      *string `json:"scale_port"`
	IngestorURL    *string `json:"ingestor_url"`
	DeviceID       *string `json:"device_id"`
	APIKey         *string `json:"api_key"`
	OTELEndpoint   *string `json:"otel_endpoint"`
	DebounceMs     *int    `json:"debounce_ms"`
	MinWeightKg    *int    `json:"min_weight_kg"`
	ReconnectSec   *int    `json:"reconnect_sec"`
	LogLevel       *string `json:"log_level"`
	HTTPTimeoutSec *int    `json:"http_timeout_sec"`
}

func (f *fileConfig) str(p *string, fallback string) string {
	if p != nil {
		return *p
	}
	return fallback
}

func (f *fileConfig) num(p *int, fallback int) int {
	if p != nil {
		return *p
	}
	return fallback
}

// readConfigFile parses a JSON config file. Missing file is silently ignored;
// a malformed file is a fatal error.
func readConfigFile(path string) *fileConfig {
	fc := &fileConfig{}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "config file %q: %v\n", path, err)
		}
		return fc
	}
	if err := json.Unmarshal(data, fc); err != nil {
		fmt.Fprintf(os.Stderr, "config file %q is invalid JSON: %v\n", path, err)
		os.Exit(1)
	}
	return fc
}

// configFilePath does a quick pre-scan of os.Args for --config / -config
// so the file can be loaded before flag.Parse() sets defaults.
func configFilePath() string {
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch {
		case (arg == "--config" || arg == "-config") && i+1 < len(os.Args):
			return os.Args[i+1]
		case strings.HasPrefix(arg, "--config="):
			return arg[len("--config="):]
		case strings.HasPrefix(arg, "-config="):
			return arg[len("-config="):]
		}
	}
	return envOr("CONFIG_FILE", "config.json")
}

func loadConfig() *config {
	path := configFilePath()
	fc := readConfigFile(path)

	cfg := &config{}
	flag.StringVar(&cfg.ConfigFile, "config", path,
		"Path to JSON config file (keys match long flag names with underscores)")
	flag.StringVar(&cfg.ScaleHost, "scale-host",
		envOr("SCALE_HOST", fc.str(fc.ScaleHost, "127.0.0.1")), "Scale TCP host")
	flag.StringVar(&cfg.ScalePort, "scale-port",
		envOr("SCALE_PORT", fc.str(fc.ScalePort, "5001")), "Scale TCP port")
	flag.StringVar(&cfg.IngestorURL, "ingestor-url",
		envOr("INGESTOR_URL", fc.str(fc.IngestorURL, "http://localhost:8090/ingest/event")), "Ingestor endpoint URL")
	flag.StringVar(&cfg.DeviceID, "device-id",
		envOr("DEVICE_ID", fc.str(fc.DeviceID, "")), "Device identifier (used in logs)")
	flag.StringVar(&cfg.APIKey, "api-key",
		envOr("API_KEY", fc.str(fc.APIKey, "")), "API key sent as Bearer token to the gateway")
	flag.StringVar(&cfg.OTELEndpoint, "otel-endpoint",
		envOr("OTEL_ENDPOINT", fc.str(fc.OTELEndpoint, "localhost:4318")), "OTLP HTTP collector endpoint (host:port)")
	flag.IntVar(&cfg.DebounceMs, "debounce-ms",
		envOrInt("DEBOUNCE_MS", fc.num(fc.DebounceMs, 2000)), "Milliseconds without a weight change before the session peak is dispatched")
	flag.IntVar(&cfg.MinWeightKg, "min-weight-kg",
		envOrInt("MIN_WEIGHT_KG", fc.num(fc.MinWeightKg, 0)), "Minimum weight in kg to report (0 = report all)")
	flag.IntVar(&cfg.ReconnectSec, "reconnect-sec",
		envOrInt("RECONNECT_SEC", fc.num(fc.ReconnectSec, 5)), "Seconds to wait before re-connecting to the scale after a disconnect")
	flag.StringVar(&cfg.LogLevel, "log-level",
		envOr("LOG_LEVEL", fc.str(fc.LogLevel, "info")), "Log level: debug, info, warn, error")
	flag.IntVar(&cfg.HTTPTimeoutSec, "http-timeout-sec",
		envOrInt("HTTP_TIMEOUT_SEC", fc.num(fc.HTTPTimeoutSec, 10)), "Timeout in seconds for HTTP requests to the ingestor")
	flag.Parse()

	return cfg
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "env %s=%q is not a valid integer, using default %d\n", key, v, fallback)
		return fallback
	}
	return n
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

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// newLogger returns a slog.Logger that writes JSON to stdout AND ships logs to
// the OTel collector via the global LoggerProvider.
func newLogger(level slog.Level) *slog.Logger {
	stdout := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
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

var httpClient *http.Client

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
		req.Header.Set("X-API-Key", cfg.APIKey)
	}

	start := time.Now()
	resp, err := httpClient.Do(req)
	latency := time.Since(start)

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	slog.DebugContext(ctx, "ingestor response",
		slog.Int("status", resp.StatusCode),
		slog.String("latency", latency.Round(time.Millisecond).String()),
		slog.Int("weight_kg", weightKg),
	)

	if resp.StatusCode >= 300 {
		return fmt.Errorf("ingestor returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// runSender reads from the weights channel and dispatches stable peak readings.
//
// Any weight change restarts the debounce timer. When the timer fires (no new
// readings for DebounceMs), the maximum weight observed since the last dispatch
// is sent. This avoids both premature sends (scale noise mid-load) and missed
// peaks (oscillation around the true max). The session resets when weight
// drops back to zero so the next vehicle starts fresh.
func runSender(ctx context.Context, cfg *config, weights <-chan int) {
	debounce := time.Duration(cfg.DebounceMs) * time.Millisecond

	var lastSeen int
	var peak     int  // max weight seen since last session reset
	var sent     bool // true after a successful dispatch; reset only on zero-crossing
	var timerC <-chan time.Time

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

			// Truck left the scale: reset session so the next vehicle starts clean.
			if w == 0 || (cfg.MinWeightKg > 0 && w < cfg.MinWeightKg) {
				if peak > 0 {
					slog.InfoContext(ctx, "session reset",
						slog.Int("peak_kg", peak),
						slog.Bool("was_sent", sent),
					)
				}
				peak = 0
				sent = false
				timerC = nil
				continue
			}

			isNewSession := peak == 0
			if w > peak {
				peak = w
			}

			if isNewSession {
				slog.InfoContext(ctx, "session started",
					slog.Int("weight_kg", w),
					slog.String("device_id", cfg.DeviceID),
				)
			} else {
				slog.DebugContext(ctx, "weight update",
					slog.Int("weight_kg", w),
					slog.Int("peak_kg", peak),
				)
			}

			t := time.NewTimer(debounce)
			timerC = t.C

		case <-timerC:
			timerC = nil
			if sent || peak < cfg.MinWeightKg {
				// Already dispatched once this session — ignore further stable readings.
				continue
			}
			if err := sendWeight(ctx, cfg, peak); err != nil {
				slog.ErrorContext(ctx, "failed to send weight",
					slog.Int("weight_kg", peak),
					slog.String("error", err.Error()),
				)
			} else {
				sent = true
				slog.InfoContext(ctx, "weight dispatched",
					slog.Int("weight_kg", peak),
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
				slog.DebugContext(ctx, "reading dropped (channel full)", slog.Int("weight_kg", w))
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

	slog.SetDefault(newLogger(parseLogLevel(cfg.LogLevel)))

	httpClient = &http.Client{Timeout: time.Duration(cfg.HTTPTimeoutSec) * time.Second}

	if cfg.APIKey == "" {
		slog.Warn("API_KEY is not set — requests will be sent without authentication")
	}

	slog.Info("scale-daemon starting",
		slog.String("version", version),
		slog.String("config_file", cfg.ConfigFile),
		slog.String("scale_addr", net.JoinHostPort(cfg.ScaleHost, cfg.ScalePort)),
		slog.String("ingestor_url", cfg.IngestorURL),
		slog.String("device_id", cfg.DeviceID),
		slog.Int("debounce_ms", cfg.DebounceMs),
		slog.Int("min_weight_kg", cfg.MinWeightKg),
		slog.Int("reconnect_sec", cfg.ReconnectSec),
		slog.Int("http_timeout_sec", cfg.HTTPTimeoutSec),
		slog.String("log_level", cfg.LogLevel),
		slog.String("otel_endpoint", cfg.OTELEndpoint),
	)

	// Buffer allows the reader to stay ahead without blocking on slow sends.
	weights := make(chan int, 32)

	go runReader(ctx, cfg, weights)
	runSender(ctx, cfg, weights) // blocks until ctx is cancelled

	slog.Info("scale-daemon stopped")
}
