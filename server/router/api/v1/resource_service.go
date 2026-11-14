package v1

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/lithammer/shortuuid/v4"
	"google.golang.org/genproto/googleapis/api/httpbody"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/usememos/memos/internal/profile"
	"github.com/usememos/memos/internal/util"
	"github.com/usememos/memos/plugin/storage/s3"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	storepb "github.com/usememos/memos/proto/gen/store"
	"github.com/usememos/memos/store"
)

const (
	// The upload memory buffer is 32 MiB.
	// It should be kept low, so RAM usage doesn't get out of control.
	// This is unrelated to maximum upload size limit, which is now set through system setting.
	MaxUploadBufferSizeBytes = 32 << 20
	MebiByte                 = 1024 * 1024
	// ThumbnailCacheFolder is the folder name where the thumbnail images are stored.
	ThumbnailCacheFolder = ".thumbnail_cache"
)

var SupportedThumbnailMimeTypes = []string{
	"image/png",
	"image/jpeg",
}

func (s *APIV1Service) CreateResource(ctx context.Context, request *v1pb.CreateResourceRequest) (*v1pb.Resource, error) {
	if request == nil || request.Resource == nil {
		return nil, invalidArgErrf("resource is required")
	}
	user, err := s.getRequiredCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(request.Resource.Filename) == "" {
		return nil, invalidArgErrf("filename is required")
	}

	create := &store.Resource{
		UID:       shortuuid.New(),
		CreatorID: user.ID,
		Filename:  request.Resource.Filename,
		Type:      request.Resource.Type,
	}

	workspaceStorageSetting, err := s.Store.GetWorkspaceStorageSetting(ctx)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get workspace storage setting", err)
	}
	size := binary.Size(request.Resource.Content)
	uploadSizeLimit := int(workspaceStorageSetting.UploadSizeLimitMb) * MebiByte
	if uploadSizeLimit == 0 {
		uploadSizeLimit = MaxUploadBufferSizeBytes
	}
	if size > uploadSizeLimit {
		return nil, invalidArgErrf("file size exceeds the limit")
	}
	create.Size = int64(size)
	create.Blob = request.Resource.Content
	if err := SaveResourceBlob(ctx, s.Profile, s.Store, create); err != nil {
		return nil, grpcErr(codes.Internal, "failed to save resource blob", err)
	}

	if request.Resource.Memo != nil {
		memoUID, err := ExtractMemoUIDFromName(*request.Resource.Memo)
		if err != nil {
			return nil, invalidArgErrf("invalid memo name: %v", err)
		}
		memo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
		if err != nil {
			return nil, grpcErr(codes.Internal, "failed to find memo", err)
		}
		if memo == nil {
			return nil, grpcErr(codes.NotFound, "memo not found", nil)
		}
		create.MemoID = &memo.ID
	}
	resource, err := s.Store.CreateResource(ctx, create)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to create resource", err)
	}

	return s.convertResourceFromStore(ctx, resource), nil
}

func (s *APIV1Service) ListResources(ctx context.Context, _ *v1pb.ListResourcesRequest) (*v1pb.ListResourcesResponse, error) {
	user, err := s.getRequiredCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	resources, err := s.Store.ListResources(ctx, &store.FindResource{
		CreatorID: &user.ID,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to list resources", err)
	}

	response := &v1pb.ListResourcesResponse{}
	for _, resource := range resources {
		response.Resources = append(response.Resources, s.convertResourceFromStore(ctx, resource))
	}
	return response, nil
}

func (s *APIV1Service) GetResource(ctx context.Context, request *v1pb.GetResourceRequest) (*v1pb.Resource, error) {
	resourceUID, err := extractResourceUID(request.Name)
	if err != nil {
		return nil, err
	}
	resource, err := s.Store.GetResource(ctx, &store.FindResource{UID: &resourceUID})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get resource", err)
	}
	if resource == nil {
		return nil, grpcErr(codes.NotFound, "resource not found", nil)
	}
	return s.convertResourceFromStore(ctx, resource), nil
}

func (s *APIV1Service) GetResourceBinary(ctx context.Context, request *v1pb.GetResourceBinaryRequest) (*httpbody.HttpBody, error) {
	resourceUID, err := extractResourceUID(request.Name)
	if err != nil {
		return nil, err
	}
	resource, err := s.Store.GetResource(ctx, &store.FindResource{
		GetBlob: true,
		UID:     &resourceUID,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get resource", err)
	}
	if resource == nil {
		return nil, grpcErr(codes.NotFound, "resource not found", nil)
	}
	if err := s.checkResourceAccess(ctx, resource, func() (*store.User, error) { return s.GetCurrentUser(ctx) }); err != nil {
		return nil, err
	}

	if request.Thumbnail && util.HasPrefixes(resource.Type, SupportedThumbnailMimeTypes...) {
		thumbnailBlob, err := s.getOrGenerateThumbnail(ctx, resource)
		if err != nil {
			// thumbnail failures are logged as warnings and not cosidered critical failures as
			// a resource image can be used in its place.
			slog.Warn("failed to get resource thumbnail image", slog.Any("error", err))
		} else {
			return &httpbody.HttpBody{
				ContentType: resource.Type,
				Data:        thumbnailBlob,
			}, nil
		}
	}

	blob, err := s.GetResourceBlob(resource)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get resource blob", err)
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

	return &httpbody.HttpBody{
		ContentType: contentType,
		Data:        blob,
	}, nil
}

func (s *APIV1Service) UpdateResource(ctx context.Context, request *v1pb.UpdateResourceRequest) (*v1pb.Resource, error) {
	if request == nil || request.Resource == nil {
		return nil, invalidArgErrf("resource is required")
	}
	resourceUID, err := extractResourceUID(request.Resource.Name)
	if err != nil {
		return nil, err
	}
	if err := requireFieldMaskPaths(request.UpdateMask, "update mask"); err != nil {
		return nil, err
	}
	resource, err := s.Store.GetResource(ctx, &store.FindResource{UID: &resourceUID})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get resource", err)
	}
	if resource == nil {
		return nil, grpcErr(codes.NotFound, "resource not found", nil)
	}
	user, err := s.getRequiredCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	if resource.CreatorID != user.ID {
		return nil, grpcErr(codes.PermissionDenied, "permission denied", nil)
	}

	currentTs := time.Now().Unix()
	update := &store.UpdateResource{
		ID:        resource.ID,
		UpdatedTs: &currentTs,
	}
	for _, field := range request.UpdateMask.Paths {
		if field == "filename" {
			update.Filename = &request.Resource.Filename
		} else {
			return nil, invalidArgErrf("invalid update path: %s", field)
		}
	}

	if err := s.Store.UpdateResource(ctx, update); err != nil {
		return nil, grpcErr(codes.Internal, "failed to update resource", err)
	}
	return s.GetResource(ctx, &v1pb.GetResourceRequest{
		Name: request.Resource.Name,
	})
}

func (s *APIV1Service) DeleteResource(ctx context.Context, request *v1pb.DeleteResourceRequest) (*emptypb.Empty, error) {
	resourceUID, err := extractResourceUID(request.Name)
	if err != nil {
		return nil, err
	}
	user, err := s.getRequiredCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	resource, err := s.Store.GetResource(ctx, &store.FindResource{
		UID:       &resourceUID,
		CreatorID: &user.ID,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to find resource", err)
	}
	if resource == nil {
		return nil, grpcErr(codes.NotFound, "resource not found", nil)
	}
	// Delete the resource from the database.
	if err := s.Store.DeleteResource(ctx, &store.DeleteResource{
		ID: resource.ID,
	}); err != nil {
		return nil, grpcErr(codes.Internal, "failed to delete resource", err)
	}
	return &emptypb.Empty{}, nil
}

func (s *APIV1Service) convertResourceFromStore(ctx context.Context, resource *store.Resource) *v1pb.Resource {
	resourceMessage := &v1pb.Resource{
		Name:       fmt.Sprintf("%s%s", ResourceNamePrefix, resource.UID),
		CreateTime: timestamppb.New(time.Unix(resource.CreatedTs, 0)),
		Filename:   resource.Filename,
		Type:       resource.Type,
		Size:       resource.Size,
	}
	if resource.StorageType == storepb.ResourceStorageType_EXTERNAL || resource.StorageType == storepb.ResourceStorageType_S3 {
		resourceMessage.ExternalLink = resource.Reference
	}
	if resource.MemoID != nil {
		memo, _ := s.Store.GetMemo(ctx, &store.FindMemo{
			ID: resource.MemoID,
		})
		if memo != nil {
			memoName := fmt.Sprintf("%s%s", MemoNamePrefix, memo.UID)
			resourceMessage.Memo = &memoName
		}
	}

	return resourceMessage
}

// SaveResourceBlob save the blob of resource based on the storage config.
func SaveResourceBlob(ctx context.Context, profile *profile.Profile, stores *store.Store, create *store.Resource) error {
	workspaceStorageSetting, err := stores.GetWorkspaceStorageSetting(ctx)
	if err != nil {
		return fmt.Errorf("failed to find workspace storage setting: %w", err)
	}

	if workspaceStorageSetting.StorageType == storepb.WorkspaceStorageSetting_LOCAL {
		filepathTemplate := "assets/{timestamp}_{filename}"
		if workspaceStorageSetting.FilepathTemplate != "" {
			filepathTemplate = workspaceStorageSetting.FilepathTemplate
		}

		internalPath := filepathTemplate
		if !strings.Contains(internalPath, "{filename}") {
			internalPath = filepath.Join(internalPath, "{filename}")
		}
		internalPath = replaceFilenameWithPathTemplate(internalPath, create.Filename)
		internalPath = filepath.ToSlash(internalPath)

		// Ensure the directory exists.
		osPath := filepath.FromSlash(internalPath)
		if !filepath.IsAbs(osPath) {
			osPath = filepath.Join(profile.Data, osPath)
		}
		dir := filepath.Dir(osPath)
		if err = os.MkdirAll(dir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
		// Write the blob to the file.
		if err := os.WriteFile(osPath, create.Blob, 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
		create.Reference = internalPath
		create.Blob = nil
		create.StorageType = storepb.ResourceStorageType_LOCAL
	} else if workspaceStorageSetting.StorageType == storepb.WorkspaceStorageSetting_S3 {
		s3Config := workspaceStorageSetting.S3Config
		if s3Config == nil {
			return errors.New("no active external storage found")
		}
		s3Client, err := s3.NewClient(ctx, s3Config)
		if err != nil {
			return fmt.Errorf("failed to create s3 client: %w", err)
		}

		filepathTemplate := workspaceStorageSetting.FilepathTemplate
		if !strings.Contains(filepathTemplate, "{filename}") {
			filepathTemplate = filepath.Join(filepathTemplate, "{filename}")
		}
		filepathTemplate = replaceFilenameWithPathTemplate(filepathTemplate, create.Filename)
		key, err := s3Client.UploadObject(ctx, filepathTemplate, create.Type, bytes.NewReader(create.Blob))
		if err != nil {
			return fmt.Errorf("failed to upload via s3 client: %w", err)
		}
		presignURL, err := s3Client.PresignGetObject(ctx, key)
		if err != nil {
			return fmt.Errorf("failed to presign via s3 client: %w", err)
		}

		create.Reference = presignURL
		create.Blob = nil
		create.StorageType = storepb.ResourceStorageType_S3
		create.Payload = &storepb.ResourcePayload{
			Payload: &storepb.ResourcePayload_S3Object_{
				S3Object: &storepb.ResourcePayload_S3Object{
					S3Config:          s3Config,
					Key:               key,
					LastPresignedTime: timestamppb.New(time.Now()),
				},
			},
		}
	}

	return nil
}

func (s *APIV1Service) GetResourceBlob(resource *store.Resource) ([]byte, error) {
	// For local storage, read the file from the local disk.
	if resource.StorageType == storepb.ResourceStorageType_LOCAL {
		resourcePath := filepath.FromSlash(resource.Reference)
		if !filepath.IsAbs(resourcePath) {
			resourcePath = filepath.Join(s.Profile.Data, resourcePath)
		}

		file, err := os.Open(resourcePath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("file not found: %w", err)
			}
			return nil, fmt.Errorf("failed to open the file: %w", err)
		}
		defer file.Close()
		blob, err := io.ReadAll(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read the file: %w", err)
		}
		return blob, nil
	}
	// For database storage, return the blob from the database.
	return resource.Blob, nil
}

const (
	// thumbnailMaxDimension bounds list previews independently of original image size.
	thumbnailMaxDimension = 512
)

// getOrGenerateThumbnail returns the thumbnail image of the resource.
func (s *APIV1Service) getOrGenerateThumbnail(ctx context.Context, resource *store.Resource) ([]byte, error) {
	filePath, err := s.getOrGenerateThumbnailPath(ctx, resource)
	if err != nil {
		return nil, err
	}
	blob, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read thumbnail file: %w", err)
	}
	return blob, nil
}

// getOrGenerateThumbnailPath returns the thumbnail file path of the resource.
func (s *APIV1Service) getOrGenerateThumbnailPath(ctx context.Context, resource *store.Resource) (string, error) {
	thumbnailCacheFolder := filepath.Join(s.Profile.Data, ThumbnailCacheFolder)
	if err := os.MkdirAll(thumbnailCacheFolder, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create thumbnail cache folder: %w", err)
	}
	filePath := filepath.Join(thumbnailCacheFolder, fmt.Sprintf("%d-v2%s", resource.ID, filepath.Ext(resource.Filename)))
	if _, err := os.Stat(filePath); err == nil {
		return filePath, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to check thumbnail image stat: %w", err)
	}

	// If thumbnail image does not exist, generate and save the thumbnail image.
	var img image.Image
	if resource.StorageType == storepb.ResourceStorageType_LOCAL {
		resourcePath := filepath.FromSlash(resource.Reference)
		if !filepath.IsAbs(resourcePath) {
			resourcePath = filepath.Join(s.Profile.Data, resourcePath)
		}
		file, err := os.Open(resourcePath)
		if err != nil {
			return "", fmt.Errorf("failed to open resource file: %w", err)
		}
		defer file.Close()
		decoded, err := imaging.Decode(file, imaging.AutoOrientation(true))
		if err != nil {
			return "", fmt.Errorf("failed to decode thumbnail image: %w", err)
		}
		img = decoded
	} else {
		// File requests initially fetch metadata only. Load database image bytes on
		// a cache miss rather than silently falling back to the full-size image.
		if resource.StorageType == storepb.ResourceStorageType_RESOURCE_STORAGE_TYPE_UNSPECIFIED && resource.Blob == nil {
			stored, err := s.Store.GetResource(ctx, &store.FindResource{ID: &resource.ID, GetBlob: true})
			if err != nil {
				return "", fmt.Errorf("failed to read thumbnail source: %w", err)
			}
			if stored == nil {
				return "", errors.New("thumbnail source not found")
			}
			resource = stored
		}
		blob, err := s.GetResourceBlob(resource)
		if err != nil {
			return "", fmt.Errorf("failed to get resource blob: %w", err)
		}
		decoded, err := imaging.Decode(bytes.NewReader(blob), imaging.AutoOrientation(true))
		if err != nil {
			return "", fmt.Errorf("failed to decode thumbnail image: %w", err)
		}
		img = decoded
	}

	thumbnailImage := imaging.Fit(img, thumbnailMaxDimension, thumbnailMaxDimension, imaging.Lanczos)
	// Publish only complete images so concurrent readers cannot see a partial cache file.
	temp, err := os.CreateTemp(thumbnailCacheFolder, "thumbnail-*"+filepath.Ext(resource.Filename))
	if err != nil {
		return "", fmt.Errorf("failed to create thumbnail file: %w", err)
	}
	defer os.Remove(temp.Name())
	if err := temp.Close(); err != nil {
		return "", err
	}
	if err := imaging.Save(thumbnailImage, temp.Name()); err != nil {
		return "", fmt.Errorf("failed to save thumbnail file: %w", err)
	}
	if err := os.Rename(temp.Name(), filePath); err != nil {
		return "", fmt.Errorf("failed to publish thumbnail file: %w", err)
	}
	return filePath, nil
}

var fileKeyPattern = regexp.MustCompile(`\{[a-z]{1,9}\}`)

func replaceFilenameWithPathTemplate(path, filename string) string {
	// Avoid path traversal and nested path injection in template replacements.
	filename = filepath.Base(strings.TrimSpace(filename))
	if filename == "." || filename == string(filepath.Separator) {
		filename = "resource.bin"
	}
	t := time.Now()
	path = fileKeyPattern.ReplaceAllStringFunc(path, func(s string) string {
		switch s {
		case "{filename}":
			return filename
		case "{timestamp}":
			return fmt.Sprintf("%d", t.Unix())
		case "{year}":
			return fmt.Sprintf("%d", t.Year())
		case "{month}":
			return fmt.Sprintf("%02d", t.Month())
		case "{day}":
			return fmt.Sprintf("%02d", t.Day())
		case "{hour}":
			return fmt.Sprintf("%02d", t.Hour())
		case "{minute}":
			return fmt.Sprintf("%02d", t.Minute())
		case "{second}":
			return fmt.Sprintf("%02d", t.Second())
		case "{uuid}":
			return util.GenUUID()
		}
		return s
	})
	return path
}

// Unbound and orphan uploads remain private to their uploader.
func (s *APIV1Service) checkResourceAccess(ctx context.Context, resource *store.Resource, getUser func() (*store.User, error)) error {
	ownerID := resource.CreatorID
	visibility := store.Private
	if resource.MemoID != nil {
		memo, err := s.Store.GetMemo(ctx, &store.FindMemo{ID: resource.MemoID})
		if err != nil {
			return grpcErr(codes.Internal, "failed to get memo", err)
		}
		if memo != nil {
			visibility = memo.Visibility
			ownerID = memo.CreatorID
		}
	}
	if visibility == store.Public {
		return nil
	}
	user, err := getUser()
	if err != nil {
		return grpcErr(codes.Internal, "failed to get current user", err)
	}
	if user == nil {
		return grpcErr(codes.Unauthenticated, "authentication required", nil)
	}
	if visibility != store.Protected && user.ID != ownerID {
		return grpcErr(codes.PermissionDenied, "permission denied", nil)
	}
	return nil
}
