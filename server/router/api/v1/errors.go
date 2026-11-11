package v1

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/usememos/memos/store"
)

func grpcErr(code codes.Code, message string, err error) error {
	if err == nil {
		return status.Error(code, message)
	}
	return status.Errorf(code, "%s: %v", message, err)
}

func invalidArgErrf(format string, args ...any) error {
	return status.Error(codes.InvalidArgument, fmt.Sprintf(format, args...))
}

func (s *APIV1Service) getRequiredCurrentUser(ctx context.Context) (*store.User, error) {
	user, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get current user", err)
	}
	if user == nil {
		return nil, grpcErr(codes.PermissionDenied, "permission denied", nil)
	}
	return user, nil
}
