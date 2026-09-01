package vsearch

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"

	"github.com/QuantumNous/new-api/common"
)

const standardCatalogVersion = "curated-2026-09-01"

type standardCapabilityDefinition struct {
	OperationKey    string
	ContractVersion string
	Name            string
	Category        string
	Description     string
	InputSchema     map[string]any
	OutputSchema    map[string]any
	Bindings        []ProviderOperation
}

func standardProviderCatalog(provider string) CatalogSnapshot {
	operations := standardProviderOperations(provider)
	payload, _ := common.Marshal(operations)
	digest := sha256.Sum256(payload)
	return CatalogSnapshot{
		Provider:   provider,
		Version:    standardCatalogVersion,
		SchemaHash: hex.EncodeToString(digest[:]),
		Operations: operations,
	}
}

func standardProviderOperations(provider string) []ProviderOperation {
	operations := make([]ProviderOperation, 0)
	for _, capability := range standardCapabilityRegistry() {
		for _, operation := range capability.Bindings {
			if provider == providerForMapping(operation.MappingKey) {
				operation.InputSchema = capability.InputSchema
				operation.OutputSchema = capability.OutputSchema
				operations = append(operations, operation)
			}
		}
	}
	sort.Slice(operations, func(i, j int) bool {
		if operations[i].OperationKey == operations[j].OperationKey {
			return operations[i].Platform < operations[j].Platform
		}
		return operations[i].OperationKey < operations[j].OperationKey
	})
	return operations
}

func providerForMapping(mappingKey string) string {
	switch mappingKey {
	case "justoneapi.direct.v1":
		return ProviderJustOneAPI
	case "tikhub.direct.v1":
		return ProviderTikHub
	default:
		return ""
	}
}

func mappingKeyForProvider(provider string) string {
	switch provider {
	case ProviderJustOneAPI:
		return justOneAPIDirectMappingKey
	case ProviderTikHub:
		return tikHubDirectMappingKey
	default:
		return ""
	}
}

func standardCapabilityRegistry() []standardCapabilityDefinition {
	accountOutput := entityOutputSchema("account")
	contentOutput := entityOutputSchema("content")
	commentListOutput := listOutputSchema("comment")
	contentListOutput := listOutputSchema("content")
	productOutput := entityOutputSchema("product")
	productListOutput := listOutputSchema("product")
	reviewListOutput := listOutputSchema("review")
	trendOutput := trendListOutputSchema()
	verifiedDouyinTrend := verifiedProviderGET("social.trend.list", "douyin", "fetch_hot_search_result_api_v1_douyin_web_fetch_hot_search_result_get", "/api/v1/douyin/web/fetch_hot_search_result", nil, nil, trendOutput, 1_000)

	return []standardCapabilityDefinition{
		{
			OperationKey: "social.account.get", ContractVersion: "v1", Name: "社交账号详情", Category: "社交媒体",
			Description: "按平台和公开账号标识获取标准化账号资料。",
			InputSchema: objectInputSchema(map[string]any{
				"platform":    stringSchema([]any{"youtube"}),
				"account_ref": stringSchema(nil),
			}, []any{"platform", "account_ref"}),
			OutputSchema: accountOutput,
			Bindings: []ProviderOperation{
				verifiedProviderGET("social.account.get", "youtube", "get_channel_info_api_v1_youtube_web_get_channel_info_get", "/api/v1/youtube/web/get_channel_info", map[string]string{"account_ref": "channel_id"}, nil, accountOutput, 1_000),
			},
		},
		{
			OperationKey: "social.account.contents.list", ContractVersion: "v1", Name: "账号公开内容", Category: "社交媒体",
			Description: "获取公开账号发布的内容列表。",
			InputSchema: objectInputSchema(map[string]any{
				"platform":    stringSchema([]any{"wechat_mp", "youtube"}),
				"account_ref": stringSchema(nil),
			}, []any{"platform", "account_ref"}),
			OutputSchema: contentListOutput,
			Bindings: []ProviderOperation{
				verifiedProviderPOST("social.account.contents.list", "wechat_mp", "fetch_account_articles_api_v1_wechat_mp_v2_fetch_account_articles_post", "/api/v1/wechat_mp/v2/fetch_account_articles", map[string]string{"account_ref": "username"}, map[string]any{"raw": false}, contentListOutput, 1_000),
				verifiedProviderGET("social.account.contents.list", "youtube", "get_channel_videos_api_v1_youtube_web_v2_get_channel_videos_get", "/api/v1/youtube/web_v2/get_channel_videos", map[string]string{"account_ref": "channel_id"}, map[string]any{"need_format": true}, contentListOutput, 1_000),
			},
		},
		{
			OperationKey: "social.content.get", ContractVersion: "v1", Name: "社交内容详情", Category: "社交媒体",
			Description: "获取公开帖子、笔记或视频的标准化详情。",
			InputSchema: objectInputSchema(map[string]any{
				"platform":    stringSchema([]any{"youtube"}),
				"content_ref": stringSchema(nil),
			}, []any{"platform", "content_ref"}),
			OutputSchema: contentOutput,
			Bindings: []ProviderOperation{
				verifiedProviderGET("social.content.get", "youtube", "get_video_info_v2_api_v1_youtube_web_v2_get_video_info_v2_get", "/api/v1/youtube/web_v2/get_video_info_v2", map[string]string{"content_ref": "video_id"}, map[string]any{"need_format": true}, contentOutput, 1_000),
			},
		},
		{
			OperationKey: "social.content.search", ContractVersion: "v1", Name: "社交内容搜索", Category: "社交媒体",
			Description: "按平台搜索公开内容。",
			InputSchema: objectInputSchema(map[string]any{
				"platform": stringSchema([]any{"youtube"}),
				"query":    stringSchema(nil),
			}, []any{"platform", "query"}),
			OutputSchema: contentListOutput,
			Bindings: []ProviderOperation{
				verifiedProviderGET("social.content.search", "youtube", "get_general_search_v2_api_v1_youtube_web_v2_get_general_search_v2_get", "/api/v1/youtube/web_v2/get_general_search_v2", map[string]string{"query": "keyword"}, map[string]any{"need_format": true, "type": "video"}, contentListOutput, 2_000),
			},
		},
		{
			OperationKey: "social.comment.list", ContractVersion: "v1", Name: "内容评论", Category: "社交媒体",
			Description: "获取公开内容的一级评论列表。",
			InputSchema: objectInputSchema(map[string]any{
				"platform":    stringSchema([]any{"youtube"}),
				"content_ref": stringSchema(nil),
			}, []any{"platform", "content_ref"}),
			OutputSchema: commentListOutput,
			Bindings: []ProviderOperation{
				verifiedProviderGET("social.comment.list", "youtube", "get_video_comments_api_v1_youtube_web_v2_get_video_comments_get", "/api/v1/youtube/web_v2/get_video_comments", map[string]string{"content_ref": "video_id"}, map[string]any{"need_format": true}, commentListOutput, 1_000),
			},
		},
		{
			OperationKey: "social.comment.replies.list", ContractVersion: "v1", Name: "评论回复", Category: "社交媒体",
			Description: "获取公开评论的回复列表。",
			InputSchema: objectInputSchema(map[string]any{
				"platform":    stringSchema([]any{"youtube"}),
				"content_ref": stringSchema(nil),
				"comment_ref": stringSchema(nil),
			}, []any{"platform", "content_ref", "comment_ref"}),
			OutputSchema: commentListOutput,
			Bindings: []ProviderOperation{
				verifiedProviderGET("social.comment.replies.list", "youtube", "get_video_comment_replies_api_v1_youtube_web_v2_get_video_comment_replies_get", "/api/v1/youtube/web_v2/get_video_comment_replies", map[string]string{"comment_ref": "continuation_token"}, map[string]any{"need_format": true}, commentListOutput, 1_000),
			},
		},
		{
			OperationKey: "social.trend.list", ContractVersion: "v1", Name: "平台趋势榜", Category: "社交媒体",
			Description: "获取指定平台的实时公开趋势榜单。",
			InputSchema: objectInputSchema(map[string]any{
				"platform": stringSchema([]any{"douyin"}),
			}, []any{"platform"}),
			OutputSchema: trendOutput,
			Bindings: []ProviderOperation{
				verifiedDouyinTrend,
			},
		},
		{
			OperationKey: "commerce.product.search", ContractVersion: "v1", Name: "商品搜索", Category: "电商",
			Description: "按平台和关键词搜索公开商品。",
			InputSchema: objectInputSchema(map[string]any{
				"platform": stringSchema([]any{"tiktok_shop"}),
				"query":    stringSchema(nil),
			}, []any{"platform", "query"}),
			OutputSchema: productListOutput,
			Bindings: []ProviderOperation{
				verifiedProviderGET("commerce.product.search", "tiktok_shop", "fetch_search_products_list_api_v1_tiktok_shop_web_fetch_search_products_list_get", "/api/v1/tiktok/shop/web/fetch_search_products_list", map[string]string{"query": "search_word"}, map[string]any{"offset": 0, "region": "US"}, productListOutput, 1_000),
			},
		},
		{
			OperationKey: "commerce.product.get", ContractVersion: "v1", Name: "商品详情", Category: "电商",
			Description: "获取公开商品的标准化详情。",
			InputSchema: objectInputSchema(map[string]any{
				"platform":    stringSchema([]any{"tiktok_shop"}),
				"product_ref": stringSchema(nil),
			}, []any{"platform", "product_ref"}),
			OutputSchema: productOutput,
			Bindings: []ProviderOperation{
				verifiedProviderGET("commerce.product.get", "tiktok_shop", "fetch_product_detail_v3_api_v1_tiktok_shop_web_fetch_product_detail_v3_get", "/api/v1/tiktok/shop/web/fetch_product_detail_v3", map[string]string{"product_ref": "product_id"}, map[string]any{"region": "US"}, productOutput, 1_000),
			},
		},
		{
			OperationKey: "commerce.product.reviews.list", ContractVersion: "v1", Name: "商品评价", Category: "电商",
			Description: "获取公开商品评价列表。",
			InputSchema: objectInputSchema(map[string]any{
				"platform":    stringSchema([]any{"tiktok_shop"}),
				"product_ref": stringSchema(nil),
			}, []any{"platform", "product_ref"}),
			OutputSchema: reviewListOutput,
			Bindings: []ProviderOperation{
				verifiedProviderGET("commerce.product.reviews.list", "tiktok_shop", "fetch_product_reviews_v2_api_v1_tiktok_shop_web_fetch_product_reviews_v2_get", "/api/v1/tiktok/shop/web/fetch_product_reviews_v2", map[string]string{"product_ref": "product_id"}, map[string]any{"page_start": 1, "sort_rule": 2, "filter_type": 1, "filter_value": 6, "region": "US"}, reviewListOutput, 1_000),
			},
		},
	}
}

func providerGET(operationKey, platform, operationID, path, mappingKey, authPlacement string, parameterMap map[string]string, fixedParams map[string]any, outputSchema map[string]any) ProviderOperation {
	costCurrency := ""
	if mappingKey == "tikhub.direct.v1" {
		costCurrency = "USD"
	}
	return ProviderOperation{
		OperationKey: operationKey, ContractVersion: "v1", Platform: platform,
		OperationID: operationID, Method: "GET", Path: path, AuthPlacement: authPlacement,
		MappingKey: mappingKey, MappingVersion: "v1", ParameterMap: parameterMap, FixedParams: fixedParams,
		OutputSchema: outputSchema, CostCurrency: costCurrency,
	}
}

func verifiedProviderGET(operationKey, platform, operationID, path string, parameterMap map[string]string, fixedParams map[string]any, outputSchema map[string]any, costAmountMicros int64) ProviderOperation {
	operation := providerGET(operationKey, platform, operationID, path, tikHubDirectMappingKey, AuthPlacementBearer, parameterMap, fixedParams, outputSchema)
	operation.CostAmountMicros = costAmountMicros
	operation.ContractEquivalent = true
	operation.BillingReady = true
	return operation
}

func providerPOST(operationKey, platform, operationID, path string, parameterMap map[string]string, fixedParams map[string]any, outputSchema map[string]any) ProviderOperation {
	operation := providerGET(operationKey, platform, operationID, path, tikHubDirectMappingKey, AuthPlacementBearer, parameterMap, fixedParams, outputSchema)
	operation.Method = "POST"
	return operation
}

func verifiedProviderPOST(operationKey, platform, operationID, path string, parameterMap map[string]string, fixedParams map[string]any, outputSchema map[string]any, costAmountMicros int64) ProviderOperation {
	operation := providerPOST(operationKey, platform, operationID, path, parameterMap, fixedParams, outputSchema)
	operation.CostAmountMicros = costAmountMicros
	operation.ContractEquivalent = true
	operation.BillingReady = true
	return operation
}

func objectInputSchema(properties map[string]any, required []any) map[string]any {
	return map[string]any{
		"type": "object", "properties": properties, "required": required, "additionalProperties": false,
	}
}

func stringSchema(enum []any) map[string]any {
	schema := map[string]any{"type": "string", "minLength": 1, "maxLength": 512}
	if len(enum) > 0 {
		schema["enum"] = enum
	}
	return schema
}

func entityOutputSchema(kind string) map[string]any {
	properties := map[string]any{
		"id": map[string]any{"type": "string"}, "type": map[string]any{"type": "string", "const": kind},
		"platform": map[string]any{"type": "string"}, "url": map[string]any{"type": "string"},
	}
	switch kind {
	case "account":
		properties["display_name"] = map[string]any{"type": "string"}
		properties["username"] = map[string]any{"type": "string"}
		properties["avatar_url"] = map[string]any{"type": "string"}
		properties["bio"] = map[string]any{"type": "string"}
		properties["verified"] = map[string]any{"type": "boolean"}
		properties["follower_count"] = map[string]any{"type": "integer", "minimum": 0}
		properties["following_count"] = map[string]any{"type": "integer", "minimum": 0}
		properties["content_count"] = map[string]any{"type": "integer", "minimum": 0}
	case "content", "product":
		properties["title"] = map[string]any{"type": "string"}
		properties["text"] = map[string]any{"type": "string"}
		properties["author"] = map[string]any{"type": "object", "additionalProperties": true}
		properties["published_at"] = map[string]any{"type": "string"}
		properties["media"] = map[string]any{"type": "array"}
		properties["metrics"] = map[string]any{"type": "object", "additionalProperties": true}
	case "comment", "review":
		properties["text"] = map[string]any{"type": "string"}
		properties["author"] = map[string]any{"type": "object", "additionalProperties": true}
		properties["published_at"] = map[string]any{"type": "string"}
		properties["like_count"] = map[string]any{"type": "integer", "minimum": 0}
		properties["reply_count"] = map[string]any{"type": "integer", "minimum": 0}
	}
	required := []any{"id", "platform"}
	if kind == "product" {
		required = append(required, "title")
	}
	return map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   required, "additionalProperties": true,
	}
}

func listOutputSchema(kind string) map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"items": map[string]any{"type": "array", "items": entityOutputSchema(kind)},
			"page": map[string]any{"type": "object", "properties": map[string]any{
				"cursor":   map[string]any{"type": []any{"string", "null"}},
				"has_more": map[string]any{"type": []any{"boolean", "null"}},
			}, "additionalProperties": false},
		},
		"required": []any{"items"}, "additionalProperties": false,
	}
}

func trendListOutputSchema() map[string]any {
	item := entityOutputSchema("trend")
	properties := item["properties"].(map[string]any)
	properties["title"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 512}
	properties["rank"] = map[string]any{"type": "integer", "minimum": 1}
	properties["score"] = map[string]any{"type": "number"}
	item["required"] = []any{"id", "platform", "title", "rank"}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"items": map[string]any{"type": "array", "items": item},
			"page": map[string]any{"type": "object", "properties": map[string]any{
				"cursor":   map[string]any{"type": []any{"string", "null"}},
				"has_more": map[string]any{"type": []any{"boolean", "null"}},
			}, "additionalProperties": false},
		},
		"required": []any{"items"}, "additionalProperties": false,
	}
}
