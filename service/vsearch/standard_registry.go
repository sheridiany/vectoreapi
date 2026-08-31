package vsearch

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"

	"github.com/QuantumNous/new-api/common"
)

const standardCatalogVersion = "curated-2026-08-29"

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

	return []standardCapabilityDefinition{
		{
			OperationKey: "social.account.get", ContractVersion: "v1", Name: "社交账号详情", Category: "社交媒体",
			Description: "按平台和公开账号标识获取标准化账号资料。",
			InputSchema: objectInputSchema(map[string]any{
				"platform":    stringSchema([]any{"instagram", "tiktok"}),
				"account_ref": stringSchema(nil),
			}, []any{"platform", "account_ref"}),
			OutputSchema: accountOutput,
			Bindings: []ProviderOperation{
				providerGET("social.account.get", "instagram", "getApiInstagramGetUserDetailV2", "/api/instagram/get-user-detail/v2", "justoneapi.direct.v1", AuthPlacementQuery, map[string]string{"account_ref": "username"}, nil, accountOutput),
				providerGET("social.account.get", "tiktok", "fetch_user_profile_api_v1_tiktok_web_fetch_user_profile_get", "/api/v1/tiktok/web/fetch_user_profile", "tikhub.direct.v1", AuthPlacementBearer, map[string]string{"account_ref": "uniqueId"}, nil, accountOutput),
			},
		},
		{
			OperationKey: "social.account.contents.list", ContractVersion: "v1", Name: "账号公开内容", Category: "社交媒体",
			Description: "获取公开账号发布的内容列表。",
			InputSchema: objectInputSchema(map[string]any{
				"platform":    stringSchema([]any{"douyin"}),
				"account_ref": stringSchema(nil),
			}, []any{"platform", "account_ref"}),
			OutputSchema: contentListOutput,
			Bindings: []ProviderOperation{
				providerGET("social.account.contents.list", "douyin", "getApiDouyinGetUserVideoListV1", "/api/douyin/get-user-video-list/v1", "justoneapi.direct.v1", AuthPlacementQuery, map[string]string{"account_ref": "secUid"}, map[string]any{"maxCursor": "0"}, contentListOutput),
			},
		},
		{
			OperationKey: "social.content.get", ContractVersion: "v1", Name: "社交内容详情", Category: "社交媒体",
			Description: "获取公开帖子、笔记或视频的标准化详情。",
			InputSchema: objectInputSchema(map[string]any{
				"platform":    stringSchema([]any{"xiaohongshu"}),
				"content_ref": stringSchema(nil),
			}, []any{"platform", "content_ref"}),
			OutputSchema: contentOutput,
			Bindings: []ProviderOperation{
				providerGET("social.content.get", "xiaohongshu", "getApiXiaohongshuGetNoteDetailV1", "/api/xiaohongshu/get-note-detail/v1", "justoneapi.direct.v1", AuthPlacementQuery, map[string]string{"content_ref": "noteId"}, nil, contentOutput),
			},
		},
		{
			OperationKey: "social.content.search", ContractVersion: "v1", Name: "社交内容搜索", Category: "社交媒体",
			Description: "按平台搜索公开内容。",
			InputSchema: objectInputSchema(map[string]any{
				"platform": stringSchema([]any{"douyin", "tiktok", "xiaohongshu"}),
				"query":    stringSchema(nil),
			}, []any{"platform", "query"}),
			OutputSchema: contentListOutput,
			Bindings: []ProviderOperation{
				providerGET("social.content.search", "douyin", "getApiDouyinHotSearchV1", "/api/douyin/hot-search/v1", "justoneapi.direct.v1", AuthPlacementQuery, map[string]string{"query": "keyword"}, map[string]any{"page": 1}, contentListOutput),
				providerGET("social.content.search", "tiktok", "fetch_search_video_api_v1_tiktok_web_fetch_search_video_get", "/api/v1/tiktok/web/fetch_search_video", "tikhub.direct.v1", AuthPlacementBearer, map[string]string{"query": "keyword"}, map[string]any{"count": 20, "offset": 0}, contentListOutput),
				providerGET("social.content.search", "xiaohongshu", "search_notes_api_v1_xiaohongshu_app_v2_search_notes_get", "/api/v1/xiaohongshu/app_v2/search_notes", "tikhub.direct.v1", AuthPlacementBearer, map[string]string{"query": "keyword"}, map[string]any{"page": 1, "ai_mode": 0}, contentListOutput),
			},
		},
		{
			OperationKey: "social.comment.list", ContractVersion: "v1", Name: "内容评论", Category: "社交媒体",
			Description: "获取公开内容的一级评论列表。",
			InputSchema: objectInputSchema(map[string]any{
				"platform":    stringSchema([]any{"xiaohongshu", "tiktok"}),
				"content_ref": stringSchema(nil),
			}, []any{"platform", "content_ref"}),
			OutputSchema: commentListOutput,
			Bindings: []ProviderOperation{
				providerGET("social.comment.list", "xiaohongshu", "getApiXiaohongshuGetNoteCommentV2", "/api/xiaohongshu/get-note-comment/v2", "justoneapi.direct.v1", AuthPlacementQuery, map[string]string{"content_ref": "noteId"}, map[string]any{"sort": "latest"}, commentListOutput),
				providerGET("social.comment.list", "tiktok", "fetch_post_comment_api_v1_tiktok_web_fetch_post_comment_get", "/api/v1/tiktok/web/fetch_post_comment", "tikhub.direct.v1", AuthPlacementBearer, map[string]string{"content_ref": "aweme_id"}, map[string]any{"cursor": 0, "count": 20}, commentListOutput),
			},
		},
		{
			OperationKey: "social.comment.replies.list", ContractVersion: "v1", Name: "评论回复", Category: "社交媒体",
			Description: "获取公开评论的回复列表。",
			InputSchema: objectInputSchema(map[string]any{
				"platform":    stringSchema([]any{"tiktok"}),
				"content_ref": stringSchema(nil),
				"comment_ref": stringSchema(nil),
			}, []any{"platform", "content_ref", "comment_ref"}),
			OutputSchema: commentListOutput,
			Bindings: []ProviderOperation{
				providerGET("social.comment.replies.list", "tiktok", "fetch_post_comment_reply_api_v1_tiktok_web_fetch_post_comment_reply_get", "/api/v1/tiktok/web/fetch_post_comment_reply", "tikhub.direct.v1", AuthPlacementBearer, map[string]string{"content_ref": "item_id", "comment_ref": "comment_id"}, map[string]any{"cursor": 0, "count": 20}, commentListOutput),
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
				providerGET("social.trend.list", "douyin", "fetch_hot_search_result_api_v1_douyin_web_fetch_hot_search_result_get", "/api/v1/douyin/web/fetch_hot_search_result", "tikhub.direct.v1", AuthPlacementBearer, nil, nil, trendOutput),
			},
		},
		{
			OperationKey: "commerce.product.search", ContractVersion: "v1", Name: "商品搜索", Category: "电商",
			Description: "按平台和关键词搜索公开商品。",
			InputSchema: objectInputSchema(map[string]any{
				"platform": stringSchema([]any{"taobao", "tiktok_shop", "xiaohongshu"}),
				"query":    stringSchema(nil),
			}, []any{"platform", "query"}),
			OutputSchema: productListOutput,
			Bindings: []ProviderOperation{
				providerGET("commerce.product.search", "taobao", "getApiTaobaoSearchItemListV1", "/api/taobao/search-item-list/v1", "justoneapi.direct.v1", AuthPlacementQuery, map[string]string{"query": "keyword"}, map[string]any{"page": 1}, productListOutput),
				providerGET("commerce.product.search", "tiktok_shop", "fetch_search_products_list_api_v1_tiktok_shop_web_fetch_search_products_list_get", "/api/v1/tiktok/shop/web/fetch_search_products_list", "tikhub.direct.v1", AuthPlacementBearer, map[string]string{"query": "search_word"}, map[string]any{"offset": 0}, productListOutput),
				providerGET("commerce.product.search", "xiaohongshu", "search_products_api_v1_xiaohongshu_app_v2_search_products_get", "/api/v1/xiaohongshu/app_v2/search_products", "tikhub.direct.v1", AuthPlacementBearer, map[string]string{"query": "keyword"}, map[string]any{"page": 1}, productListOutput),
			},
		},
		{
			OperationKey: "commerce.product.get", ContractVersion: "v1", Name: "商品详情", Category: "电商",
			Description: "获取公开商品的标准化详情。",
			InputSchema: objectInputSchema(map[string]any{
				"platform":    stringSchema([]any{"taobao", "tiktok_shop", "xiaohongshu"}),
				"product_ref": stringSchema(nil),
			}, []any{"platform", "product_ref"}),
			OutputSchema: productOutput,
			Bindings: []ProviderOperation{
				providerGET("commerce.product.get", "taobao", "getApiTaobaoGetItemDetailV3", "/api/taobao/get-item-detail/v3", "justoneapi.direct.v1", AuthPlacementQuery, map[string]string{"product_ref": "itemId"}, nil, productOutput),
				providerGET("commerce.product.get", "tiktok_shop", "fetch_product_detail_api_v1_tiktok_shop_web_fetch_product_detail_get", "/api/v1/tiktok/shop/web/fetch_product_detail", "tikhub.direct.v1", AuthPlacementBearer, map[string]string{"product_ref": "product_id"}, nil, productOutput),
				providerGET("commerce.product.get", "xiaohongshu", "get_product_detail_api_v1_xiaohongshu_app_v2_get_product_detail_get", "/api/v1/xiaohongshu/app_v2/get_product_detail", "tikhub.direct.v1", AuthPlacementBearer, map[string]string{"product_ref": "sku_id"}, nil, productOutput),
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
				providerGET("commerce.product.reviews.list", "tiktok_shop", "fetch_product_reviews_api_v1_tiktok_shop_web_fetch_product_reviews_get", "/api/v1/tiktok/shop/web/fetch_product_reviews", "tikhub.direct.v1", AuthPlacementBearer, map[string]string{"product_ref": "product_id"}, map[string]any{"page_start": 1, "page_size": 10}, reviewListOutput),
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
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{"type": "string"}, "type": map[string]any{"type": "string", "const": kind},
			"platform": map[string]any{"type": "string"}, "url": map[string]any{"type": "string"},
		},
		"required": []any{"id", "platform"}, "additionalProperties": true,
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
