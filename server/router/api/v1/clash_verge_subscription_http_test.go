package v1

import (
	"context"
	"errors"
	"strings"
	"testing"
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

	filtered, matched, err := filterSubscriptionContent(payload, []string{"香港"})
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
		Include:               "HK",
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
