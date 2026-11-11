package v1

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"runtime/debug"

	"github.com/labstack/echo/v4"

	"github.com/usememos/memos/store"
)

const (
	minMemoryLimitBytes int64 = 128 * 1024 * 1024       // 128 MiB
	maxMemoryLimitBytes int64 = 1024 * 1024 * 1024 * 64 // 64 GiB
)

type runtimeMemoryLimitResponse struct {
	Enabled          bool  `json:"enabled"`
	MemoryLimitBytes int64 `json:"memory_limit_bytes"`
	MemoryLimitMb    int64 `json:"memory_limit_mb"`
	UpdatedTs        int64 `json:"updated_ts"`
}

type upsertRuntimeMemoryLimitRequest struct {
	Enabled       bool  `json:"enabled"`
	MemoryLimitMb int64 `json:"memory_limit_mb"`
}

func (s *APIV1Service) getRuntimeMemoryLimit(c echo.Context) error {
	ctx := c.Request().Context()
	currentUser, err := s.getCurrentUserFromRequest(ctx, c.Request())
	if err != nil || currentUser == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
		})
	}
	if currentUser.Role != store.RoleHost {
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "permission denied",
		})
	}

	cfg, err := s.runtimeConfigStore.Get()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to get runtime memory limit config",
		})
	}
	return c.JSON(http.StatusOK, convertRuntimeMemoryLimitResponse(cfg))
}

func (s *APIV1Service) upsertRuntimeMemoryLimit(c echo.Context) error {
	ctx := c.Request().Context()
	currentUser, err := s.getCurrentUserFromRequest(ctx, c.Request())
	if err != nil || currentUser == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
		})
	}
	if currentUser.Role != store.RoleHost {
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "permission denied",
		})
	}

	request := &upsertRuntimeMemoryLimitRequest{}
	if err := json.NewDecoder(c.Request().Body).Decode(request); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
	}

	memoryLimitBytes := int64(-1)
	if request.Enabled {
		if request.MemoryLimitMb < minMemoryLimitBytes/MebiByte || request.MemoryLimitMb > maxMemoryLimitBytes/MebiByte {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("memory limit must be between %dMB and %dMB", minMemoryLimitBytes/MebiByte, maxMemoryLimitBytes/MebiByte),
			})
		}
		memoryLimitBytes = request.MemoryLimitMb * 1024 * 1024
	}

	cfg, err := s.runtimeConfigStore.Upsert(memoryLimitBytes)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to save runtime memory limit config",
		})
	}

	s.applyMemoryLimit(cfg.MemoryLimitBytes)
	return c.JSON(http.StatusOK, convertRuntimeMemoryLimitResponse(cfg))
}

func convertRuntimeMemoryLimitResponse(cfg *RuntimeConfig) *runtimeMemoryLimitResponse {
	if cfg == nil {
		return &runtimeMemoryLimitResponse{
			Enabled:          false,
			MemoryLimitBytes: -1,
			MemoryLimitMb:    0,
		}
	}
	enabled := cfg.MemoryLimitBytes > 0
	memoryLimitMb := int64(0)
	if enabled {
		memoryLimitMb = cfg.MemoryLimitBytes / (1024 * 1024)
	}
	return &runtimeMemoryLimitResponse{
		Enabled:          enabled,
		MemoryLimitBytes: cfg.MemoryLimitBytes,
		MemoryLimitMb:    memoryLimitMb,
		UpdatedTs:        cfg.UpdatedTs,
	}
}

func (s *APIV1Service) applyMemoryLimit(memoryLimitBytes int64) int64 {
	if memoryLimitBytes <= 0 {
		memoryLimitBytes = math.MaxInt64
	}
	debug.SetMemoryLimit(memoryLimitBytes)
	return memoryLimitBytes
}
