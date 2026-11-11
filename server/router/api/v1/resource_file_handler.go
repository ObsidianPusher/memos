package v1

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"

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

	// Check the related memo visibility.
	if resource.MemoID != nil {
		memo, err := s.Store.GetMemo(ctx, &store.FindMemo{ID: resource.MemoID})
		if err != nil {
			return c.NoContent(http.StatusInternalServerError)
		}
		if memo != nil && memo.Visibility != store.Public {
			user, err := s.getCurrentUserFromRequest(ctx, c.Request())
			if err != nil || user == nil {
				return c.NoContent(http.StatusUnauthorized)
			}
			if memo.Visibility == store.Private && user.ID != resource.CreatorID {
				return c.NoContent(http.StatusUnauthorized)
			}
		}
	}

	if c.QueryParam("thumbnail") == "true" && util.HasPrefixes(resource.Type, SupportedThumbnailMimeTypes...) {
		if path, err := s.getOrGenerateThumbnailPath(resource); err == nil {
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
		return c.Blob(http.StatusOK, contentType, resource.Blob)
	}
}
