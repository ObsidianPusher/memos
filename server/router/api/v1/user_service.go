package v1

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"google.golang.org/genproto/googleapis/api/httpbody"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/usememos/memos/internal/base"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	storepb "github.com/usememos/memos/proto/gen/store"
	"github.com/usememos/memos/store"
)

func (s *APIV1Service) ListUsers(ctx context.Context, _ *v1pb.ListUsersRequest) (*v1pb.ListUsersResponse, error) {
	currentUser, err := s.getRequiredCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	if currentUser.Role != store.RoleHost && currentUser.Role != store.RoleAdmin {
		return nil, grpcErr(codes.PermissionDenied, "permission denied", nil)
	}

	users, err := s.Store.ListUsers(ctx, &store.FindUser{})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to list users", err)
	}

	response := &v1pb.ListUsersResponse{
		Users: []*v1pb.User{},
	}
	for _, user := range users {
		response.Users = append(response.Users, convertUserFromStore(user))
	}
	return response, nil
}

func (s *APIV1Service) GetUser(ctx context.Context, request *v1pb.GetUserRequest) (*v1pb.User, error) {
	userID, err := extractUserID(request.Name)
	if err != nil {
		return nil, err
	}
	user, err := s.Store.GetUser(ctx, &store.FindUser{
		ID: &userID,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get user", err)
	}
	if user == nil {
		return nil, grpcErr(codes.NotFound, "user not found", nil)
	}

	return convertUserFromStore(user), nil
}

func (s *APIV1Service) GetUserByUsername(ctx context.Context, request *v1pb.GetUserByUsernameRequest) (*v1pb.User, error) {
	user, err := s.Store.GetUser(ctx, &store.FindUser{
		Username: &request.Username,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get user", err)
	}
	if user == nil {
		return nil, grpcErr(codes.NotFound, "user not found", nil)
	}

	return convertUserFromStore(user), nil
}

func (s *APIV1Service) GetUserAvatarBinary(ctx context.Context, request *v1pb.GetUserAvatarBinaryRequest) (*httpbody.HttpBody, error) {
	userID, err := extractUserID(request.Name)
	if err != nil {
		return nil, err
	}
	user, err := s.Store.GetUser(ctx, &store.FindUser{
		ID: &userID,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get user", err)
	}
	if user == nil {
		return nil, grpcErr(codes.NotFound, "user not found", nil)
	}
	if user.AvatarURL == "" {
		return nil, grpcErr(codes.NotFound, "avatar not found", nil)
	}

	imageType, base64Data, err := extractImageInfo(user.AvatarURL)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to extract image info", err)
	}
	imageData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to decode string", err)
	}
	httpBody := &httpbody.HttpBody{
		ContentType: imageType,
		Data:        imageData,
	}
	return httpBody, nil
}

func (s *APIV1Service) CreateUser(ctx context.Context, request *v1pb.CreateUserRequest) (*v1pb.User, error) {
	if request == nil || request.User == nil {
		return nil, invalidArgErrf("user is required")
	}
	currentUser, err := s.getRequiredCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	if currentUser.Role != store.RoleHost {
		return nil, grpcErr(codes.PermissionDenied, "permission denied", nil)
	}
	if !base.UIDMatcher.MatchString(strings.ToLower(request.User.Username)) {
		return nil, invalidArgErrf("invalid username: %s", request.User.Username)
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(request.User.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to generate password hash", err)
	}

	user, err := s.Store.CreateUser(ctx, &store.User{
		Username:     request.User.Username,
		Role:         convertUserRoleToStore(request.User.Role),
		Email:        request.User.Email,
		Nickname:     request.User.Nickname,
		PasswordHash: string(passwordHash),
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to create user", err)
	}

	return convertUserFromStore(user), nil
}

func (s *APIV1Service) UpdateUser(ctx context.Context, request *v1pb.UpdateUserRequest) (*v1pb.User, error) {
	if request == nil || request.User == nil {
		return nil, invalidArgErrf("user is required")
	}
	if request.UpdateMask == nil || len(request.UpdateMask.Paths) == 0 {
		return nil, invalidArgErrf("update mask is empty")
	}
	userID, err := ExtractUserIDFromName(request.User.Name)
	if err != nil {
		return nil, invalidArgErrf("invalid user name: %v", err)
	}
	currentUser, err := s.getRequiredCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	// Check permission.
	// Only allow admin or self to update user.
	if currentUser.ID != userID && currentUser.Role != store.RoleAdmin && currentUser.Role != store.RoleHost {
		return nil, grpcErr(codes.PermissionDenied, "permission denied", nil)
	}

	user, err := s.Store.GetUser(ctx, &store.FindUser{ID: &userID})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get user", err)
	}
	if user == nil {
		return nil, grpcErr(codes.NotFound, "user not found", nil)
	}

	currentTs := time.Now().Unix()
	update := &store.UpdateUser{
		ID:        user.ID,
		UpdatedTs: &currentTs,
	}
	workspaceGeneralSetting, err := s.Store.GetWorkspaceGeneralSetting(ctx)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get workspace general setting", err)
	}
	for _, field := range request.UpdateMask.Paths {
		if field == "username" {
			if workspaceGeneralSetting.DisallowChangeUsername {
				return nil, grpcErr(codes.PermissionDenied, "permission denied: disallow change username", nil)
			}
			if !base.UIDMatcher.MatchString(strings.ToLower(request.User.Username)) {
				return nil, invalidArgErrf("invalid username: %s", request.User.Username)
			}
			update.Username = &request.User.Username
		} else if field == "nickname" {
			if workspaceGeneralSetting.DisallowChangeNickname {
				return nil, grpcErr(codes.PermissionDenied, "permission denied: disallow change nickname", nil)
			}
			update.Nickname = &request.User.Nickname
		} else if field == "email" {
			update.Email = &request.User.Email
		} else if field == "avatar_url" {
			update.AvatarURL = &request.User.AvatarUrl
		} else if field == "description" {
			update.Description = &request.User.Description
		} else if field == "role" {
			// Only allow admin to update role.
			if currentUser.Role != store.RoleAdmin && currentUser.Role != store.RoleHost {
				return nil, grpcErr(codes.PermissionDenied, "permission denied", nil)
			}
			role := convertUserRoleToStore(request.User.Role)
			update.Role = &role
		} else if field == "password" {
			passwordHash, err := bcrypt.GenerateFromPassword([]byte(request.User.Password), bcrypt.DefaultCost)
			if err != nil {
				return nil, grpcErr(codes.Internal, "failed to generate password hash", err)
			}
			passwordHashStr := string(passwordHash)
			update.PasswordHash = &passwordHashStr
		} else if field == "state" {
			rowStatus := convertStateToStore(request.User.State)
			update.RowStatus = &rowStatus
		} else {
			return nil, invalidArgErrf("invalid update path: %s", field)
		}
	}

	updatedUser, err := s.Store.UpdateUser(ctx, update)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to update user", err)
	}

	return convertUserFromStore(updatedUser), nil
}

func (s *APIV1Service) DeleteUser(ctx context.Context, request *v1pb.DeleteUserRequest) (*emptypb.Empty, error) {
	userID, err := extractUserID(request.Name)
	if err != nil {
		return nil, err
	}
	currentUser, err := s.getRequiredCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	if currentUser.ID != userID && currentUser.Role != store.RoleAdmin && currentUser.Role != store.RoleHost {
		return nil, grpcErr(codes.PermissionDenied, "permission denied", nil)
	}

	user, err := s.Store.GetUser(ctx, &store.FindUser{ID: &userID})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to get user", err)
	}
	if user == nil {
		return nil, grpcErr(codes.NotFound, "user not found", nil)
	}

	if err := s.Store.DeleteUser(ctx, &store.DeleteUser{
		ID: user.ID,
	}); err != nil {
		return nil, grpcErr(codes.Internal, "failed to delete user", err)
	}

	return &emptypb.Empty{}, nil
}

func getDefaultUserSetting() *v1pb.UserSetting {
	return &v1pb.UserSetting{
		Locale:         "en",
		Appearance:     "system",
		MemoVisibility: "PRIVATE",
	}
}

func (s *APIV1Service) GetUserSetting(ctx context.Context, _ *v1pb.GetUserSettingRequest) (*v1pb.UserSetting, error) {
	user, err := s.getRequiredCurrentUser(ctx)
	if err != nil {
		return nil, err
	}

	userSettings, err := s.Store.ListUserSettings(ctx, &store.FindUserSetting{
		UserID: &user.ID,
	})
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to list user settings", err)
	}
	userSettingMessage := getDefaultUserSetting()
	for _, setting := range userSettings {
		if setting.Key == storepb.UserSettingKey_LOCALE {
			userSettingMessage.Locale = setting.GetLocale()
		} else if setting.Key == storepb.UserSettingKey_APPEARANCE {
			userSettingMessage.Appearance = setting.GetAppearance()
		} else if setting.Key == storepb.UserSettingKey_MEMO_VISIBILITY {
			userSettingMessage.MemoVisibility = setting.GetMemoVisibility()
		}
	}
	return userSettingMessage, nil
}

func (s *APIV1Service) UpdateUserSetting(ctx context.Context, request *v1pb.UpdateUserSettingRequest) (*v1pb.UserSetting, error) {
	if request == nil || request.Setting == nil {
		return nil, invalidArgErrf("setting is required")
	}
	user, err := s.getRequiredCurrentUser(ctx)
	if err != nil {
		return nil, err
	}

	if request.UpdateMask == nil || len(request.UpdateMask.Paths) == 0 {
		return nil, invalidArgErrf("update mask is empty")
	}

	for _, field := range request.UpdateMask.Paths {
		if field == "locale" {
			if _, err := s.Store.UpsertUserSetting(ctx, &storepb.UserSetting{
				UserId: user.ID,
				Key:    storepb.UserSettingKey_LOCALE,
				Value: &storepb.UserSetting_Locale{
					Locale: request.Setting.Locale,
				},
			}); err != nil {
				return nil, grpcErr(codes.Internal, "failed to upsert user setting", err)
			}
		} else if field == "appearance" {
			if _, err := s.Store.UpsertUserSetting(ctx, &storepb.UserSetting{
				UserId: user.ID,
				Key:    storepb.UserSettingKey_APPEARANCE,
				Value: &storepb.UserSetting_Appearance{
					Appearance: request.Setting.Appearance,
				},
			}); err != nil {
				return nil, grpcErr(codes.Internal, "failed to upsert user setting", err)
			}
		} else if field == "memo_visibility" {
			if _, err := s.Store.UpsertUserSetting(ctx, &storepb.UserSetting{
				UserId: user.ID,
				Key:    storepb.UserSettingKey_MEMO_VISIBILITY,
				Value: &storepb.UserSetting_MemoVisibility{
					MemoVisibility: request.Setting.MemoVisibility,
				},
			}); err != nil {
				return nil, grpcErr(codes.Internal, "failed to upsert user setting", err)
			}
		} else {
			return nil, invalidArgErrf("invalid update path: %s", field)
		}
	}

	return s.GetUserSetting(ctx, &v1pb.GetUserSettingRequest{})
}

func (s *APIV1Service) ListUserAccessTokens(ctx context.Context, request *v1pb.ListUserAccessTokensRequest) (*v1pb.ListUserAccessTokensResponse, error) {
	userID, err := ExtractUserIDFromName(request.Name)
	if err != nil {
		return nil, invalidArgErrf("invalid user name: %v", err)
	}

	currentUser, err := s.getRequiredCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	if currentUser.ID != userID {
		return nil, grpcErr(codes.PermissionDenied, "permission denied", nil)
	}

	userAccessTokens, err := s.Store.GetUserAccessTokens(ctx, userID)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to list access tokens", err)
	}

	accessTokens := []*v1pb.UserAccessToken{}
	for _, userAccessToken := range userAccessTokens {
		claims, err := parseAccessTokenClaims(userAccessToken.AccessToken, s.Secret)
		if err != nil {
			// If the access token is invalid or expired, just ignore it.
			continue
		}

		userAccessToken := &v1pb.UserAccessToken{
			AccessToken: userAccessToken.AccessToken,
			Description: userAccessToken.Description,
			IssuedAt:    timestamppb.New(claims.IssuedAt.Time),
		}
		if claims.ExpiresAt != nil {
			userAccessToken.ExpiresAt = timestamppb.New(claims.ExpiresAt.Time)
		}
		accessTokens = append(accessTokens, userAccessToken)
	}

	// Sort by issued time in descending order.
	slices.SortFunc(accessTokens, func(i, j *v1pb.UserAccessToken) int {
		if i.IssuedAt.Seconds == j.IssuedAt.Seconds {
			return 0
		}
		if i.IssuedAt.Seconds > j.IssuedAt.Seconds {
			return -1
		}
		return 1
	})
	response := &v1pb.ListUserAccessTokensResponse{
		AccessTokens: accessTokens,
	}
	return response, nil
}

func (s *APIV1Service) CreateUserAccessToken(ctx context.Context, request *v1pb.CreateUserAccessTokenRequest) (*v1pb.UserAccessToken, error) {
	userID, err := ExtractUserIDFromName(request.Name)
	if err != nil {
		return nil, invalidArgErrf("invalid user name: %v", err)
	}
	currentUser, err := s.getRequiredCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	if currentUser.ID != userID {
		return nil, grpcErr(codes.PermissionDenied, "permission denied", nil)
	}

	expiresAt := time.Time{}
	if request.ExpiresAt != nil {
		expiresAt = request.ExpiresAt.AsTime()
	}

	accessToken, err := GenerateAccessToken(currentUser.Username, currentUser.ID, expiresAt, []byte(s.Secret))
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to generate access token", err)
	}

	claims, err := parseAccessTokenClaims(accessToken, s.Secret)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to parse access token", err)
	}

	// Upsert the access token to user setting store.
	if err := s.UpsertAccessTokenToStore(ctx, currentUser, accessToken, request.Description); err != nil {
		return nil, grpcErr(codes.Internal, "failed to upsert access token to store", err)
	}

	userAccessToken := &v1pb.UserAccessToken{
		AccessToken: accessToken,
		Description: request.Description,
		IssuedAt:    timestamppb.New(claims.IssuedAt.Time),
	}
	if claims.ExpiresAt != nil {
		userAccessToken.ExpiresAt = timestamppb.New(claims.ExpiresAt.Time)
	}
	return userAccessToken, nil
}

func (s *APIV1Service) DeleteUserAccessToken(ctx context.Context, request *v1pb.DeleteUserAccessTokenRequest) (*emptypb.Empty, error) {
	userID, err := ExtractUserIDFromName(request.Name)
	if err != nil {
		return nil, invalidArgErrf("invalid user name: %v", err)
	}
	currentUser, err := s.getRequiredCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	if currentUser.ID != userID {
		return nil, grpcErr(codes.PermissionDenied, "permission denied", nil)
	}

	userAccessTokens, err := s.Store.GetUserAccessTokens(ctx, currentUser.ID)
	if err != nil {
		return nil, grpcErr(codes.Internal, "failed to list access tokens", err)
	}
	updatedUserAccessTokens := []*storepb.AccessTokensUserSetting_AccessToken{}
	for _, userAccessToken := range userAccessTokens {
		if userAccessToken.AccessToken == request.AccessToken {
			continue
		}
		updatedUserAccessTokens = append(updatedUserAccessTokens, userAccessToken)
	}
	if _, err := s.Store.UpsertUserSetting(ctx, &storepb.UserSetting{
		UserId: currentUser.ID,
		Key:    storepb.UserSettingKey_ACCESS_TOKENS,
		Value: &storepb.UserSetting_AccessTokens{
			AccessTokens: &storepb.AccessTokensUserSetting{
				AccessTokens: updatedUserAccessTokens,
			},
		},
	}); err != nil {
		return nil, grpcErr(codes.Internal, "failed to upsert user setting", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *APIV1Service) UpsertAccessTokenToStore(ctx context.Context, user *store.User, accessToken, description string) error {
	userAccessTokens, err := s.Store.GetUserAccessTokens(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("failed to get user access tokens: %w", err)
	}
	userAccessToken := storepb.AccessTokensUserSetting_AccessToken{
		AccessToken: accessToken,
		Description: description,
	}
	userAccessTokens = append(userAccessTokens, &userAccessToken)
	if _, err := s.Store.UpsertUserSetting(ctx, &storepb.UserSetting{
		UserId: user.ID,
		Key:    storepb.UserSettingKey_ACCESS_TOKENS,
		Value: &storepb.UserSetting_AccessTokens{
			AccessTokens: &storepb.AccessTokensUserSetting{
				AccessTokens: userAccessTokens,
			},
		},
	}); err != nil {
		return fmt.Errorf("failed to upsert user setting: %w", err)
	}
	return nil
}

func convertUserFromStore(user *store.User) *v1pb.User {
	userpb := &v1pb.User{
		Name:        fmt.Sprintf("%s%d", UserNamePrefix, user.ID),
		State:       convertStateFromStore(user.RowStatus),
		CreateTime:  timestamppb.New(time.Unix(user.CreatedTs, 0)),
		UpdateTime:  timestamppb.New(time.Unix(user.UpdatedTs, 0)),
		Role:        convertUserRoleFromStore(user.Role),
		Username:    user.Username,
		Email:       user.Email,
		Nickname:    user.Nickname,
		AvatarUrl:   user.AvatarURL,
		Description: user.Description,
	}
	// Use the avatar URL instead of raw base64 image data to reduce the response size.
	if user.AvatarURL != "" {
		// Check if avatar url is base64 format.
		_, _, err := extractImageInfo(user.AvatarURL)
		if err == nil {
			userpb.AvatarUrl = fmt.Sprintf("/file/%s/avatar", userpb.Name)
		} else {
			userpb.AvatarUrl = user.AvatarURL
		}
	}
	return userpb
}

func convertUserRoleFromStore(role store.Role) v1pb.User_Role {
	switch role {
	case store.RoleHost:
		return v1pb.User_HOST
	case store.RoleAdmin:
		return v1pb.User_ADMIN
	case store.RoleUser:
		return v1pb.User_USER
	default:
		return v1pb.User_ROLE_UNSPECIFIED
	}
}

func convertUserRoleToStore(role v1pb.User_Role) store.Role {
	switch role {
	case v1pb.User_HOST:
		return store.RoleHost
	case v1pb.User_ADMIN:
		return store.RoleAdmin
	case v1pb.User_USER:
		return store.RoleUser
	default:
		return store.RoleUser
	}
}

func extractImageInfo(dataURI string) (string, string, error) {
	dataURIRegex := regexp.MustCompile(`^data:(?P<type>.+);base64,(?P<base64>.+)`)
	matches := dataURIRegex.FindStringSubmatch(dataURI)
	if len(matches) != 3 {
		return "", "", errors.New("Invalid data URI format")
	}
	imageType := matches[1]
	base64Data := matches[2]
	return imageType, base64Data, nil
}
