package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const daemonBaseURL = "http://lara-nuxd"

type RemoteError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *RemoteError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Code) == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

type Client struct {
	socketPath string
	baseURL    string
	httpClient *http.Client
}

type rpcEnvelope[T any] struct {
	OK    bool      `json:"ok"`
	Data  T         `json:"data"`
	Error *RpcError `json:"error,omitempty"`
}

func NewClient(socketPath string) *Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}

	return &Client{
		socketPath: socketPath,
		baseURL:    daemonBaseURL,
		httpClient: &http.Client{Transport: transport, Timeout: 10 * time.Second},
	}
}

func newClientForTests(baseURL string, httpClient *http.Client) *Client {
	return &Client{baseURL: baseURL, httpClient: httpClient}
}

func (c *Client) SocketPath() string {
	return c.socketPath
}

func (c *Client) Health(ctx context.Context) (HealthReport, error) {
	return get[HealthReport](ctx, c, "/rpc/health")
}

func (c *Client) ListSites(ctx context.Context) ([]SiteRecord, error) {
	return get[[]SiteRecord](ctx, c, "/rpc/sites.list")
}

func (c *Client) GetSite(ctx context.Context, siteID string) (SiteRecord, error) {
	target := "/rpc/sites.get?siteId=" + url.QueryEscape(strings.TrimSpace(siteID))
	return get[SiteRecord](ctx, c, target)
}

func (c *Client) RegisterSite(ctx context.Context, request RegisterSiteRequest) (ActivationResult, error) {
	return post[RegisterSiteRequest, ActivationResult](ctx, c, "/rpc/sites.register", request)
}

func (c *Client) UpdateSite(ctx context.Context, request UpdateSiteRequest) (SiteRecord, error) {
	return post[UpdateSiteRequest, SiteRecord](ctx, c, "/rpc/sites.update", request)
}

func (c *Client) RuntimeCatalog(ctx context.Context) (RuntimeCatalog, error) {
	return get[RuntimeCatalog](ctx, c, "/rpc/php.inventory")
}

func (c *Client) SetDefaultRuntime(ctx context.Context, request SetDefaultPHPRequest) (DefaultRuntimeResponse, error) {
	return post[SetDefaultPHPRequest, DefaultRuntimeResponse](ctx, c, "/rpc/php.default", request)
}

func (c *Client) SwitchSiteRuntime(ctx context.Context, request SwitchPHPRequest) (SiteRecord, error) {
	return post[SwitchPHPRequest, SiteRecord](ctx, c, "/rpc/php.switch", request)
}

func (c *Client) ServiceAction(ctx context.Context, request ServiceActionRequest) (ServiceStatus, error) {
	return post[ServiceActionRequest, ServiceStatus](ctx, c, "/rpc/services.action", request)
}

func get[T any](ctx context.Context, client *Client, path string) (T, error) {
	var zero T
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+path, nil)
	if err != nil {
		return zero, err
	}
	return do[T](client, request)
}

func post[Req any, Resp any](ctx context.Context, client *Client, path string, payload Req) (Resp, error) {
	var zero Resp
	body, err := json.Marshal(payload)
	if err != nil {
		return zero, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return zero, err
	}
	request.Header.Set("Content-Type", "application/json")
	return do[Resp](client, request)
}

func do[T any](client *Client, request *http.Request) (T, error) {
	var zero T
	response, err := client.httpClient.Do(request)
	if err != nil {
		return zero, err
	}
	defer func() { _ = response.Body.Close() }()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return zero, err
	}

	var envelope rpcEnvelope[T]
	if err := json.Unmarshal(body, &envelope); err != nil {
		return zero, fmt.Errorf("decode daemon response: %w", err)
	}

	if !envelope.OK {
		remoteErr := &RemoteError{StatusCode: response.StatusCode}
		if envelope.Error != nil {
			remoteErr.Code = envelope.Error.Code
			remoteErr.Message = envelope.Error.Message
		}
		return zero, remoteErr
	}

	return envelope.Data, nil
}
