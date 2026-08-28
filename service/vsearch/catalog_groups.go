package vsearch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/QuantumNous/new-api/model"
)

type PublicCatalogGroup struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Category        string `json:"category"`
	Description     string `json:"description"`
	Status          string `json:"status"`
	Enabled         bool   `json:"enabled"`
	InterfaceCount  int64  `json:"interface_count"`
	CostLabel       string `json:"cost_label"`
	PriceMinMicros  int64  `json:"price_min_micros"`
	PriceMaxMicros  int64  `json:"price_max_micros"`
	RecentLatencyMs *int64 `json:"recent_latency_ms,omitempty"`
	LastSyncedAt    int64  `json:"last_synced_at"`
}

type catalogGroupAccumulator struct {
	key          string
	label        string
	categories   map[string]int
	descriptions map[string]int
	visibleTools map[string]struct{}
	callable     map[string]struct{}
	prices       []int64
	lastSyncedAt int64
}

type catalogGroupMetadata struct {
	label       string
	category    string
	description string
}

var catalogProviderAliases = map[string]string{
	"alpha-vantage": "alphavantage",
	"booking-com":   "booking",
	"brave":         "brave-search",
	"yfinance":      "yahoo",
	"yahoo-finance": "yahoo",
}

var catalogBrokerProviders = map[string]struct{}{
	"justoneapi": {},
	"tikhub":     {},
	"v2":         {},
}

var catalogGroupMetadataByKey = map[string]catalogGroupMetadata{
	"brave-search":     {label: "Brave Search", category: "搜索", description: "提供网页、新闻和图片等搜索能力。"},
	"alphavantage":     {label: "Alpha Vantage", category: "金融", description: "提供股票、外汇和市场数据能力。"},
	"yahoo":            {label: "Yahoo Finance", category: "金融", description: "提供公开金融市场与公司行情数据。"},
	"firecrawl":        {label: "Firecrawl", category: "抓取", description: "提供网页抓取、提取和结构化内容能力。"},
	"jina":             {label: "Jina", category: "抓取", description: "提供网页读取和内容提取能力。"},
	"booking":          {label: "Booking.com", category: "旅行", description: "提供旅行目的地、住宿和景点数据能力。"},
	"wechat":           {label: "微信", category: "社交媒体", description: "提供微信公众号等公开内容数据能力。"},
	"youtube":          {label: "YouTube", category: "社交媒体", description: "提供公开视频与互动数据能力。"},
	"reddit":           {label: "Reddit", category: "社交媒体", description: "提供公开社区帖子与评论数据能力。"},
	"douyin":           {label: "抖音", category: "社交媒体", description: "提供抖音公开内容数据能力。"},
	"douyin-ecommerce": {label: "抖音电商", category: "电商", description: "提供抖音商品和电商公开数据能力。"},
	"tiktok":           {label: "TikTok", category: "社交媒体", description: "提供 TikTok 公开内容数据能力。"},
	"tiktok-ecommerce": {label: "TikTok 电商", category: "电商", description: "提供 TikTok 商品和电商公开数据能力。"},
	"xiaohongshu":      {label: "小红书", category: "社交媒体", description: "提供小红书公开内容数据能力。"},
	"instagram":        {label: "Instagram", category: "社交媒体", description: "提供 Instagram 公开内容数据能力。"},
	"twitter":          {label: "X / Twitter", category: "社交媒体", description: "提供 X / Twitter 公开内容数据能力。"},
	"linkedin":         {label: "LinkedIn", category: "企业工商", description: "提供 LinkedIn 公开职业与企业数据能力。"},
	"amazon":           {label: "Amazon", category: "电商", description: "提供 Amazon 商品与评价数据能力。"},
	"taobao":           {label: "淘宝", category: "电商", description: "提供淘宝公开商品数据能力。"},
	"jd":               {label: "京东", category: "电商", description: "提供京东公开商品数据能力。"},
	"liepin":           {label: "猎聘", category: "招聘", description: "提供公开职位与招聘数据能力。"},
}

var catalogBrokerPlatformPatterns = []struct {
	key      string
	patterns []string
}{
	{key: "douyin-ecommerce", patterns: []string{"douyinec", "douyinecommerce", "douyinshop"}},
	{key: "tiktok-ecommerce", patterns: []string{"tiktokec", "tiktokecommerce", "tiktokshop"}},
	{key: "xiaohongshu", patterns: []string{"xiaohongshu", "redbook"}},
	{key: "wechat", patterns: []string{"wechat", "weixin"}},
	{key: "youtube", patterns: []string{"youtube"}},
	{key: "reddit", patterns: []string{"reddit"}},
	{key: "bilibili", patterns: []string{"bilibili"}},
	{key: "douyin", patterns: []string{"douyin"}},
	{key: "tiktok", patterns: []string{"tiktok"}},
	{key: "instagram", patterns: []string{"instagram"}},
	{key: "twitter", patterns: []string{"twitter"}},
	{key: "linkedin", patterns: []string{"linkedin"}},
	{key: "zhihu", patterns: []string{"zhihu"}},
	{key: "weibo", patterns: []string{"weibo"}},
	{key: "kuaishou", patterns: []string{"kuaishou"}},
	{key: "lemon8", patterns: []string{"lemon8"}},
	{key: "pipixia", patterns: []string{"pipixia"}},
	{key: "toutiao", patterns: []string{"toutiao"}},
	{key: "xigua", patterns: []string{"xigua"}},
	{key: "douban", patterns: []string{"douban"}},
	{key: "youku", patterns: []string{"youku"}},
	{key: "facebook", patterns: []string{"facebook"}},
	{key: "imdb", patterns: []string{"imdb"}},
	{key: "amazon", patterns: []string{"amazon"}},
	{key: "taobao", patterns: []string{"taobao"}},
	{key: "jd", patterns: []string{"jdapi"}},
	{key: "1688", patterns: []string{"1688"}},
	{key: "dewu", patterns: []string{"dewu"}},
	{key: "xianyu", patterns: []string{"xianyu"}},
	{key: "booking", patterns: []string{"booking"}},
	{key: "liepin", patterns: []string{"liepin"}},
}

func (runtime *ExecutionRuntime) ListCatalogGroups(ctx context.Context, principal Principal) ([]PublicCatalogGroup, error) {
	catalog, err := loadCatalogSnapshot(principal, false)
	if err != nil {
		return nil, err
	}
	groups := make(map[string]*catalogGroupAccumulator)
	for _, entry := range catalog {
		capability := entry.capability
		bindings := entry.bindings
		item := entry.public
		groupKey, label, toolIdentity := catalogGroupIdentity(capability, bindings)
		group := groups[groupKey]
		if group == nil {
			group = &catalogGroupAccumulator{
				key: groupKey, label: label, categories: make(map[string]int), descriptions: make(map[string]int),
				visibleTools: make(map[string]struct{}), callable: make(map[string]struct{}), prices: make([]int64, 0),
			}
			groups[groupKey] = group
		}
		group.visibleTools[toolIdentity] = struct{}{}
		if item.Enabled {
			group.callable[toolIdentity] = struct{}{}
		}
		group.categories[item.Category]++
		if description := strings.TrimSpace(item.Description); description != "" {
			group.descriptions[description]++
		}
		group.prices = append(group.prices, item.PriceMicros)
		if item.LastSyncedAt > group.lastSyncedAt {
			group.lastSyncedAt = item.LastSyncedAt
		}
	}

	result := make([]PublicCatalogGroup, 0, len(groups))
	for _, group := range groups {
		metadata := catalogGroupMetadataByKey[strings.TrimPrefix(group.key, "provider:")]
		name := metadata.label
		if name == "" {
			name = group.label
		}
		if name == "" {
			name = "vSearch 数据能力"
		}
		category := metadata.category
		if category == "" {
			category = stableCatalogMode(group.categories)
		}
		if category == "" {
			category = "搜索"
		}
		description := metadata.description
		if description == "" && len(group.descriptions) == 1 {
			for value := range group.descriptions {
				description = value
			}
		}
		if description == "" {
			description = fmt.Sprintf("%s 提供%s相关能力，可由 vSearch 按需调用。", name, category)
		}
		minPrice, maxPrice := catalogPriceRange(group.prices)
		available := len(group.callable) > 0
		status := "unavailable"
		if available {
			status = "available"
		}
		result = append(result, PublicCatalogGroup{
			ID: catalogGroupPublicID(group.key), Name: name, Category: category, Description: description,
			Status: status, Enabled: available, InterfaceCount: int64(len(group.callable)),
			CostLabel: catalogPriceLabel(minPrice, maxPrice), PriceMinMicros: minPrice, PriceMaxMicros: maxPrice,
			LastSyncedAt: group.lastSyncedAt,
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Category == result[j].Category {
			return result[i].Name < result[j].Name
		}
		return result[i].Category < result[j].Category
	})
	return result, nil
}

func catalogGroupIdentity(capability *model.SearchCapability, bindings []*model.SearchCapabilityBinding) (string, string, string) {
	derived := ""
	label := ""
	toolIdentity := capability.PublicID
	for _, binding := range bindings {
		toolName := strings.TrimSpace(binding.ToolName)
		if toolName == "" {
			continue
		}
		toolIdentity = strings.ToLower(toolName)
		key, publicLabel := catalogGroupFromToolName(toolName)
		if key == "" || (derived != "" && derived != key) {
			return "capability:" + capability.PublicID, safeCatalogGroupLabel(capability.Name), capability.PublicID
		}
		derived = key
		label = publicLabel
	}
	if derived == "" {
		return "capability:" + capability.PublicID, safeCatalogGroupLabel(capability.Name), capability.PublicID
	}
	return derived, label, toolIdentity
}

func catalogGroupFromToolName(toolName string) (string, string) {
	parts := strings.Split(strings.TrimSpace(toolName), "/")
	if len(parts) < 2 {
		return "", ""
	}
	provider := catalogSlug(parts[0])
	if alias := catalogProviderAliases[provider]; alias != "" {
		provider = alias
	}
	if provider == "" {
		return "", ""
	}
	if _, broker := catalogBrokerProviders[provider]; broker {
		normalized := catalogCompactIdentifier(toolName)
		for _, candidate := range catalogBrokerPlatformPatterns {
			for _, pattern := range candidate.patterns {
				if strings.Contains(normalized, pattern) {
					metadata := catalogGroupMetadataByKey[candidate.key]
					return "provider:" + candidate.key, metadata.label
				}
			}
		}
		return "", ""
	}
	metadata := catalogGroupMetadataByKey[provider]
	if metadata.label != "" {
		return "provider:" + provider, metadata.label
	}
	return "provider:" + provider, catalogProviderLabel(parts[0])
}

func catalogSlug(value string) string {
	var builder strings.Builder
	previousDash := false
	for _, character := range strings.ToLower(strings.TrimSpace(value)) {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') {
			builder.WriteRune(character)
			previousDash = false
		} else if !previousDash {
			builder.WriteByte('-')
			previousDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func catalogCompactIdentifier(value string) string {
	var builder strings.Builder
	for _, character := range strings.ToLower(value) {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') {
			builder.WriteRune(character)
		}
	}
	return builder.String()
}

func catalogProviderLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len([]rune(value)) > 48 {
		return "vSearch 数据能力"
	}
	for _, character := range value {
		if !unicode.IsLetter(character) && !unicode.IsDigit(character) && character != ' ' && character != '-' && character != '.' {
			return "vSearch 数据能力"
		}
	}
	return value
}

func safeCatalogGroupLabel(value string) string {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	if value == "" || len([]rune(value)) > 64 || strings.ContainsAny(value, "/_") || strings.Contains(lower, "agentkey") {
		return "vSearch 数据能力"
	}
	return value
}

func catalogGroupPublicID(key string) string {
	digest := sha256.Sum256([]byte("vsearch-catalog-group:v1:" + key))
	return "vr_grp_" + hex.EncodeToString(digest[:8])
}

func stableCatalogMode(values map[string]int) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	selected := ""
	selectedCount := -1
	for _, key := range keys {
		if values[key] > selectedCount {
			selected = key
			selectedCount = values[key]
		}
	}
	return selected
}

func catalogPriceRange(prices []int64) (int64, int64) {
	if len(prices) == 0 {
		return 0, 0
	}
	minPrice := prices[0]
	maxPrice := prices[0]
	for _, price := range prices[1:] {
		if price < minPrice {
			minPrice = price
		}
		if price > maxPrice {
			maxPrice = price
		}
	}
	return minPrice, maxPrice
}

func catalogPriceLabel(minPrice, maxPrice int64) string {
	if minPrice == maxPrice {
		return formatMicros(minPrice)
	}
	return fmt.Sprintf("¥%.4f–¥%.4f / 次", float64(minPrice)/1_000_000, float64(maxPrice)/1_000_000)
}
