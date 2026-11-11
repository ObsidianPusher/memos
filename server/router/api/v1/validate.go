package v1

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/usememos/memos/internal/base"
	"github.com/usememos/memos/store"
)

func requireFieldMaskPaths(updateMask *fieldmaskpb.FieldMask, name string) error {
	if updateMask == nil || len(updateMask.Paths) == 0 {
		return invalidArgErrf("%s is required", name)
	}
	return nil
}

func validateUsername(username string) error {
	if !base.UIDMatcher.MatchString(username) {
		return invalidArgErrf("invalid username: %s", username)
	}
	return nil
}

func extractUserID(name string) (int32, error) {
	userID, err := ExtractUserIDFromName(name)
	if err != nil {
		return 0, invalidArgErrf("invalid user name: %v", err)
	}
	return userID, nil
}

func extractResourceUID(name string) (string, error) {
	resourceUID, err := ExtractResourceUIDFromName(name)
	if err != nil {
		return "", invalidArgErrf("invalid resource id: %v", err)
	}
	return resourceUID, nil
}

func (s *APIV1Service) getCurrentUserByParent(ctx context.Context, parent string) (*store.User, int32, error) {
	userID, err := ExtractUserIDFromName(parent)
	if err != nil {
		return nil, 0, invalidArgErrf("invalid user name: %v", err)
	}

	currentUser, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, 0, grpcErr(codes.Internal, "failed to get current user", err)
	}
	if currentUser == nil || currentUser.ID != userID {
		return nil, 0, grpcErr(codes.PermissionDenied, "permission denied", nil)
	}

	return currentUser, userID, nil
}
