package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jopsam/lara-nux/daemon/internal/app"
)

func TestRPCContractsGoldenCompatibility(t *testing.T) {
	t.Parallel()

	fixtures := loadContractFixtures(t)
	schema := loadContractSchema(t)

	tests := []struct {
		name               string
		endpoint           string
		method             string
		target             string
		status             int
		requestFixtureKey  string
		responseFixtureKey string
		setup              func(t *testing.T, ctx *contractCaseContext)
	}{
		{
			name:               "sites register",
			endpoint:           "sites.register",
			method:             http.MethodPost,
			target:             "/rpc/sites.register",
			status:             http.StatusCreated,
			requestFixtureKey:  "sites.register.request",
			responseFixtureKey: "sites.register.response",
			setup: func(t *testing.T, ctx *contractCaseContext) {
				projectC := createLaravelProject(t, filepath.Join(ctx.tempDir, "charlie"))
				ctx.replacements["__PROJECT_C__"] = projectC
			},
		},
		{
			name:               "sites list",
			endpoint:           "sites.list",
			method:             http.MethodGet,
			target:             "/rpc/sites.list",
			status:             http.StatusOK,
			responseFixtureKey: "sites.list.response",
			setup:              seedAlphaBravoSites,
		},
		{
			name:               "sites get",
			endpoint:           "sites.get",
			method:             http.MethodGet,
			target:             "/rpc/sites.get",
			status:             http.StatusOK,
			requestFixtureKey:  "sites.get.request",
			responseFixtureKey: "sites.get.response",
			setup: func(t *testing.T, ctx *contractCaseContext) {
				seedAlphaBravoSites(t, ctx)
				ctx.replacements["__SITE_A_ID__"] = ctx.siteA.ID
			},
		},
		{
			name:               "sites update",
			endpoint:           "sites.update",
			method:             http.MethodPost,
			target:             "/rpc/sites.update",
			status:             http.StatusOK,
			requestFixtureKey:  "sites.update.request",
			responseFixtureKey: "sites.update.response",
			setup: func(t *testing.T, ctx *contractCaseContext) {
				seedAlphaBravoSites(t, ctx)
				ctx.replacements["__SITE_A_ID__"] = ctx.siteA.ID
			},
		},
		{
			name:               "php list",
			endpoint:           "php.list",
			method:             http.MethodGet,
			target:             "/rpc/php.list",
			status:             http.StatusOK,
			responseFixtureKey: "php.list.response",
		},
		{
			name:               "php default get",
			endpoint:           "php.default.get",
			method:             http.MethodGet,
			target:             "/rpc/php.default",
			status:             http.StatusOK,
			responseFixtureKey: "php.default.get.response",
		},
		{
			name:               "php default set",
			endpoint:           "php.default.set",
			method:             http.MethodPost,
			target:             "/rpc/php.default",
			status:             http.StatusOK,
			requestFixtureKey:  "php.default.set.request",
			responseFixtureKey: "php.default.set.response",
		},
		{
			name:               "php inventory",
			endpoint:           "php.inventory",
			method:             http.MethodGet,
			target:             "/rpc/php.inventory",
			status:             http.StatusOK,
			responseFixtureKey: "php.inventory.response",
		},
		{
			name:               "php register",
			endpoint:           "php.register",
			method:             http.MethodPost,
			target:             "/rpc/php.register",
			status:             http.StatusCreated,
			requestFixtureKey:  "php.register.request",
			responseFixtureKey: "php.register.response",
			setup: func(t *testing.T, ctx *contractCaseContext) {
				binary84 := filepath.Join(ctx.tempDir, "php84")
				writeVersionExecutable(t, binary84, "8.4")
				ctx.replacements["__PHP84_BINARY__"] = binary84
			},
		},
		{
			name:               "php switch",
			endpoint:           "php.switch",
			method:             http.MethodPost,
			target:             "/rpc/php.switch",
			status:             http.StatusOK,
			requestFixtureKey:  "php.switch.request",
			responseFixtureKey: "php.switch.response",
			setup: func(t *testing.T, ctx *contractCaseContext) {
				seedAlphaBravoSites(t, ctx)
				ctx.replacements["__SITE_A_ID__"] = ctx.siteA.ID
			},
		},
		{
			name:               "services action",
			endpoint:           "services.action",
			method:             http.MethodPost,
			target:             "/rpc/services.action",
			status:             http.StatusOK,
			requestFixtureKey:  "services.action.request",
			responseFixtureKey: "services.action.response",
		},
		{
			name:     "health schema compatibility",
			endpoint: "health",
			method:   http.MethodGet,
			target:   "/rpc/health",
			status:   http.StatusOK,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := newContractCaseContext(t)
			if tt.setup != nil {
				tt.setup(t, ctx)
			}

			var requestBody any
			if tt.requestFixtureKey != "" {
				fixture := cloneFixture(fixtures[tt.requestFixtureKey])
				requestBody = expandPlaceholders(fixture, ctx.replacements)
				validateSchemaDocument(t, schema, requestBody, schema.endpointRequestSchema(tt.endpoint), tt.endpoint+" request")
			}

			target := tt.target
			if tt.method == http.MethodGet && tt.requestFixtureKey != "" {
				requestMap, ok := requestBody.(map[string]any)
				if !ok {
					t.Fatalf("expected request fixture %s to be an object", tt.requestFixtureKey)
				}
				target = target + "?" + encodeQuery(requestMap)
				requestBody = nil
			}

			response := performJSONRequest(t, ctx.router, tt.method, target, requestBody)
			if response.Code != tt.status {
				t.Fatalf("expected status %d, got %d: %s", tt.status, response.Code, response.Body.String())
			}

			var actual any
			if err := json.Unmarshal(response.Body.Bytes(), &actual); err != nil {
				t.Fatalf("decode response JSON: %v", err)
			}
			validateSchemaDocument(t, schema, actual, schema.endpointResponseSchema(tt.endpoint), tt.endpoint+" response")

			if tt.responseFixtureKey == "" {
				return
			}

			normalizedActual := normalizeContractValue(actual, invertReplacements(ctx.replacements))
			expected := cloneFixture(fixtures[tt.responseFixtureKey])
			assertJSONEqual(t, expected, normalizedActual)
		})
	}
}

type contractCaseContext struct {
	tempDir      string
	router       http.Handler
	deps         *routerTestDeps
	replacements map[string]string
	siteA        app.SiteRecord
	siteB        app.SiteRecord
}

func newContractCaseContext(t *testing.T) *contractCaseContext {
	t.Helper()

	deps := newRouterTestDeps(t)
	return &contractCaseContext{
		tempDir: depsDir(deps.projectA),
		router:  NewRouter(deps.routerDeps()),
		deps:    deps,
		replacements: map[string]string{
			"__PROJECT_A__":    deps.projectA,
			"__PROJECT_B__":    deps.projectB,
			"__PHP82_BINARY__": filepath.Join(depsDir(deps.projectA), "php82"),
			"__PHP83_BINARY__": filepath.Join(depsDir(deps.projectA), "php83"),
		},
	}
}

func seedAlphaBravoSites(t *testing.T, ctx *contractCaseContext) {
	t.Helper()
	ctx.siteA = seedSite(t, ctx.deps, seedSiteInput{RootPath: ctx.deps.projectA, Domain: "alpha.test", PHPVersion: "8.2"})
	ctx.siteB = seedSite(t, ctx.deps, seedSiteInput{RootPath: ctx.deps.projectB, Domain: "bravo.test", PHPVersion: "8.2"})
	ctx.replacements["__SITE_A_ID__"] = ctx.siteA.ID
	ctx.replacements["__SITE_B_ID__"] = ctx.siteB.ID
	ctx.router = NewRouter(ctx.deps.routerDeps())
}

func depsDir(projectPath string) string {
	return filepath.Dir(projectPath)
}

func loadContractFixtures(t *testing.T) map[string]any {
	t.Helper()

	path := filepath.Join("testdata", "contracts", "v1.golden.json")
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}

	var fixtures map[string]any
	if err := json.Unmarshal(payload, &fixtures); err != nil {
		t.Fatalf("decode fixtures: %v", err)
	}
	return fixtures
}

type contractSchema struct {
	root map[string]any
}

func loadContractSchema(t *testing.T) contractSchema {
	t.Helper()

	path := filepath.Join("..", "..", "..", "shared", "contracts", "rpc", "v1", "contracts.schema.json")
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}

	var root map[string]any
	if err := json.Unmarshal(payload, &root); err != nil {
		t.Fatalf("decode schema: %v", err)
	}
	return contractSchema{root: root}
}

func (s contractSchema) endpointRequestSchema(endpoint string) map[string]any {
	endpointSchema := s.endpointSchema(endpoint)
	properties := mustMap(endpointSchema["properties"])
	request, ok := properties["request"]
	if !ok {
		return nil
	}
	return mustMap(request)
}

func (s contractSchema) endpointResponseSchema(endpoint string) map[string]any {
	endpointSchema := s.endpointSchema(endpoint)
	properties := mustMap(endpointSchema["properties"])
	return mustMap(properties["response"])
}

func (s contractSchema) endpointSchema(endpoint string) map[string]any {
	properties := mustMap(s.root["properties"])
	endpoints := mustMap(properties["endpoints"])
	endpointProperties := mustMap(endpoints["properties"])
	return mustMap(endpointProperties[endpoint])
}

func validateSchemaDocument(t *testing.T, schema contractSchema, value any, rule map[string]any, label string) {
	t.Helper()
	if rule == nil {
		return
	}
	if err := schema.validate(rule, value, label); err != nil {
		t.Fatalf("schema validation failed for %s: %v", label, err)
	}
}

func (s contractSchema) validate(rule map[string]any, value any, path string) error {
	resolved, err := s.resolveRule(rule)
	if err != nil {
		return err
	}

	if oneOf, ok := resolved["oneOf"].([]any); ok {
		var errs []string
		for _, option := range oneOf {
			optionRule := mustMap(option)
			if err := s.validate(optionRule, value, path); err == nil {
				return nil
			} else {
				errs = append(errs, err.Error())
			}
		}
		return fmt.Errorf("%s: no oneOf schema matched (%s)", path, strings.Join(errs, "; "))
	}

	if enumValues, ok := resolved["enum"].([]any); ok {
		for _, candidate := range enumValues {
			if reflect.DeepEqual(value, candidate) {
				return nil
			}
		}
		return fmt.Errorf("%s: %v is not in enum %v", path, value, enumValues)
	}

	if constant, ok := resolved["const"]; ok {
		if !reflect.DeepEqual(value, constant) {
			return fmt.Errorf("%s: expected const %v, got %v", path, constant, value)
		}
	}

	typeName, _ := resolved["type"].(string)
	switch typeName {
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: expected object, got %T", path, value)
		}

		properties := map[string]any{}
		if rawProperties, ok := resolved["properties"]; ok {
			properties = mustMap(rawProperties)
		}

		if required, ok := resolved["required"].([]any); ok {
			for _, entry := range required {
				key := entry.(string)
				if _, exists := object[key]; !exists {
					return fmt.Errorf("%s: missing required property %q", path, key)
				}
			}
		}

		if additionalAllowed, ok := resolved["additionalProperties"].(bool); ok && !additionalAllowed {
			for key := range object {
				if _, exists := properties[key]; !exists {
					return fmt.Errorf("%s: unexpected property %q", path, key)
				}
			}
		}

		keys := make([]string, 0, len(properties))
		for key := range properties {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			child, exists := object[key]
			if !exists {
				continue
			}
			if err := s.validate(mustMap(properties[key]), child, path+"."+key); err != nil {
				return err
			}
		}
	case "array":
		items, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s: expected array, got %T", path, value)
		}
		itemRule := mustMap(resolved["items"])
		for index, item := range items {
			if err := s.validate(itemRule, item, fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
	case "string":
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s: expected string, got %T", path, value)
		}
		if format, _ := resolved["format"].(string); format == "date-time" {
			if _, err := time.Parse(time.RFC3339, text); err != nil {
				if _, nanoErr := time.Parse(time.RFC3339Nano, text); nanoErr != nil {
					return fmt.Errorf("%s: expected RFC3339 timestamp, got %q", path, text)
				}
			}
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s: expected boolean, got %T", path, value)
		}
	case "null":
		if value != nil {
			return fmt.Errorf("%s: expected null, got %T", path, value)
		}
	case "":
		if len(resolved) == 0 {
			return nil
		}
	}

	return nil
}

func (s contractSchema) resolveRule(rule map[string]any) (map[string]any, error) {
	if rule == nil {
		return nil, nil
	}
	if ref, ok := rule["$ref"].(string); ok {
		return s.resolveRef(ref)
	}
	return rule, nil
}

func (s contractSchema) resolveRef(ref string) (map[string]any, error) {
	if !strings.HasPrefix(ref, "#/") {
		return nil, fmt.Errorf("unsupported ref: %s", ref)
	}
	current := any(s.root)
	for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		currentMap, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid ref segment %s in %s", part, ref)
		}
		current = currentMap[part]
	}
	return mustMap(current), nil
}

func mustMap(value any) map[string]any {
	result, ok := value.(map[string]any)
	if !ok {
		panic(fmt.Sprintf("expected map[string]any, got %T", value))
	}
	return result
}

func cloneFixture(value any) any {
	payload, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	var cloned any
	if err := json.Unmarshal(payload, &cloned); err != nil {
		panic(err)
	}
	return cloned
}

func expandPlaceholders(value any, replacements map[string]string) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			result[key] = expandPlaceholders(child, replacements)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = expandPlaceholders(child, replacements)
		}
		return result
	case string:
		if replacement, ok := replacements[typed]; ok {
			return replacement
		}
		return typed
	default:
		return value
	}
}

func invertReplacements(replacements map[string]string) map[string]string {
	result := make(map[string]string, len(replacements))
	for placeholder, actual := range replacements {
		result[actual] = placeholder
	}
	return result
}

func normalizeContractValue(value any, replacements map[string]string) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			result[key] = normalizeContractField(key, child, replacements)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = normalizeContractValue(child, replacements)
		}
		return result
	default:
		return normalizeContractScalar(typed, replacements)
	}
}

func normalizeContractField(key string, value any, replacements map[string]string) any {
	if text, ok := value.(string); ok {
		if strings.HasSuffix(key, "At") {
			if _, err := time.Parse(time.RFC3339, text); err == nil {
				return "__TIMESTAMP__"
			}
			if _, err := time.Parse(time.RFC3339Nano, text); err == nil {
				return "__TIMESTAMP__"
			}
		}
		if key == "id" && strings.HasPrefix(text, "site-") {
			if replacement, ok := replacements[text]; ok {
				return replacement
			}
			return "__SITE_ID__"
		}
	}
	return normalizeContractValue(value, replacements)
}

func normalizeContractScalar(value any, replacements map[string]string) any {
	text, ok := value.(string)
	if !ok {
		return value
	}
	if replacement, ok := replacements[text]; ok {
		return replacement
	}
	if _, err := time.Parse(time.RFC3339, text); err == nil {
		return "__TIMESTAMP__"
	}
	if _, err := time.Parse(time.RFC3339Nano, text); err == nil {
		return "__TIMESTAMP__"
	}
	return text
}

func encodeQuery(values map[string]any) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", key, values[key]))
	}
	return strings.Join(parts, "&")
}

func assertJSONEqual(t *testing.T, expected any, actual any) {
	t.Helper()
	if reflect.DeepEqual(expected, actual) {
		return
	}
	expectedJSON := prettyJSON(t, expected)
	actualJSON := prettyJSON(t, actual)
	t.Fatalf("golden mismatch\nexpected:\n%s\nactual:\n%s", expectedJSON, actualJSON)
}

func prettyJSON(t *testing.T, value any) string {
	t.Helper()
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal pretty JSON: %v", err)
	}
	return string(bytes.TrimSpace(payload))
}
