package v1

import (
	"context"
	"net/http"

	"google.golang.org/grpc/metadata"

	"github.com/usememos/memos/store"
)

func (s *APIV1Service) getCurrentUserFromRequest(ctx context.Context, r *http.Request) (*store.User, error) {
	token, err := getTokenFromRequest(r)
	if err != nil || token == "" {
		return nil, err
	}
	auth := NewGRPCAuthInterceptor(s.Store, s.Secret)
	username, err := auth.authenticate(ctx, token)
	if err != nil {
		return nil, err
	}
	user, err := s.Store.GetUser(ctx, &store.FindUser{
		Username: &username,
	})
	if err != nil {
		return nil, err
	}
	return user, nil
}

func getTokenFromRequest(r *http.Request) (string, error) {
	md := metadata.MD{}
	if auth := r.Header.Get("Authorization"); auth != "" {
		md.Set("Authorization", auth)
	}
	if cookie := r.Header.Get("Cookie"); cookie != "" {
		md.Set("cookie", cookie)
	}
	return getTokenFromMetadata(md)
}
