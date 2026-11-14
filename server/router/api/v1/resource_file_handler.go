package v1

import (
	"bytes"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/labstack/echo/v4"
	"google.golang.org/grpc/status"

	"github.com/usememos/memos/internal/util"
	storepb "github.com/usememos/memos/proto/gen/store"
	"github.com/usememos/memos/store"
)

func (s *APIV1Service) handleResourceFile(c echo.Context) error {
	ctx := c.Request().Context()
	resourceUID := c.Param("uid")
	if resourceUID == "" {
		return c.NoContent(http.StatusBadRequest)
	}

	resource, err := s.Store.GetResource(ctx, &store.FindResource{UID: &resourceUID})
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	if resource == nil {
		return c.NoContent(http.StatusNotFound)
	}

	if err := s.checkResourceAccess(ctx, resource, func() (*store.User, error) {
		user, err := s.getCurrentUserFromRequest(ctx, c.Request())
		if err != nil {
			return nil, nil
		}
		return user, nil
	}); err != nil {
		return c.NoContent(runtime.HTTPStatusFromCode(status.Code(err)))
	}

	if c.QueryParam("thumbnail") == "true" && util.HasPrefixes(resource.Type, SupportedThumbnailMimeTypes...) {
		if path, err := s.getOrGenerateThumbnailPath(ctx, resource); err == nil {
			return c.File(path)
		}
	}

	switch resource.StorageType {
	case storepb.ResourceStorageType_LOCAL:
		resourcePath := filepath.FromSlash(resource.Reference)
		if !filepath.IsAbs(resourcePath) {
			resourcePath = filepath.Join(s.Profile.Data, resourcePath)
		}
		return c.File(resourcePath)
	case storepb.ResourceStorageType_EXTERNAL, storepb.ResourceStorageType_S3:
		if resource.Reference == "" {
			return c.NoContent(http.StatusNotFound)
		}
		return c.Redirect(http.StatusFound, resource.Reference)
	default:
		if resource.Blob == nil {
			resource, err = s.Store.GetResource(ctx, &store.FindResource{UID: &resourceUID, GetBlob: true})
			if err != nil {
				return c.NoContent(http.StatusInternalServerError)
			}
			if resource == nil {
				return c.NoContent(http.StatusNotFound)
			}
		}
		contentType := resource.Type
		if strings.HasPrefix(contentType, "text/") {
			contentType += "; charset=utf-8"
		}
		// Prevent XSS attacks by serving potentially unsafe files with a content type that prevents script execution.
		if strings.EqualFold(contentType, "image/svg+xml") ||
			strings.EqualFold(contentType, "text/html") ||
			strings.EqualFold(contentType, "application/xhtml+xml") {
			contentType = "application/octet-stream"
		}
		c.Response().Header().Set(echo.HeaderContentType, contentType)
		http.ServeContent(c.Response(), c.Request(), resource.Filename, time.Unix(resource.UpdatedTs, 0), bytes.NewReader(resource.Blob))
		return nil
	}
}
