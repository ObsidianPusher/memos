package profiler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"runtime"
	"time"

	"github.com/labstack/echo/v4"
)

// Profiler provides HTTP endpoints for memory profiling.
type Profiler struct {
	memStatsLogInterval time.Duration
}

// NewProfiler creates a new profiler.
func NewProfiler() *Profiler {
	return &Profiler{
		memStatsLogInterval: 1 * time.Minute,
	}
}

// RegisterRoutes adds profiling endpoints to the Echo server.
func (*Profiler) RegisterRoutes(e *echo.Echo) {
	g := e.Group("/debug/pprof")
	g.GET("", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
	g.GET("/cmdline", echo.WrapHandler(http.HandlerFunc(pprof.Cmdline)))
	g.GET("/profile", echo.WrapHandler(http.HandlerFunc(pprof.Profile)))
	g.POST("/symbol", echo.WrapHandler(http.HandlerFunc(pprof.Symbol)))
	g.GET("/symbol", echo.WrapHandler(http.HandlerFunc(pprof.Symbol)))
	g.GET("/trace", echo.WrapHandler(http.HandlerFunc(pprof.Trace)))
	g.GET("/allocs", echo.WrapHandler(http.HandlerFunc(pprof.Handler("allocs").ServeHTTP)))
	g.GET("/block", echo.WrapHandler(http.HandlerFunc(pprof.Handler("block").ServeHTTP)))
	g.GET("/goroutine", echo.WrapHandler(http.HandlerFunc(pprof.Handler("goroutine").ServeHTTP)))
	g.GET("/heap", echo.WrapHandler(http.HandlerFunc(pprof.Handler("heap").ServeHTTP)))
	g.GET("/mutex", echo.WrapHandler(http.HandlerFunc(pprof.Handler("mutex").ServeHTTP)))
	g.GET("/threadcreate", echo.WrapHandler(http.HandlerFunc(pprof.Handler("threadcreate").ServeHTTP)))

	g.GET("/memstats", func(c echo.Context) error {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return c.JSON(http.StatusOK, map[string]interface{}{
			"alloc":       m.Alloc,
			"totalAlloc":  m.TotalAlloc,
			"sys":         m.Sys,
			"numGC":       m.NumGC,
			"heapAlloc":   m.HeapAlloc,
			"heapSys":     m.HeapSys,
			"heapInuse":   m.HeapInuse,
			"heapObjects": m.HeapObjects,
		})
	})
}

// StartMemoryMonitor keeps a lightweight background GC watchdog without noisy periodic logs.
func (p *Profiler) StartMemoryMonitor(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(p.memStatsLogInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)

				if m.HeapAlloc > 500*1024*1024 { // 500 MB threshold
					slog.Warn("forcing garbage collection due to high memory usage",
						"heapAlloc", formatBytes(m.HeapAlloc),
						"heapObjects", m.HeapObjects,
						"numGoroutine", runtime.NumGoroutine(),
					)
					runtime.GC()
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}

	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
