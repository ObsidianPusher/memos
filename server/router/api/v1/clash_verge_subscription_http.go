package v1

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"gopkg.in/yaml.v3"

	"github.com/usememos/memos/store"
)

const (
	maxSubscriptionBytes           = 8 * 1024 * 1024
	clashSubscriptionCacheDir      = "clash_verge_subscription_cache"
	clashSubscriptionSourceURL     = "url"
	clashSubscriptionSourceContent = "content"
)

var (
	privateCIDRs = mustParseCIDRs(
		"127.0.0.0/8",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	)
)

func (s *APIV1Service) handleFilteredClashVergeSubscription(c echo.Context) error {
	ctx := c.Request().Context()
	currentUser, err := s.getCurrentUserFromSubscriptionRequest(ctx, c.Request())
	if err != nil || currentUser == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
		})
	}

	source := strings.TrimSpace(c.QueryParam("source"))
	if source == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "missing required query parameter: source",
		})
	}
	keywords := parseIncludeKeywords(c.QueryParam("include"))
	if len(keywords) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "missing required query parameter: include",
		})
	}

	sourceURL, err := validateSubscriptionSourceURL(ctx, source)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	payload, err := fetchSubscriptionContent(ctx, sourceURL.String())
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{
			"error": err.Error(),
		})
	}

	filteredContent, matchedCount, err := filterSubscriptionContent(payload, keywords)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}
	if matchedCount == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "no node matched the include keywords in auto-select groups",
		})
	}

	c.Response().Header().Set(echo.HeaderContentType, "text/yaml; charset=utf-8")
	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	c.Response().Header().Set(echo.HeaderContentDisposition, `inline; filename="clash-verge-filtered.yaml"`)
	return c.Blob(http.StatusOK, "text/yaml; charset=utf-8", filteredContent)
}

type upsertClashVergeSubscriptionRequest struct {
	Name                  string `json:"name"`
	SourceType            string `json:"source_type"`
	Source                string `json:"source"`
	Content               string `json:"content"`
	Include               string `json:"include"`
	AutoUpdate            *bool  `json:"auto_update"`
	UpdateIntervalSeconds *int64 `json:"update_interval_seconds"`
}

type clashVergeSubscriptionResponse struct {
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	SourceType            string `json:"source_type"`
	Source                string `json:"source"`
	Content               string `json:"content,omitempty"`
	Include               string `json:"include"`
	AutoUpdate            bool   `json:"auto_update"`
	UpdateIntervalSeconds int64  `json:"update_interval_seconds"`
	LastRefreshTs         int64  `json:"last_refresh_ts"`
	LastRefreshError      string `json:"last_refresh_error,omitempty"`
	PublicURL             string `json:"public_url"`
	CreatedTs             int64  `json:"created_ts"`
	UpdatedTs             int64  `json:"updated_ts"`
}

func (s *APIV1Service) listClashVergeSubscriptions(c echo.Context) error {
	ctx := c.Request().Context()
	currentUser, err := s.getCurrentUserFromRequest(ctx, c.Request())
	if err != nil || currentUser == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
		})
	}

	list, err := s.clashSubscriptionStore.ListByUser(currentUser.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to list subscriptions",
		})
	}
	response := make([]*clashVergeSubscriptionResponse, 0, len(list))
	for _, item := range list {
		response = append(response, convertClashVergeSubscriptionToResponse(c, item))
	}
	return c.JSON(http.StatusOK, map[string]any{
		"subscriptions": response,
	})
}

func (s *APIV1Service) createClashVergeSubscription(c echo.Context) error {
	ctx := c.Request().Context()
	currentUser, err := s.getCurrentUserFromRequest(ctx, c.Request())
	if err != nil || currentUser == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
		})
	}

	request := &upsertClashVergeSubscriptionRequest{}
	if err := decodeStrictJSONBody(c.Request().Body, request); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
	}

	item, err := s.validateAndNormalizeClashVergeSubscriptionInput(ctx, request)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	created, err := s.clashSubscriptionStore.Create(currentUser.ID, item.Name, item.SourceType, item.Source, item.Content, item.Include, *item.AutoUpdate, *item.UpdateIntervalSeconds)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create subscription",
		})
	}
	if _, err := s.refreshClashSubscriptionCache(ctx, created); err != nil {
		_, _ = s.clashSubscriptionStore.UpdateRefreshState(created.ID, 0, err.Error())
		_, _ = s.clashSubscriptionStore.Delete(created.ID, currentUser.ID)
		_ = s.removeClashSubscriptionCache(created.ID)
		return c.JSON(http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("failed to fetch source subscription: %v", err),
		})
	}
	created, _ = s.clashSubscriptionStore.Get(created.ID)
	return c.JSON(http.StatusOK, convertClashVergeSubscriptionToResponse(c, created))
}

func (s *APIV1Service) updateClashVergeSubscription(c echo.Context) error {
	ctx := c.Request().Context()
	currentUser, err := s.getCurrentUserFromRequest(ctx, c.Request())
	if err != nil || currentUser == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
		})
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "missing subscription id",
		})
	}

	request := &upsertClashVergeSubscriptionRequest{}
	if err := decodeStrictJSONBody(c.Request().Body, request); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
	}
	item, err := s.validateAndNormalizeClashVergeSubscriptionInput(ctx, request)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	updated, err := s.clashSubscriptionStore.Update(id, currentUser.ID, item.Name, item.SourceType, item.Source, item.Content, item.Include, *item.AutoUpdate, *item.UpdateIntervalSeconds)
	if err != nil {
		if stderrors.Is(err, errClashSubscriptionPermissionDenied) {
			return c.JSON(http.StatusForbidden, map[string]string{
				"error": "permission denied",
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to update subscription",
		})
	}
	if updated == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "subscription not found",
		})
	}
	if _, err := s.refreshClashSubscriptionCache(ctx, updated); err != nil {
		_, _ = s.clashSubscriptionStore.UpdateRefreshState(updated.ID, 0, err.Error())
		return c.JSON(http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("failed to refresh subscription cache: %v", err),
		})
	}
	updated, _ = s.clashSubscriptionStore.Get(updated.ID)
	return c.JSON(http.StatusOK, convertClashVergeSubscriptionToResponse(c, updated))
}

func (s *APIV1Service) deleteClashVergeSubscription(c echo.Context) error {
	ctx := c.Request().Context()
	currentUser, err := s.getCurrentUserFromRequest(ctx, c.Request())
	if err != nil || currentUser == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
		})
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "missing subscription id",
		})
	}

	deleted, err := s.clashSubscriptionStore.Delete(id, currentUser.ID)
	if err != nil {
		if stderrors.Is(err, errClashSubscriptionPermissionDenied) {
			return c.JSON(http.StatusForbidden, map[string]string{
				"error": "permission denied",
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to delete subscription",
		})
	}
	if !deleted {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "subscription not found",
		})
	}
	_ = s.removeClashSubscriptionCache(id)
	return c.NoContent(http.StatusNoContent)
}

func (s *APIV1Service) getClashVergeSubscriptionConfig(c echo.Context) error {
	ctx := c.Request().Context()
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "missing subscription id",
		})
	}
	item, err := s.clashSubscriptionStore.Get(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to load subscription",
		})
	}
	if item == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "subscription not found",
		})
	}

	cachedContent, _ := s.readClashSubscriptionCache(item.ID)
	shouldRefresh := len(cachedContent) == 0 || s.shouldRefreshSubscription(item)
	if shouldRefresh {
		refreshedContent, err := s.refreshClashSubscriptionCache(ctx, item)
		if err == nil {
			cachedContent = refreshedContent
		} else if len(cachedContent) == 0 {
			_, _ = s.clashSubscriptionStore.UpdateRefreshState(item.ID, 0, err.Error())
			return c.JSON(http.StatusBadGateway, map[string]string{
				"error": err.Error(),
			})
		} else {
			_, _ = s.clashSubscriptionStore.UpdateRefreshState(item.ID, 0, err.Error())
			c.Response().Header().Set("X-Memos-Subscription-Stale", "true")
		}
	}
	if len(cachedContent) == 0 {
		return c.JSON(http.StatusBadGateway, map[string]string{
			"error": "subscription cache is empty",
		})
	}
	c.Response().Header().Set(echo.HeaderContentType, "text/yaml; charset=utf-8")
	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	c.Response().Header().Set(echo.HeaderContentDisposition, `inline; filename="clash-verge-filtered.yaml"`)
	return c.Blob(http.StatusOK, "text/yaml; charset=utf-8", cachedContent)
}

func (s *APIV1Service) validateAndNormalizeClashVergeSubscriptionInput(ctx context.Context, request *upsertClashVergeSubscriptionRequest) (*upsertClashVergeSubscriptionRequest, error) {
	if request == nil {
		return nil, fmt.Errorf("empty request")
	}
	sourceType := strings.TrimSpace(request.SourceType)
	if sourceType == "" {
		sourceType = clashSubscriptionSourceURL
	}
	if sourceType != clashSubscriptionSourceURL && sourceType != clashSubscriptionSourceContent {
		return nil, fmt.Errorf("unsupported source type")
	}
	source := strings.TrimSpace(request.Source)
	content := strings.TrimSpace(request.Content)
	if sourceType == clashSubscriptionSourceURL {
		if source == "" {
			return nil, fmt.Errorf("missing required field: source")
		}
		if _, err := validateSubscriptionSourceURL(ctx, source); err != nil {
			return nil, err
		}
		if content != "" {
			return nil, fmt.Errorf("content is only supported for content imports")
		}
	} else {
		if content == "" {
			return nil, fmt.Errorf("missing required field: content")
		}
		if len(content) > maxSubscriptionBytes {
			return nil, fmt.Errorf("subscription content is too large")
		}
		if err := validateClashYAMLContent([]byte(content)); err != nil {
			return nil, err
		}
		source = ""
	}
	include := strings.TrimSpace(request.Include)
	if len(parseIncludeKeywords(include)) == 0 {
		return nil, fmt.Errorf("missing required field: include")
	}
	name := strings.TrimSpace(request.Name)
	if name == "" {
		name = "Clash Verge Subscription"
	}
	if len(name) > 120 {
		return nil, fmt.Errorf("name is too long")
	}
	autoUpdate := true
	if request.AutoUpdate != nil {
		autoUpdate = *request.AutoUpdate
	}
	intervalSeconds := int64(0)
	if request.UpdateIntervalSeconds != nil {
		intervalSeconds = *request.UpdateIntervalSeconds
	}
	if sourceType == clashSubscriptionSourceContent {
		autoUpdate = false
		intervalSeconds = 0
	} else if autoUpdate {
		if intervalSeconds == 0 {
			intervalSeconds = defaultClashSubscriptionUpdateIntervalSeconds
		}
		if intervalSeconds < minClashSubscriptionUpdateIntervalSeconds || intervalSeconds > maxClashSubscriptionUpdateIntervalSeconds {
			return nil, fmt.Errorf("update interval must be between %d and %d seconds", minClashSubscriptionUpdateIntervalSeconds, maxClashSubscriptionUpdateIntervalSeconds)
		}
	} else {
		intervalSeconds = 0
	}
	return &upsertClashVergeSubscriptionRequest{
		Name:                  name,
		SourceType:            sourceType,
		Source:                source,
		Content:               content,
		Include:               include,
		AutoUpdate:            &autoUpdate,
		UpdateIntervalSeconds: &intervalSeconds,
	}, nil
}

func validateClashYAMLContent(payload []byte) error {
	root := map[string]any{}
	if err := yaml.Unmarshal(payload, &root); err != nil {
		return fmt.Errorf("content import only supports Clash YAML")
	}
	if _, ok := root["proxy-groups"].([]any); !ok {
		return fmt.Errorf("content import only supports Clash YAML with proxy-groups")
	}
	if _, ok := root["proxies"].([]any); !ok {
		return fmt.Errorf("content import only supports Clash YAML with proxies")
	}
	return nil
}

func convertClashVergeSubscriptionToResponse(c echo.Context, item *ClashVergeSubscription) *clashVergeSubscriptionResponse {
	return &clashVergeSubscriptionResponse{
		ID:                    item.ID,
		Name:                  item.Name,
		SourceType:            item.SourceType,
		Source:                item.Source,
		Content:               item.Content,
		Include:               item.Include,
		AutoUpdate:            item.AutoUpdate,
		UpdateIntervalSeconds: item.UpdateIntervalSeconds,
		LastRefreshTs:         item.LastRefreshTs,
		LastRefreshError:      item.LastRefreshError,
		PublicURL:             buildClashVergeSubscriptionPublicURL(c, item.ID),
		CreatedTs:             item.CreatedTs,
		UpdatedTs:             item.UpdatedTs,
	}
}

func buildClashVergeSubscriptionPublicURL(c echo.Context, id string) string {
	request := c.Request()
	scheme := request.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = request.URL.Scheme
	}
	if scheme == "" {
		if request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := request.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = request.Host
	}
	return fmt.Sprintf("%s://%s/api/v1/clash-verge/subscriptions/%s/config", scheme, host, id)
}

func (s *APIV1Service) shouldRefreshSubscription(item *ClashVergeSubscription) bool {
	if item == nil {
		return false
	}
	if !item.AutoUpdate {
		return false
	}
	if item.LastRefreshTs == 0 {
		return true
	}
	if item.UpdateIntervalSeconds <= 0 {
		return false
	}
	return time.Now().Unix()-item.LastRefreshTs >= item.UpdateIntervalSeconds
}

func (s *APIV1Service) refreshClashSubscriptionCache(ctx context.Context, item *ClashVergeSubscription) ([]byte, error) {
	if item == nil {
		return nil, fmt.Errorf("subscription is nil")
	}
	keywords := parseIncludeKeywords(item.Include)
	if len(keywords) == 0 {
		return nil, fmt.Errorf("subscription include keywords are empty")
	}
	var payload []byte
	if item.SourceType == clashSubscriptionSourceContent {
		payload = []byte(item.Content)
	} else {
		sourceURL, err := validateSubscriptionSourceURL(ctx, item.Source)
		if err != nil {
			return nil, err
		}
		payload, err = fetchSubscriptionContent(ctx, sourceURL.String())
		if err != nil {
			return nil, err
		}
	}
	filteredContent, _, err := filterSubscriptionContent(payload, keywords)
	if err != nil {
		return nil, err
	}
	if err := s.writeClashSubscriptionCache(item.ID, filteredContent); err != nil {
		return nil, err
	}
	_, _ = s.clashSubscriptionStore.UpdateRefreshState(item.ID, time.Now().Unix(), "")
	return filteredContent, nil
}

func (s *APIV1Service) getClashSubscriptionCachePath(id string) string {
	return filepath.Join(s.Profile.Data, clashSubscriptionCacheDir, fmt.Sprintf("%s.yaml", id))
}

func (s *APIV1Service) readClashSubscriptionCache(id string) ([]byte, error) {
	path := s.getClashSubscriptionCachePath(id)
	content, err := os.ReadFile(path)
	if err != nil {
		if stderrors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return content, nil
}

func (s *APIV1Service) writeClashSubscriptionCache(id string, content []byte) error {
	cachePath := s.getClashSubscriptionCachePath(id)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return err
	}
	tmpPath := cachePath + ".tmp"
	if err := os.WriteFile(tmpPath, content, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, cachePath); err != nil {
		return err
	}
	return nil
}

func (s *APIV1Service) removeClashSubscriptionCache(id string) error {
	cachePath := s.getClashSubscriptionCachePath(id)
	err := os.Remove(cachePath)
	if err != nil && !stderrors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *APIV1Service) getCurrentUserFromSubscriptionRequest(ctx context.Context, request *http.Request) (*store.User, error) {
	token := strings.TrimSpace(request.URL.Query().Get("access_token"))
	if token == "" {
		token = strings.TrimSpace(request.URL.Query().Get("token"))
	}
	if token == "" {
		return s.getCurrentUserFromRequest(ctx, request)
	}
	cloned := request.Clone(ctx)
	cloned.Header = request.Header.Clone()
	cloned.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	return s.getCurrentUserFromRequest(ctx, cloned)
}

func parseIncludeKeywords(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == ';'
	})
	keywordMap := make(map[string]struct{})
	keywords := make([]string, 0, len(parts))
	for _, part := range parts {
		keyword := strings.TrimSpace(part)
		if keyword == "" {
			continue
		}
		lowerKeyword := strings.ToLower(keyword)
		if _, exists := keywordMap[lowerKeyword]; exists {
			continue
		}
		keywordMap[lowerKeyword] = struct{}{}
		keywords = append(keywords, keyword)
	}
	return keywords
}

func decodeStrictJSONBody(body io.Reader, target any) error {
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected extra JSON content")
		}
		return err
	}
	return nil
}

func validateSubscriptionSourceURL(ctx context.Context, rawURL string) (*url.URL, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("invalid source URL: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported source URL scheme: %s", parsedURL.Scheme)
	}
	host := parsedURL.Hostname()
	if host == "" {
		return nil, fmt.Errorf("invalid source URL host")
	}
	if isLocalHostname(host) {
		return nil, fmt.Errorf("source URL host is not allowed")
	}

	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return nil, fmt.Errorf("source URL host is not allowed")
		}
		return parsedURL, nil
	}

	resolveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ipAddrs, err := net.DefaultResolver.LookupIPAddr(resolveCtx, host)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve source URL host: %w", err)
	}
	for _, ipAddr := range ipAddrs {
		if isPrivateIP(ipAddr.IP) {
			return nil, fmt.Errorf("source URL host is not allowed")
		}
	}
	return parsedURL, nil
}

func fetchSubscriptionContent(ctx context.Context, sourceURL string) ([]byte, error) {
	requestCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}
	request.Header.Set("User-Agent", "memos-clash-verge-filter/1.0")
	client := &http.Client{
		Timeout: 20 * time.Second,
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch source subscription: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("source subscription returned status %d", response.StatusCode)
	}

	limitedReader := io.LimitReader(response.Body, maxSubscriptionBytes+1)
	payload, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read source subscription: %w", err)
	}
	if len(payload) > maxSubscriptionBytes {
		return nil, fmt.Errorf("source subscription is too large")
	}
	return payload, nil
}

func filterAutoSelectNodes(payload []byte, includeKeywords []string) ([]byte, int, error) {
	root := map[string]any{}
	if err := yaml.Unmarshal(payload, &root); err != nil {
		return nil, 0, fmt.Errorf("failed to parse subscription YAML: %w", err)
	}

	proxyNames := getProxyNameSet(root["proxies"])
	proxyGroups, ok := root["proxy-groups"].([]any)
	if !ok {
		return nil, 0, fmt.Errorf("invalid subscription: missing proxy-groups")
	}

	autoGroupCount := 0
	totalMatched := 0
	for _, proxyGroupRaw := range proxyGroups {
		proxyGroup, ok := proxyGroupRaw.(map[string]any)
		if !ok || !isAutoSelectGroup(proxyGroup) {
			continue
		}
		autoGroupCount++

		filteredProxies, matched := filterProxyNameList(proxyGroup["proxies"], proxyNames, includeKeywords)
		if filteredProxies == nil {
			continue
		}
		proxyGroup["proxies"] = filteredProxies
		totalMatched += matched
	}
	if autoGroupCount == 0 {
		return nil, 0, fmt.Errorf("no auto-select proxy group was found")
	}
	if totalMatched == 0 {
		return nil, 0, fmt.Errorf("no node matched include keywords")
	}

	output, err := yaml.Marshal(root)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal filtered subscription: %w", err)
	}
	return output, totalMatched, nil
}

func filterSubscriptionContent(payload []byte, includeKeywords []string) ([]byte, int, error) {
	return filterAutoSelectNodes(payload, includeKeywords)
}

func filterProxyNameList(raw any, proxyNameSet map[string]struct{}, includeKeywords []string) ([]string, int) {
	proxyList, ok := raw.([]any)
	if !ok {
		return nil, 0
	}

	filtered := make([]string, 0, len(proxyList))
	filteredSet := make(map[string]struct{})
	matched := 0
	for _, proxyItem := range proxyList {
		proxyName, ok := proxyItem.(string)
		if !ok {
			continue
		}
		if _, exists := proxyNameSet[proxyName]; !exists {
			continue
		}
		if !matchesIncludeKeywords(proxyName, includeKeywords) {
			continue
		}
		if _, exists := filteredSet[proxyName]; exists {
			continue
		}
		matched++
		filteredSet[proxyName] = struct{}{}
		filtered = append(filtered, proxyName)
	}
	return filtered, matched
}

func getProxyNameSet(raw any) map[string]struct{} {
	nameSet := make(map[string]struct{})
	proxies, ok := raw.([]any)
	if !ok {
		return nameSet
	}
	for _, proxyRaw := range proxies {
		proxy, ok := proxyRaw.(map[string]any)
		if !ok {
			continue
		}
		name, ok := proxy["name"].(string)
		if !ok || strings.TrimSpace(name) == "" {
			continue
		}
		nameSet[name] = struct{}{}
	}
	return nameSet
}

func isAutoSelectGroup(proxyGroup map[string]any) bool {
	groupType, _ := proxyGroup["type"].(string)
	switch strings.ToLower(strings.TrimSpace(groupType)) {
	case "url-test", "fallback", "load-balance":
		return true
	}

	groupName, _ := proxyGroup["name"].(string)
	normalizedName := strings.ToLower(strings.TrimSpace(groupName))
	return strings.Contains(normalizedName, "自动选择") ||
		strings.Contains(normalizedName, "自动测速") ||
		strings.Contains(normalizedName, "auto")
}

func matchesIncludeKeywords(proxyName string, includeKeywords []string) bool {
	normalizedName := strings.ToLower(proxyName)
	for _, keyword := range includeKeywords {
		if strings.Contains(normalizedName, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func mustParseCIDRs(cidrs ...string) []*net.IPNet {
	parsed := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			slog.Error("failed to parse CIDR", "cidr", cidr, "error", err)
			continue
		}
		parsed = append(parsed, ipnet)
	}
	return parsed
}

func isPrivateIP(ip net.IP) bool {
	for _, cidr := range privateCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

func isLocalHostname(hostname string) bool {
	normalized := strings.ToLower(strings.TrimSpace(hostname))
	return normalized == "localhost" || strings.HasSuffix(normalized, ".local")
}
