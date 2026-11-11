package v1

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"

	"github.com/usememos/memos/internal/util"
	storepb "github.com/usememos/memos/proto/gen/store"
	"github.com/usememos/memos/store"
)

// ContextKey is the key type of context value.
type ContextKey int

const (
	// The key name used to store username in the context
	// user id is extracted from the jwt token subject field.
	usernameContextKey ContextKey = iota
	accessTokenContextKey
)

// GRPCAuthInterceptor is the auth interceptor for gRPC server.
type GRPCAuthInterceptor struct {
	Store  *store.Store
	secret string
}

// NewGRPCAuthInterceptor returns a new API auth interceptor.
func NewGRPCAuthInterceptor(store *store.Store, secret string) *GRPCAuthInterceptor {
	return &GRPCAuthInterceptor{
		Store:  store,
		secret: secret,
	}
}

// AuthenticationInterceptor is the unary interceptor for gRPC API.
func (in *GRPCAuthInterceptor) AuthenticationInterceptor(ctx context.Context, request any, serverInfo *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, grpcErr(codes.Unauthenticated, "failed to parse metadata from incoming context", nil)
	}
	accessToken, err := getTokenFromMetadata(md)
	if err != nil {
		return nil, grpcErr(codes.Unauthenticated, "failed to get access token", err)
	}

	username, err := in.authenticate(ctx, accessToken)
	if err != nil {
		if isUnauthorizeAllowedMethod(serverInfo.FullMethod) {
			return handler(ctx, request)
		}
		return nil, err
	}
	user, err := in.Store.GetUser(ctx, &store.FindUser{
		Username: &username,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get user", err)
	}
	if user == nil {
		return nil, grpcErr(codes.Unauthenticated, "user not exists", nil)
	}
	if user.RowStatus == store.Archived {
		return nil, grpcErr(codes.Unauthenticated, "user is archived", nil)
	}
	if isOnlyForAdminAllowedMethod(serverInfo.FullMethod) && user.Role != store.RoleHost && user.Role != store.RoleAdmin {
		return nil, grpcErr(codes.PermissionDenied, "permission denied", nil)
	}

	ctx = context.WithValue(ctx, usernameContextKey, username)
	ctx = context.WithValue(ctx, accessTokenContextKey, accessToken)
	return handler(ctx, request)
}

func (in *GRPCAuthInterceptor) authenticate(ctx context.Context, accessToken string) (string, error) {
	if accessToken == "" {
		return "", grpcErr(codes.Unauthenticated, "access token not found", nil)
	}
	claims, err := parseAccessTokenClaims(accessToken, in.secret)
	if err != nil {
		return "", grpcErr(codes.Unauthenticated, "Invalid or expired access token", nil)
	}

	// We either have a valid access token or we will attempt to generate new access token.
	userID, err := util.ConvertStringToInt32(claims.Subject)
	if err != nil {
		return "", grpcErr(codes.Unauthenticated, "malformed ID in the token", err)
	}
	user, err := in.Store.GetUser(ctx, &store.FindUser{
		ID: &userID,
	})
	if err != nil {
		return "", grpcErr(codes.Internal, "failed to get user", err)
	}
	if user == nil {
		return "", grpcErr(codes.Unauthenticated, "user not exists", nil)
	}
	if user.RowStatus == store.Archived {
		return "", grpcErr(codes.Unauthenticated, "user is archived", nil)
	}

	accessTokens, err := in.Store.GetUserAccessTokens(ctx, user.ID)
	if err != nil {
		return "", grpcErr(codes.Internal, "failed to get user access tokens", err)
	}
	if !validateAccessToken(accessToken, accessTokens) {
		return "", grpcErr(codes.Unauthenticated, "invalid access token", nil)
	}

	return user.Username, nil
}

func getTokenFromMetadata(md metadata.MD) (string, error) {
	// Check the HTTP request header first.
	authorizationHeaders := md.Get("Authorization")
	if len(md.Get("Authorization")) > 0 {
		authHeaderParts := strings.Fields(authorizationHeaders[0])
		if len(authHeaderParts) != 2 || strings.ToLower(authHeaderParts[0]) != "bearer" {
			return "", errors.New("authorization header format must be Bearer {token}")
		}
		return authHeaderParts[1], nil
	}
	// Check the cookie header.
	var accessToken string
	for _, t := range append(md.Get("grpcgateway-cookie"), md.Get("cookie")...) {
		header := http.Header{}
		header.Add("Cookie", t)
		request := http.Request{Header: header}
		if v, _ := request.Cookie(AccessTokenCookieName); v != nil {
			accessToken = v.Value
		}
	}
	return accessToken, nil
}

func validateAccessToken(accessTokenString string, userAccessTokens []*storepb.AccessTokensUserSetting_AccessToken) bool {
	for _, userAccessToken := range userAccessTokens {
		if accessTokenString == userAccessToken.AccessToken {
			return true
		}
	}
	return false
}
