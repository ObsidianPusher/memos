package v1

import (
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/usememos/memos/internal/profile"
	"gopkg.in/yaml.v3"
)

func TestDecodeStrictJSONBodyRejectsUnknownFields(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	var got payload
	err := decodeStrictJSONBody(strings.NewReader(`{"name":"ok","extra":1}`), &got)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFilterSubscriptionContentKeepsClashYAMLRules(t *testing.T) {
	payload := []byte(`
proxies:
  - name: 香港 01
    type: socks5
    server: 127.0.0.1
    port: 1080
  - name: Japan 01
    type: socks5
    server: 127.0.0.2
    port: 1080
proxy-groups:
  - name: 自动选择
    type: url-test
    proxies:
      - 香港 01
      - Japan 01
rules:
  - MATCH,自动选择
`)

	filtered, matched, err := filterSubscriptionContent(payload, []string{"香港"}, "自动选择")
	if err != nil {
		t.Fatalf("filter subscription content: %v", err)
	}
	if matched != 1 {
		t.Fatalf("matched = %d, want 1", matched)
	}
	if got := string(filtered); !strings.Contains(got, "rules:") || !strings.Contains(got, "MATCH,自动选择") {
		t.Fatalf("unexpected filtered content: %q", got)
	}
	if !strings.Contains(string(filtered), "name: Japan 01") {
		t.Fatalf("non-auto node must remain in the config: %q", filtered)
	}
	if !strings.Contains(string(filtered), "- 香港 01") || strings.Contains(string(filtered), "- Japan 01\nrules:") {
		t.Fatalf("unexpected auto proxy group: %q", filtered)
	}
}

func TestClashSubscriptionFilterRejectsEmptyGroupAfterRemovingDirect(t *testing.T) {
	const payload = `proxies: []
proxy-groups:
- name: "选择出口"
  type: "select"
  proxies:
    - DIRECT
rules:
- MATCH,选择出口
`
	if _, err := prepareClashSubscriptionContent([]byte(payload), "选择出口", "HK"); err == nil {
		t.Fatal("expected an error when removing DIRECT leaves the group empty")
	}
}

func TestClashSubscriptionFilterRejectsNoMatchingNodes(t *testing.T) {
	const payload = `proxies:
- name: JP
proxy-groups:
- name: 选择出口
  type: select
  proxies: [JP, DIRECT]
`
	if _, err := prepareClashSubscriptionContent([]byte(payload), "选择出口", "HK"); err == nil {
		t.Fatal("expected unmatched nodes to be reported even when DIRECT is present")
	}
}

func TestPublicClashSubscriptionGroupFilterAndFilename(t *testing.T) {
	const payload = `# Original profile
proxies:
  - name: HK
  - name: JP
proxy-groups:
  - name: 备用节点
    type: select
    proxies: [HK, JP]
  - name: 工作节点
    type: select
    proxies: [HK, JP, DIRECT, 备用节点]
rules: [ 'GEOIP,CN,DIRECT', 'MATCH,工作节点' ]
`
	for _, tt := range []struct {
		name                 string
		group                *string
		include              string
		wantBackup, wantWork []string
	}{
		{name: "named group", group: stringPointer("工作节点"), include: "HK", wantBackup: []string{"HK", "JP"}, wantWork: []string{"HK", "备用节点"}},
		{name: "multiple named groups", group: stringPointer("工作节点， 备用节点"), include: "HK", wantBackup: []string{"HK"}, wantWork: []string{"HK", "备用节点"}},
		{name: "blank group", group: stringPointer(""), include: "HK"},
		{name: "blank keywords", group: stringPointer("工作节点")},
		{name: "missing group field skips filtering even with old keywords", include: "HK"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			storePath := filepath.Join(dir, "subscriptions.json")
			record := map[string]any{
				"id": "test", "name": "我的订阅", "source_type": "content", "content": payload, "include": tt.include,
			}
			if tt.group != nil {
				record["proxy_group"] = *tt.group
			}
			records, err := json.Marshal([]map[string]any{record})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(storePath, records, 0600); err != nil {
				t.Fatal(err)
			}
			service := &APIV1Service{Profile: &profile.Profile{Data: dir}, clashSubscriptionStore: newClashVergeSubscriptionStore(storePath)}
			rec := httptest.NewRecorder()
			c := echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
			c.SetParamNames("id")
			c.SetParamValues("test")
			if err := service.getClashVergeSubscriptionConfig(c); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
			}
			if tt.wantBackup == nil {
				if rec.Body.String() != payload {
					t.Fatalf("unfiltered profile was changed: %s", rec.Body)
				}
			} else {
				var config struct {
					Groups []struct{ Proxies []string } `yaml:"proxy-groups"`
					Rules  []string
				}
				if err := yaml.Unmarshal(rec.Body.Bytes(), &config); err != nil {
					t.Fatal(err)
				}
				if len(config.Groups) != 2 || !reflect.DeepEqual(config.Groups[0].Proxies, tt.wantBackup) ||
					!reflect.DeepEqual(config.Groups[1].Proxies, tt.wantWork) || !reflect.DeepEqual(config.Rules, []string{"GEOIP,CN,DIRECT", "MATCH,工作节点"}) {
					t.Fatalf("unexpected config: %s", rec.Body)
				}
			}
			_, params, err := mime.ParseMediaType(rec.Header().Get(echo.HeaderContentDisposition))
			if err != nil || params["filename"] != "我的订阅.yaml" {
				t.Fatalf("unexpected filename: %v (%v)", params, err)
			}
			if got := rec.Header().Get(echo.HeaderCacheControl); got != "no-store" {
				t.Fatalf("Cache-Control = %q", got)
			}
		})
	}
}

func stringPointer(value string) *string {
	return &value
}

func TestValidateClashYAMLContentRejectsNonYAMLImports(t *testing.T) {
	if err := validateClashYAMLContent([]byte("vless://id@example.com:443#HK")); err == nil {
		t.Fatal("expected non-yaml import to be rejected")
	}
	if err := validateClashYAMLContent([]byte("proxies: []")); err == nil {
		t.Fatal("expected yaml without proxy-groups to be rejected")
	}
}

func TestContentSubscriptionForcesStaticUpdatePolicy(t *testing.T) {
	autoUpdate := true
	interval := int64(3600)
	request := &upsertClashVergeSubscriptionRequest{
		Name:                  "Static nodes",
		SourceType:            clashSubscriptionSourceContent,
		Content:               "proxies: []\nproxy-groups: []\nrules: []\n",
		AutoUpdate:            &autoUpdate,
		UpdateIntervalSeconds: &interval,
	}

	normalized, err := (&APIV1Service{}).validateAndNormalizeClashVergeSubscriptionInput(context.Background(), request)
	if err != nil {
		t.Fatalf("normalize static subscription: %v", err)
	}
	if *normalized.AutoUpdate || *normalized.UpdateIntervalSeconds != 0 {
		t.Fatalf("static subscription update policy = (%t, %d), want (false, 0)", *normalized.AutoUpdate, *normalized.UpdateIntervalSeconds)
	}
}

func TestClashSubscriptionPermissionErrorUsesSentinel(t *testing.T) {
	if !errors.Is(errClashSubscriptionPermissionDenied, errClashSubscriptionPermissionDenied) {
		t.Fatal("expected sentinel error to match itself")
	}
}

func TestFetchSubscriptionRejectsPrivateTargetBeforeSending(t *testing.T) {
	hits := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits++; _, _ = w.Write([]byte("private data")) }))
	defer target.Close()
	_, err := fetchSubscriptionContent(context.Background(), target.URL)
	if err == nil {
		t.Error("private subscription target was accepted")
	}
	if hits != 0 {
		t.Errorf("private server received %d requests", hits)
	}
}

func TestSubscriptionClientRejectsPrivateRedirect(t *testing.T) {
	client := newSubscriptionHTTPClient()
	defer client.CloseIdleConnections()
	for _, target := range []string{"http://127.0.0.1/private", "http://[::1]/private", "file:///etc/passwd", "http://localhost/private"} {
		request, err := http.NewRequest(http.MethodGet, target, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := client.CheckRedirect(request, nil); err == nil {
			t.Errorf("redirect to %s accepted", target)
		}
	}
}

func TestSubscriptionClientRejectsPrivateConnection(t *testing.T) {
	client := newSubscriptionHTTPClient()
	defer client.CloseIdleConnections()
	transport := client.Transport.(*http.Transport)
	if conn, err := transport.DialContext(context.Background(), "tcp", "127.0.0.1:80"); err == nil {
		conn.Close()
		t.Fatal("private connection accepted")
	}
}

func TestSubscriptionRejectsNonPublicAddresses(t *testing.T) {
	for _, host := range []string{"0.0.0.0", "::", "224.0.0.1", "ff02::1", "100.64.0.1", "198.18.0.1", "::ffff:127.0.0.1"} {
		t.Run(host, func(t *testing.T) {
			if !isPrivateIP(net.ParseIP(host)) {
				t.Fatalf("non-public address %s allowed", host)
			}
		})
	}
}
