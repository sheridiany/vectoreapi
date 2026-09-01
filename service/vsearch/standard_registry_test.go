package vsearch

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStandardCapabilityRegistryPinsStablePublicIDs(t *testing.T) {
	expected := map[string]string{
		"social.account.get":            "vr_svc_703a7b675d910359",
		"social.account.contents.list":  "vr_svc_9599512edaae8e8b",
		"social.content.get":            "vr_svc_f38e121018dc5bae",
		"social.content.search":         "vr_svc_df956954dc93a1b6",
		"social.comment.list":           "vr_svc_7f672577f87c4642",
		"social.comment.replies.list":   "vr_svc_33712e613142165f",
		"social.trend.list":             "vr_svc_e86b09671dae57d3",
		"commerce.product.search":       "vr_svc_58d59c63f583781a",
		"commerce.product.get":          "vr_svc_dd31b9f15b6908aa",
		"commerce.product.reviews.list": "vr_svc_334afd6f79701f9b",
	}
	definitions := standardCapabilityRegistry()
	require.Len(t, definitions, len(expected), "adding or removing a public capability requires an explicit stable-ID review")
	seen := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		require.Equal(t, "v1", definition.ContractVersion, definition.OperationKey)
		wantID, exists := expected[definition.OperationKey]
		require.True(t, exists, "unreviewed canonical operation: %s", definition.OperationKey)
		publicID, err := model.GenerateSearchCapabilityPublicID(definition.OperationKey + "@" + definition.ContractVersion)
		require.NoError(t, err)
		assert.Equal(t, wantID, publicID, definition.OperationKey)
		_, duplicate := seen[publicID]
		assert.False(t, duplicate, definition.OperationKey)
		seen[publicID] = struct{}{}
	}
}

func TestStandardCapabilityRegistryContainsOnlyTikHubAndSafeHTTPBindings(t *testing.T) {
	allowedProviders := map[string]string{
		tikHubDirectMappingKey: ProviderTikHub,
	}
	wantAuth := map[string]string{
		ProviderTikHub: AuthPlacementBearer,
	}
	seenProviders := make(map[string]struct{}, len(allowedProviders))
	seenOperations := make(map[string]struct{})
	verifiedCosts := map[string]int64{
		"social.account.get/youtube":                1_000,
		"social.account.contents.list/wechat_mp":    1_000,
		"social.account.contents.list/youtube":      1_000,
		"social.content.get/youtube":                1_000,
		"social.content.search/youtube":             2_000,
		"social.comment.list/youtube":               1_000,
		"social.comment.replies.list/youtube":       1_000,
		"social.trend.list/douyin":                  1_000,
		"commerce.product.search/tiktok_shop":       1_000,
		"commerce.product.get/tiktok_shop":          1_000,
		"commerce.product.reviews.list/tiktok_shop": 1_000,
	}
	for _, definition := range standardCapabilityRegistry() {
		require.NotEmpty(t, definition.Bindings, definition.OperationKey)
		for _, operation := range definition.Bindings {
			provider, reviewed := allowedProviders[operation.MappingKey]
			require.True(t, reviewed, "%s has an unreviewed mapping %q", definition.OperationKey, operation.MappingKey)
			assert.Equal(t, provider, providerForMapping(operation.MappingKey))
			assert.Equal(t, operation.MappingKey, mappingKeyForProvider(provider))
			if provider == ProviderTikHub {
				assert.Equal(t, "USD", operation.CostCurrency, operation.OperationID)
			} else {
				assert.Empty(t, operation.CostCurrency, operation.OperationID)
			}
			expectedCost, verified := verifiedCosts[operation.OperationKey+"/"+operation.Platform]
			assert.Equal(t, verified, operation.ContractEquivalent, operation.OperationID)
			assert.Equal(t, verified, operation.BillingReady, operation.OperationID)
			if verified {
				assert.Equal(t, expectedCost, operation.CostAmountMicros, operation.OperationID)
			}
			seenProviders[provider] = struct{}{}

			assert.Equal(t, definition.OperationKey, operation.OperationKey)
			assert.Equal(t, definition.ContractVersion, operation.ContractVersion)
			assert.Contains(t, []string{http.MethodGet, http.MethodPost}, operation.Method, operation.OperationID)
			assert.Equal(t, "v1", operation.MappingVersion, operation.OperationID)
			assert.Equal(t, wantAuth[provider], operation.AuthPlacement, operation.OperationID)
			assert.NotEmpty(t, operation.Platform, operation.OperationID)
			assert.NotEmpty(t, operation.OperationID)
			assert.False(t, strings.Contains(strings.ToLower(strings.Join([]string{
				provider, operation.MappingKey, operation.OperationID, operation.Path,
			}, " ")), "agentkey"), operation.OperationID)

			parsed, err := url.ParseRequestURI(operation.Path)
			require.NoError(t, err, operation.OperationID)
			assert.True(t, strings.HasPrefix(operation.Path, "/api/"), operation.OperationID)
			assert.Empty(t, parsed.Scheme, operation.OperationID)
			assert.Empty(t, parsed.Host, operation.OperationID)
			assert.Empty(t, parsed.RawQuery, operation.OperationID)
			assert.Empty(t, parsed.Fragment, operation.OperationID)
			decodedPath, err := url.PathUnescape(operation.Path)
			require.NoError(t, err, operation.OperationID)
			assert.Equal(t, path.Clean(decodedPath), decodedPath, operation.OperationID)
			assert.NotContains(t, strings.TrimPrefix(decodedPath, "/"), "//", operation.OperationID)
			assert.NotContains(t, operation.Path, "{", operation.OperationID)
			assert.NotContains(t, operation.Path, "}", operation.OperationID)

			identity := provider + "|" + operation.OperationKey + "|" + operation.OperationID
			_, duplicate := seenOperations[identity]
			assert.False(t, duplicate, identity)
			seenOperations[identity] = struct{}{}
		}
	}
	assert.Equal(t, map[string]struct{}{ProviderTikHub: {}}, seenProviders)
	assert.Empty(t, standardProviderOperations(ProviderJustOneAPI))
	assert.Empty(t, standardProviderOperations("agentkey_mcp"))
	assert.Empty(t, providerForMapping("agentkey_mcp"))
}

func TestStandardCapabilityRegistryOnlyMapsWhitelistedCanonicalParameters(t *testing.T) {
	reviewedCanonicalParameters := map[string]struct{}{
		"platform": {}, "account_ref": {}, "content_ref": {}, "query": {}, "comment_ref": {}, "product_ref": {},
	}
	reviewedUpstreamParameters := map[string]struct{}{
		"username": {}, "uniqueId": {}, "secUid": {}, "maxCursor": {}, "noteId": {}, "keyword": {},
		"count": {}, "offset": {}, "page": {}, "ai_mode": {}, "sort": {}, "aweme_id": {}, "cursor": {},
		"item_id": {}, "comment_id": {}, "search_word": {}, "itemId": {}, "product_id": {}, "sku_id": {},
		"page_start": {}, "page_size": {}, "uid": {}, "raw": {}, "channel_id": {}, "url": {}, "id": {}, "query": {},
		"object_id": {}, "post_id": {}, "urn": {}, "business_type": {}, "continuation_token": {},
		"need_format": {}, "type": {}, "search_type": {}, "allow_nsfw": {}, "video_id": {}, "content_id": {},
		"sort_rule": {}, "filter_type": {}, "filter_value": {}, "region": {},
	}
	for _, definition := range standardCapabilityRegistry() {
		properties, ok := definition.InputSchema["properties"].(map[string]any)
		require.True(t, ok, definition.OperationKey)
		additionalProperties, ok := definition.InputSchema["additionalProperties"].(bool)
		require.True(t, ok, definition.OperationKey)
		assert.False(t, additionalProperties, definition.OperationKey)
		for canonicalName := range properties {
			_, reviewed := reviewedCanonicalParameters[canonicalName]
			assert.True(t, reviewed, definition.OperationKey+":"+canonicalName)
		}
		requiredProperties, ok := definition.InputSchema["required"].([]any)
		require.True(t, ok, definition.OperationKey)
		for _, requiredValue := range requiredProperties {
			requiredName, isString := requiredValue.(string)
			require.True(t, isString, definition.OperationKey)
			_, exists := properties[requiredName]
			assert.True(t, exists, definition.OperationKey+":"+requiredName)
		}

		platformSchema, ok := properties["platform"].(map[string]any)
		require.True(t, ok, definition.OperationKey)
		platformValues, ok := platformSchema["enum"].([]any)
		require.True(t, ok, definition.OperationKey)
		for _, operation := range definition.Bindings {
			assert.Contains(t, platformValues, any(operation.Platform), operation.OperationID)
			upstreamNames := make(map[string]struct{}, len(operation.ParameterMap)+len(operation.FixedParams))
			for canonicalName, upstreamName := range operation.ParameterMap {
				_, exists := properties[canonicalName]
				assert.True(t, exists, operation.OperationID+":"+canonicalName)
				assert.NotEmpty(t, strings.TrimSpace(upstreamName), operation.OperationID)
				_, reviewed := reviewedUpstreamParameters[upstreamName]
				assert.True(t, reviewed, operation.OperationID+":"+upstreamName)
				_, duplicate := upstreamNames[upstreamName]
				assert.False(t, duplicate, operation.OperationID+":"+upstreamName)
				upstreamNames[upstreamName] = struct{}{}
			}
			for upstreamName := range operation.FixedParams {
				assert.NotEmpty(t, strings.TrimSpace(upstreamName), operation.OperationID)
				_, reviewed := reviewedUpstreamParameters[upstreamName]
				assert.True(t, reviewed, operation.OperationID+":"+upstreamName)
				_, duplicate := upstreamNames[upstreamName]
				assert.False(t, duplicate, operation.OperationID+":"+upstreamName)
				upstreamNames[upstreamName] = struct{}{}
			}
		}
	}
}

func TestStandardCapabilityRegistryPublishesReviewedPlatformSet(t *testing.T) {
	platforms := make(map[string]struct{})
	for _, definition := range standardCapabilityRegistry() {
		for _, operation := range definition.Bindings {
			platforms[operation.Platform] = struct{}{}
		}
	}
	assert.Equal(t, map[string]struct{}{
		"douyin": {}, "tiktok_shop": {}, "wechat_mp": {}, "youtube": {},
	}, platforms)
}

func TestEveryAdvertisedPlatformBindingIsExecutable(t *testing.T) {
	for _, definition := range standardCapabilityRegistry() {
		properties := definition.InputSchema["properties"].(map[string]any)
		platforms := properties["platform"].(map[string]any)["enum"].([]any)
		require.Len(t, definition.Bindings, len(platforms), definition.OperationKey)
		for _, platform := range platforms {
			var operation *ProviderOperation
			for index := range definition.Bindings {
				if definition.Bindings[index].Platform == platform {
					operation = &definition.Bindings[index]
					break
				}
			}
			require.NotNil(t, operation, definition.OperationKey+"/"+platform.(string)+" has no binding")
			identity := operation.OperationKey + "/" + operation.Platform
			assert.True(t, operation.ContractEquivalent, identity+" is advertised but not contract verified")
			assert.True(t, operation.BillingReady, identity+" is advertised but not billing ready")
			assert.Positive(t, operation.CostAmountMicros, identity+" has no verified upstream cost")
			assert.Equal(t, "USD", operation.CostCurrency, identity+" has no verified upstream currency")
		}
	}
}

func TestStandardCapabilityRegistryExcludesJustOneAPI(t *testing.T) {
	for _, definition := range standardCapabilityRegistry() {
		for _, operation := range definition.Bindings {
			assert.NotEqual(t, justOneAPIDirectMappingKey, operation.MappingKey)
			assert.NotContains(t, strings.ToLower(operation.Path), "justoneapi")
		}
	}
}

func TestStandardTrendOutputContractRequiresDisplayFields(t *testing.T) {
	var outputSchema map[string]any
	for _, definition := range standardCapabilityRegistry() {
		if definition.OperationKey == "social.trend.list" {
			outputSchema = definition.OutputSchema
			break
		}
	}
	require.NotNil(t, outputSchema)
	assert.NoError(t, validateSchemaValue(map[string]any{
		"items": []any{map[string]any{
			"id": "trend-1", "type": "trend", "platform": "douyin",
			"title": "AI Agent", "rank": 1, "score": float64(987654),
		}},
	}, outputSchema, "$"))
	assert.Error(t, validateSchemaValue(map[string]any{
		"items": []any{map[string]any{"id": "trend-1", "type": "trend", "platform": "douyin"}},
	}, outputSchema, "$"))
}

func TestStandardProviderCatalogHashAndOrderAreDeterministic(t *testing.T) {
	hashes := make(map[string]string, 1)
	for _, provider := range []string{ProviderTikHub} {
		first := standardProviderCatalog(provider)
		require.NotEmpty(t, first.Operations)
		require.Equal(t, provider, first.Provider)
		require.Equal(t, standardCatalogVersion, first.Version)
		digest, err := hex.DecodeString(first.SchemaHash)
		require.NoError(t, err)
		assert.Len(t, digest, sha256.Size)

		payload, err := common.Marshal(first.Operations)
		require.NoError(t, err)
		wantHash := sha256.Sum256(payload)
		assert.Equal(t, hex.EncodeToString(wantHash[:]), first.SchemaHash)
		hashes[provider] = first.SchemaHash

		for iteration := 0; iteration < 5; iteration++ {
			assert.Equal(t, first, standardProviderCatalog(provider))
		}
		keys := make([]string, 0, len(first.Operations))
		for _, operation := range first.Operations {
			keys = append(keys, operation.OperationKey+"\x00"+operation.Platform)
		}
		assert.True(t, sort.StringsAreSorted(keys), provider)
	}
	assert.NotEmpty(t, hashes[ProviderTikHub])
}
