package v1

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/emptypb"

	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/store"
)

func (s *APIV1Service) SetMemoResources(ctx context.Context, request *v1pb.SetMemoResourcesRequest) (*emptypb.Empty, error) {
	err := s.Store.WithTransaction(ctx, func(stores *store.Store) error {
		scoped := *s
		scoped.Store = stores
		return scoped.setMemoResources(ctx, request)
	})
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *APIV1Service) setMemoResources(ctx context.Context, request *v1pb.SetMemoResourcesRequest) error {
	if request == nil {
		return invalidArgErrf("request is required")
	}
	memo, err := s.getOwnedMemo(ctx, request.Name)
	if err != nil {
		return err
	}
	// Validate every attachment before changing existing bindings.
	requested := make([]*store.Resource, 0, len(request.Resources))
	keep := make(map[int32]bool)
	for _, resource := range request.Resources {
		if resource == nil {
			return invalidArgErrf("resource is required")
		}
		uid, err := ExtractResourceUIDFromName(resource.Name)
		if err != nil {
			return invalidArgErrf("invalid resource name: %v", err)
		}
		stored, err := s.Store.GetResource(ctx, &store.FindResource{UID: &uid})
		if err != nil {
			return grpcErr(codes.Internal, "failed to get resource", err)
		}
		if stored == nil {
			return grpcErr(codes.NotFound, "resource not found", nil)
		}
		if stored.CreatorID != memo.CreatorID {
			return grpcErr(codes.PermissionDenied, "resource belongs to another user", nil)
		}
		if stored.MemoID != nil && *stored.MemoID != memo.ID {
			return grpcErr(codes.FailedPrecondition, "resource belongs to another memo", nil)
		}
		if !keep[stored.ID] {
			requested = append(requested, stored)
			keep[stored.ID] = true
		}
	}
	limit := int(^uint(0) >> 1)
	existing, err := s.Store.ListResources(ctx, &store.FindResource{MemoID: &memo.ID, Limit: &limit})
	if err != nil {
		return grpcErr(codes.Internal, "failed to list resources", err)
	}
	for _, resource := range existing {
		if !keep[resource.ID] {
			if err := s.Store.DeleteResource(ctx, &store.DeleteResource{ID: resource.ID, MemoID: &memo.ID}); err != nil {
				return grpcErr(codes.Internal, "failed to delete resource", err)
			}
		}
	}
	now := time.Now().Unix()
	for index, resource := range requested {
		updatedTs := now + int64(len(requested)-1-index)
		if err := s.Store.UpdateResource(ctx, &store.UpdateResource{ID: resource.ID, MemoID: &memo.ID, UpdatedTs: &updatedTs}); err != nil {
			return grpcErr(codes.Internal, "failed to update resource", err)
		}
	}
	return nil
}

func (s *APIV1Service) getOwnedMemo(ctx context.Context, name string) (*store.Memo, error) {
	uid, err := ExtractMemoUIDFromName(name)
	if err != nil {
		return nil, invalidArgErrf("invalid memo name: %v", err)
	}
	user, err := s.getRequiredCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	memo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &uid})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get memo", err)
	}
	if memo == nil {
		return nil, grpcErr(codes.NotFound, "memo not found", nil)
	}
	if memo.CreatorID != user.ID && !isSuperUser(user) {
		return nil, grpcErr(codes.PermissionDenied, "permission denied", nil)
	}
	return memo, nil
}

func (s *APIV1Service) ListMemoResources(ctx context.Context, request *v1pb.ListMemoResourcesRequest) (*v1pb.ListMemoResourcesResponse, error) {
	memoUID, err := ExtractMemoUIDFromName(request.Name)
	if err != nil {
		return nil, invalidArgErrf("invalid memo name: %v", err)
	}
	memo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get memo", err)
	}
	if memo == nil {
		return nil, grpcErr(codes.NotFound, "memo not found", nil)
	}
	resources, err := s.Store.ListResources(ctx, &store.FindResource{
		MemoID: &memo.ID,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to list resources", err)
	}

	response := &v1pb.ListMemoResourcesResponse{
		Resources: []*v1pb.Resource{},
	}
	for _, resource := range resources {
		response.Resources = append(response.Resources, s.convertResourceFromStore(ctx, resource))
	}
	return response, nil
}
