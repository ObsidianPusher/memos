package v1

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"

	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/store"
)

func (s *APIV1Service) GetWorkspaceProfile(ctx context.Context, _ *v1pb.GetWorkspaceProfileRequest) (*v1pb.WorkspaceProfile, error) {
	workspaceProfile := &v1pb.WorkspaceProfile{
		Version:     s.Profile.Version,
		Mode:        s.Profile.Mode,
		InstanceUrl: s.Profile.InstanceURL,
	}
	owner, err := s.GetInstanceOwner(ctx)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get instance owner", err)
	}
	if owner != nil {
		workspaceProfile.Owner = owner.Name
	}
	return workspaceProfile, nil
}

var ownerCache *v1pb.User

func (s *APIV1Service) GetInstanceOwner(ctx context.Context) (*v1pb.User, error) {
	if ownerCache != nil {
		return ownerCache, nil
	}

	hostUserType := store.RoleHost
	user, err := s.Store.GetUser(ctx, &store.FindUser{
		Role: &hostUserType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find owner: %w", err)
	}
	if user == nil {
		return nil, nil
	}

	ownerCache = convertUserFromStore(user)
	return ownerCache, nil
}
