// Command gpu-exporter publishes GPU and VRAM metrics for Prometheus.
//
// It belongs to the compute plane, not the Control Plane: the GPU is on VM5 and
// only a process with the NVIDIA runtime can see it (ARCHITECTURE-v1 section 9).
// It shells out to nvidia-smi rather than binding NVML, because the alternative
// is a cgo dependency and a driver version to match for a handful of numbers.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// query is the field list, in the order the parser expects.
const query = "index,name,uuid,memory.total,memory.used,memory.free,utilization.gpu,utilization.memory,temperature.gpu"

type reading struct {
	Index       string
	Name        string
	UUID        string
	MemoryTotal float64
	MemoryUsed  float64
	MemoryFree  float64
	Utilisation float64
	MemoryUtil  float64
	Temperature float64
}

type exporter struct {
	log      *slog.Logger
	timeout  time.Duration
	mu       sync.RWMutex
	readings []reading
	lastErr  error
	scrapes  int
	failures int
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	addr := env("GPU_EXPORTER_ADDR", ":9401")
	interval, err := time.ParseDuration(env("GPU_EXPORTER_INTERVAL", "10s"))
	if err != nil {
		log.Error("invalid GPU_EXPORTER_INTERVAL", "error", err)
		os.Exit(1)
	}
	timeout, err := time.ParseDuration(env("GPU_EXPORTER_TIMEOUT", "5s"))
	if err != nil {
		log.Error("invalid GPU_EXPORTER_TIMEOUT", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	collector := &exporter{log: log, timeout: timeout}
	// Collect once before serving so the first scrape is not empty.
	collector.collect(ctx)
	go collector.run(ctx, interval)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /metrics", collector.serve)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		// Liveness does not depend on the GPU: a driver problem is something to
		// report, not a reason to have the container restarted in a loop.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Info("gpu exporter listening", "address", addr, "interval", interval.String())
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("gpu exporter stopped", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
	log.Info("gpu exporter stopped cleanly")
}

// run polls on a timer rather than on scrape, so a slow or hung nvidia-smi cannot
// stall Prometheus.
func (e *exporter) run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.collect(ctx)
		}
	}
}

func (e *exporter) collect(ctx context.Context) {
	callCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	output, err := exec.CommandContext(callCtx, "nvidia-smi",
		"--query-gpu="+query, "--format=csv,noheader,nounits").Output()

	e.mu.Lock()
	defer e.mu.Unlock()
	e.scrapes++
	if err != nil {
		e.failures++
		e.lastErr = err
		// The previous readings are kept: a transient failure should show as a
		// stale scrape rather than a GPU that appears to have vanished.
		e.log.Error("nvidia-smi failed", "error", err)
		return
	}
	e.lastErr = nil
	e.readings = parse(string(output), e.log)
}

func parse(output string, log *slog.Logger) []reading {
	var readings []reading
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) < 9 {
			log.Warn("unexpected nvidia-smi row", "row", line)
			continue
		}
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}
		readings = append(readings, reading{
			Index: fields[0], Name: fields[1], UUID: fields[2],
			MemoryTotal: number(fields[3]), MemoryUsed: number(fields[4]), MemoryFree: number(fields[5]),
			Utilisation: number(fields[6]), MemoryUtil: number(fields[7]), Temperature: number(fields[8]),
		})
	}
	return readings
}

// number tolerates the "[N/A]" nvidia-smi emits for unsupported fields on older
// cards, which is exactly what a Quadro P620 does for several of them.
func number(field string) float64 {
	value, err := strconv.ParseFloat(field, 64)
	if err != nil {
		return -1
	}
	return value
}

func (e *exporter) serve(w http.ResponseWriter, _ *http.Request) {
	e.mu.RLock()
	readings := append([]reading(nil), e.readings...)
	scrapes, failures := e.scrapes, e.failures
	healthy := 0.0
	if e.lastErr == nil {
		healthy = 1
	}
	e.mu.RUnlock()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	writeMetric(w, "gpu_exporter_up", "gauge", "Whether the last nvidia-smi call succeeded.", "", healthy)
	writeMetric(w, "gpu_exporter_collections_total", "counter", "nvidia-smi calls attempted.", "", float64(scrapes))
	writeMetric(w, "gpu_exporter_failures_total", "counter", "nvidia-smi calls that failed.", "", float64(failures))

	// Bytes rather than mebibytes: Prometheus convention is base units, and a
	// dashboard that has to know the unit from the metric name gets it wrong.
	series := []struct {
		name, kind, help string
		value            func(reading) float64
	}{
		{"gpu_memory_total_bytes", "gauge", "Total VRAM.", func(r reading) float64 { return r.MemoryTotal * 1024 * 1024 }},
		{"gpu_memory_used_bytes", "gauge", "VRAM in use.", func(r reading) float64 { return r.MemoryUsed * 1024 * 1024 }},
		{"gpu_memory_free_bytes", "gauge", "VRAM available.", func(r reading) float64 { return r.MemoryFree * 1024 * 1024 }},
		{"gpu_utilisation_ratio", "gauge", "GPU busy fraction.", func(r reading) float64 { return r.Utilisation / 100 }},
		{"gpu_memory_utilisation_ratio", "gauge", "Memory bandwidth busy fraction.", func(r reading) float64 { return r.MemoryUtil / 100 }},
		{"gpu_temperature_celsius", "gauge", "GPU temperature.", func(r reading) float64 { return r.Temperature }},
	}

	for _, metric := range series {
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n", metric.name, metric.help, metric.name, metric.kind)
		for _, r := range readings {
			value := metric.value(r)
			// A negative value marks a field this card does not report; emitting it
			// would show as a real reading on a graph.
			if value < 0 {
				continue
			}
			fmt.Fprintf(w, "%s{gpu=\"%s\",name=\"%s\",uuid=\"%s\"} %g\n",
				metric.name, r.Index, escape(r.Name), escape(r.UUID), value)
		}
	}
}

func writeMetric(w http.ResponseWriter, name, kind, help, labels string, value float64) {
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n%s%s %g\n", name, help, name, kind, name, labels, value)
}

func escape(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", " ")
	return replacer.Replace(value)
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
