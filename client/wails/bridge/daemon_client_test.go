package bridge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientListSites(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rpc/sites.list" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"data": []map[string]any{{
				"id": "site-1",
				"name": "alpha",
				"rootPath": "/srv/alpha",
				"domain": "alpha.test",
				"phpVersion": "8.2",
				"tls": "auto",
				"status": "ready",
				"createdAt": "2026-04-18T00:00:00Z",
				"updatedAt": "2026-04-18T00:00:00Z",
			}},
		})
	}))
	defer server.Close()

	client := newClientForTests(server.URL, server.Client())
	sites, err := client.ListSites(context.Background())
	if err != nil {
		t.Fatalf("list sites: %v", err)
	}
	if len(sites) != 1 {
		t.Fatalf("expected one site, got %d", len(sites))
	}
	if sites[0].Domain != "alpha.test" {
		t.Fatalf("expected alpha.test, got %s", sites[0].Domain)
	}
}

func TestClientMapsRemoteErrors(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": false,
			"error": map[string]any{
				"code": "conflict",
				"message": "duplicate site domain: alpha.test",
			},
		})
	}))
	defer server.Close()

	client := newClientForTests(server.URL, server.Client())
	_, err := client.RegisterSite(context.Background(), RegisterSiteRequest{RootPath: "/srv/alpha"})
	if err == nil {
		t.Fatal("expected conflict error")
	}

	remoteErr, ok := err.(*RemoteError)
	if !ok {
		t.Fatalf("expected RemoteError, got %T", err)
	}
	if remoteErr.Code != "conflict" {
		t.Fatalf("expected conflict code, got %s", remoteErr.Code)
	}
}

func TestClientPostsJSONPayloads(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		var payload SetDefaultPHPRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Version != "8.3" {
			t.Fatalf("expected runtime 8.3, got %s", payload.Version)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"data": map[string]any{
				"runtime": map[string]any{
					"version": "8.3",
					"binaryPath": "/usr/bin/php8.3",
					"fpmService": "php8.3-fpm",
					"registeredAt": "2026-04-18T00:00:00Z",
				},
			},
		})
	}))
	defer server.Close()

	client := newClientForTests(server.URL, server.Client())
	result, err := client.SetDefaultRuntime(context.Background(), SetDefaultPHPRequest{Version: "8.3"})
	if err != nil {
		t.Fatalf("set default runtime: %v", err)
	}
	if result.Runtime == nil || result.Runtime.Version != "8.3" {
		t.Fatalf("expected runtime 8.3, got %+v", result.Runtime)
	}
}
