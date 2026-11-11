package v1

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/usememos/memos/internal/util"
	"github.com/usememos/memos/plugin/idp"
	"github.com/usememos/memos/plugin/idp/oauth2"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	storepb "github.com/usememos/memos/proto/gen/store"
	"github.com/usememos/memos/store"
)

const (
	unmatchedUsernameAndPasswordError = "unmatched username and password"
)

func (s *APIV1Service) GetAuthStatus(ctx context.Context, _ *v1pb.GetAuthStatusRequest) (*v1pb.User, error) {
	user, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, grpcErr(codes.Unauthenticated, "failed to get current user", err)
	}
	if user == nil {
		// Set the cookie header to expire access token.
		if err := s.clearAccessTokenCookie(ctx); err != nil {
			return nil, grpcErr(codes.Internal, "failed to set grpc header", err)
		}
		return nil, grpcErr(codes.Unauthenticated, "user not found", nil)
	}
	return convertUserFromStore(user), nil
}

func (s *APIV1Service) SignIn(ctx context.Context, request *v1pb.SignInRequest) (*v1pb.User, error) {
	if request == nil {
		return nil, invalidArgErrf("request is required")
	}
	var existingUser *store.User
	if passwordCredentials := request.GetPasswordCredentials(); passwordCredentials != nil {
		user, err := s.Store.GetUser(ctx, &store.FindUser{
			Username: &passwordCredentials.Username,
		})
		if err != nil {
			return nil, grpcErr(codes.Internal, "failed to get user", err)
		}
		if user == nil {
			return nil, grpcErr(codes.InvalidArgument, unmatchedUsernameAndPasswordError, nil)
		}
		// Compare the stored hashed password, with the hashed version of the password that was received.
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(passwordCredentials.Password)); err != nil {
			return nil, grpcErr(codes.InvalidArgument, unmatchedUsernameAndPasswordError, nil)
		}
		workspaceGeneralSetting, err := s.Store.GetWorkspaceGeneralSetting(ctx)
		if err != nil {
			return nil, grpcErr(codes.Internal, "failed to get workspace general setting", err)
		}
		// Check if the password auth in is allowed.
		if workspaceGeneralSetting.DisallowPasswordAuth && user.Role == store.RoleUser {
			return nil, grpcErr(codes.PermissionDenied, "password signin is not allowed", nil)
		}
		existingUser = user
	} else if ssoCredentials := request.GetSsoCredentials(); ssoCredentials != nil {
		identityProvider, err := s.Store.GetIdentityProvider(ctx, &store.FindIdentityProvider{
			ID: &ssoCredentials.IdpId,
		})
		if err != nil {
			return nil, grpcErr(codes.Internal, "failed to get identity provider", err)
		}
		if identityProvider == nil {
			return nil, grpcErr(codes.InvalidArgument, "identity provider not found", nil)
		}

		var userInfo *idp.IdentityProviderUserInfo
		if identityProvider.Type == storepb.IdentityProvider_OAUTH2 {
			oauth2IdentityProvider, err := oauth2.NewIdentityProvider(identityProvider.Config.GetOauth2Config())
			if err != nil {
				return nil, grpcErr(codes.Internal, "failed to create oauth2 identity provider", err)
			}
			token, err := oauth2IdentityProvider.ExchangeToken(ctx, ssoCredentials.RedirectUri, ssoCredentials.Code)
			if err != nil {
				return nil, grpcErr(codes.Internal, "failed to exchange token", err)
			}
			userInfo, err = oauth2IdentityProvider.UserInfo(token)
			if err != nil {
				return nil, grpcErr(codes.Internal, "failed to get user info", err)
			}
		}

		identifierFilter := identityProvider.IdentifierFilter
		if identifierFilter != "" {
			identifierFilterRegex, err := regexp.Compile(identifierFilter)
			if err != nil {
				return nil, grpcErr(codes.Internal, "failed to compile identifier filter regex", err)
			}
			if !identifierFilterRegex.MatchString(userInfo.Identifier) {
				return nil, grpcErr(codes.PermissionDenied, fmt.Sprintf("identifier %s is not allowed", userInfo.Identifier), nil)
			}
		}

		user, err := s.Store.GetUser(ctx, &store.FindUser{
			Username: &userInfo.Identifier,
		})
		if err != nil {
			return nil, grpcErr(codes.Internal, "failed to get user", err)
		}
		if user == nil {
			// Check if the user is allowed to sign up.
			workspaceGeneralSetting, err := s.Store.GetWorkspaceGeneralSetting(ctx)
			if err != nil {
				return nil, grpcErr(codes.Internal, "failed to get workspace general setting", err)
			}
			if workspaceGeneralSetting.DisallowUserRegistration {
				return nil, grpcErr(codes.PermissionDenied, "user registration is not allowed", nil)
			}

			// Create a new user with the user info from the identity provider.
			userCreate := &store.User{
				Username: userInfo.Identifier,
				// The new signup user should be normal user by default.
				Role:      store.RoleUser,
				Nickname:  userInfo.DisplayName,
				Email:     userInfo.Email,
				AvatarURL: userInfo.AvatarURL,
			}
			password, err := util.RandomString(20)
			if err != nil {
				return nil, grpcErr(codes.Internal, "failed to generate random password", err)
			}
			passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				return nil, grpcErr(codes.Internal, "failed to generate password hash", err)
			}
			userCreate.PasswordHash = string(passwordHash)
			user, err = s.Store.CreateUser(ctx, userCreate)
			if err != nil {
				return nil, grpcErr(codes.Internal, "failed to create user", err)
			}
		}
		existingUser = user
	}

	if existingUser == nil {
		return nil, grpcErr(codes.InvalidArgument, "invalid credentials", nil)
	}
	if existingUser.RowStatus == store.Archived {
		return nil, grpcErr(codes.PermissionDenied, fmt.Sprintf("user has been archived with username %s", existingUser.Username), nil)
	}

	expireTime := time.Now().Add(AccessTokenDuration)
	if request.NeverExpire {
		// Set the expire time to 100 years.
		expireTime = time.Now().Add(100 * 365 * 24 * time.Hour)
	}
	if err := s.doSignIn(ctx, existingUser, expireTime); err != nil {
		return nil, grpcErr(codes.Internal, "failed to sign in", err)
	}
	return convertUserFromStore(existingUser), nil
}

func (s *APIV1Service) doSignIn(ctx context.Context, user *store.User, expireTime time.Time) error {
	accessToken, err := GenerateAccessToken(user.Email, user.ID, expireTime, []byte(s.Secret))
	if err != nil {
		return grpcErr(codes.Internal, "failed to generate access token", err)
	}
	if err := s.UpsertAccessTokenToStore(ctx, user, accessToken, "user login"); err != nil {
		return grpcErr(codes.Internal, "failed to upsert access token to store", err)
	}

	cookie, err := s.buildAccessTokenCookie(ctx, accessToken, expireTime)
	if err != nil {
		return grpcErr(codes.Internal, "failed to build access token cookie", err)
	}
	if err := grpc.SetHeader(ctx, metadata.New(map[string]string{
		"Set-Cookie": cookie,
	})); err != nil {
		return grpcErr(codes.Internal, "failed to set grpc header", err)
	}

	return nil
}

func (s *APIV1Service) SignUp(ctx context.Context, request *v1pb.SignUpRequest) (*v1pb.User, error) {
	if request == nil {
		return nil, invalidArgErrf("request is required")
	}
	if strings.TrimSpace(request.Username) == "" {
		return nil, invalidArgErrf("username is required")
	}
	if request.Password == "" {
		return nil, invalidArgErrf("password is required")
	}
	workspaceGeneralSetting, err := s.Store.GetWorkspaceGeneralSetting(ctx)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get workspace general setting", err)
	}
	if workspaceGeneralSetting.DisallowUserRegistration {
		return nil, grpcErr(codes.PermissionDenied, "sign up is not allowed", nil)
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to generate password hash", err)
	}

	create := &store.User{
		Username:     request.Username,
		Nickname:     request.Username,
		PasswordHash: string(passwordHash),
	}
	if err := validateUsername(strings.ToLower(create.Username)); err != nil {
		return nil, err
	}

	hostUserType := store.RoleHost
	existedHostUsers, err := s.Store.ListUsers(ctx, &store.FindUser{
		Role: &hostUserType,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to list host users", err)
	}
	if len(existedHostUsers) == 0 {
		// Change the default role to host if there is no host user.
		create.Role = store.RoleHost
	} else {
		create.Role = store.RoleUser
	}

	user, err := s.Store.CreateUser(ctx, create)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to create user", err)
	}

	if err := s.doSignIn(ctx, user, time.Now().Add(AccessTokenDuration)); err != nil {
		return nil, grpcErr(codes.Internal, "failed to sign in", err)
	}
	return convertUserFromStore(user), nil
}

func (s *APIV1Service) SignOut(ctx context.Context, _ *v1pb.SignOutRequest) (*emptypb.Empty, error) {
	accessToken, ok := ctx.Value(accessTokenContextKey).(string)
	// Try to delete the access token from the store.
	if ok {
		user, _ := s.GetCurrentUser(ctx)
		if user != nil {
			if _, err := s.DeleteUserAccessToken(ctx, &v1pb.DeleteUserAccessTokenRequest{
				Name:        fmt.Sprintf("%s%d", UserNamePrefix, user.ID),
				AccessToken: accessToken,
			}); err != nil {
				slog.Error("failed to delete access token", "error", err)
			}
		}
	}

	if err := s.clearAccessTokenCookie(ctx); err != nil {
		return nil, grpcErr(codes.Internal, "failed to set grpc header", err)
	}
	return &emptypb.Empty{}, nil
}

func (s *APIV1Service) clearAccessTokenCookie(ctx context.Context) error {
	cookie, err := s.buildAccessTokenCookie(ctx, "", time.Time{})
	if err != nil {
		return fmt.Errorf("failed to build access token cookie: %w", err)
	}
	if err := grpc.SetHeader(ctx, metadata.New(map[string]string{
		"Set-Cookie": cookie,
	})); err != nil {
		return fmt.Errorf("failed to set grpc header: %w", err)
	}
	return nil
}

func (*APIV1Service) buildAccessTokenCookie(ctx context.Context, accessToken string, expireTime time.Time) (string, error) {
	attrs := []string{
		fmt.Sprintf("%s=%s", AccessTokenCookieName, accessToken),
		"Path=/",
		"HttpOnly",
	}
	if expireTime.IsZero() {
		attrs = append(attrs, "Expires=Thu, 01 Jan 1970 00:00:00 GMT")
	} else {
		attrs = append(attrs, "Expires="+expireTime.Format(time.RFC1123))
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", errors.New("failed to get metadata from context")
	}
	var origin string
	for _, v := range md.Get("origin") {
		origin = v
	}
	isHTTPS := strings.HasPrefix(origin, "https://")
	if isHTTPS {
		attrs = append(attrs, "SameSite=None")
		attrs = append(attrs, "Secure")
	} else {
		attrs = append(attrs, "SameSite=Strict")
	}
	return strings.Join(attrs, "; "), nil
}

func (s *APIV1Service) GetCurrentUser(ctx context.Context) (*store.User, error) {
	username, ok := ctx.Value(usernameContextKey).(string)
	if !ok {
		return nil, nil
	}
	user, err := s.Store.GetUser(ctx, &store.FindUser{
		Username: &username,
	})
	if err != nil {
		return nil, err
	}
	return user, nil
}
