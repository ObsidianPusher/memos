package v1

import (
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/store"
	"math"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"
)

func TestApplyMemoryLimitDisablesExistingLimit(t *testing.T) {
	previous := debug.SetMemoryLimit(256 << 20)
	t.Cleanup(func() { debug.SetMemoryLimit(previous) })
	(&APIV1Service{}).applyMemoryLimit(-1)
	if got := debug.SetMemoryLimit(-1); got != math.MaxInt64 {
		t.Fatalf("disabled limit = %d, want MaxInt64", got)
	}
}

func TestApplyMemoryLimitReturnsAppliedLimit(t *testing.T) {
	previous := debug.SetMemoryLimit(math.MaxInt64)
	t.Cleanup(func() { debug.SetMemoryLimit(previous) })
	if got := (&APIV1Service{}).applyMemoryLimit(256 << 20); got != 256<<20 {
		t.Fatalf("reported limit = %d, want 256 MiB", got)
	}
}

func TestRuntimeMemoryLimitRejectsOverflowingMegabytes(t *testing.T) {
	s, ctx, user := testAPI(t)
	role := store.RoleHost
	_, err := s.Store.UpdateUser(ctx, &store.UpdateUser{ID: user.ID, Role: &role})
	require.NoError(t, err)
	token, err := GenerateAccessToken(user.Username, user.ID, time.Time{}, []byte(s.Secret))
	require.NoError(t, err)
	require.NoError(t, s.UpsertAccessTokenToStore(ctx, user, token, "test"))
	s.runtimeConfigStore = newRuntimeConfigStore(filepath.Join(t.TempDir(), "runtime.json"))
	previous := debug.SetMemoryLimit(-1)
	t.Cleanup(func() { debug.SetMemoryLimit(previous) })
	request := httptest.NewRequest(http.MethodPut, "/api/v1/runtime/memory-limit", strings.NewReader(`{"enabled":true,"memory_limit_mb":17592186044544}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	require.NoError(t, s.upsertRuntimeMemoryLimit(echo.New().NewContext(request, response)))
	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Equal(t, previous, debug.SetMemoryLimit(-1))
	cfg, err := s.runtimeConfigStore.Get()
	require.NoError(t, err)
	require.Equal(t, int64(-1), cfg.MemoryLimitBytes)
}
