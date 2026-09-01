package vsearch

import (
	"encoding/json"
	"html"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

func normalizeTikHubResult(data json.RawMessage, operation ProviderOperation) (map[string]any, error) {
	if operation.OperationKey == "social.trend.list" && operation.Platform == "douyin" {
		return normalizeTikHubTrendList(data, operation.Platform)
	}
	var payload any
	if err := common.Unmarshal(data, &payload); err != nil {
		return nil, tikHubContractMismatch()
	}
	switch operation.Platform {
	case "youtube":
		return normalizeTikHubYouTube(payload, operation.OperationKey)
	case "xiaohongshu":
		if operation.OperationKey == "commerce.product.search" {
			return normalizeTikHubXHSProductSearch(payload, operation)
		}
		return normalizeTikHubGeneric(payload, operation)
	case "wechat_mp":
		if operation.OperationKey == "social.content.search" || operation.OperationKey == "social.account.contents.list" {
			return normalizeTikHubWeChat(payload, operation.OperationKey)
		}
		return normalizeTikHubGeneric(payload, operation)
	case "tiktok_shop":
		return normalizeTikHubShop(payload, operation.OperationKey)
	default:
		return normalizeTikHubGeneric(payload, operation)
	}
}

func normalizeTikHubXHSProductSearch(payload any, operation ProviderOperation) (map[string]any, error) {
	root, ok := tikHubMap(payload)
	if !ok {
		return nil, tikHubContractMismatch()
	}
	data, ok := tikHubMap(root["data"])
	if !ok {
		return nil, tikHubContractMismatch()
	}
	module, ok := tikHubMap(data["module"])
	if !ok {
		return nil, tikHubContractMismatch()
	}
	entries, ok := tikHubArray(module["data"])
	if !ok {
		return nil, tikHubContractMismatch()
	}
	items := make([]any, 0, len(entries))
	for index, raw := range entries {
		entry, valid := tikHubMap(raw)
		if !valid {
			continue
		}
		content, valid := tikHubMap(entry["content"])
		if !valid {
			continue
		}
		entity, valid := normalizeTikHubGenericEntity(content, operation.Platform, "product", index)
		if valid {
			items = append(items, entity)
		}
	}
	if len(items) == 0 && len(entries) > 0 {
		return nil, tikHubContractMismatch()
	}
	return tikHubList(items, root["search_id"], root["next_page"]), nil
}

func normalizeTikHubWeChat(payload any, operationKey string) (map[string]any, error) {
	root, ok := tikHubMap(payload)
	if !ok {
		return nil, tikHubContractMismatch()
	}
	switch operationKey {
	case "social.content.search":
		items, ok := tikHubArray(root["items"])
		if !ok {
			return nil, tikHubContractMismatch()
		}
		return normalizeTikHubWeChatArticles(items, root["cursor"], root["continue_flag"])
	case "social.account.contents.list":
		items, ok := tikHubArray(root["articles"])
		if !ok {
			return nil, tikHubContractMismatch()
		}
		return normalizeTikHubWeChatArticles(items, root["next_offset"], !tikHubTruthy(root["is_end"]))
	default:
		return nil, tikHubContractMismatch()
	}
}

func normalizeTikHubWeChatArticles(items []any, cursorValue, hasMoreValue any) (map[string]any, error) {
	result := make([]any, 0, len(items))
	for _, raw := range items {
		source, ok := tikHubMap(raw)
		if !ok {
			return nil, tikHubContractMismatch()
		}
		id, ok := tikHubString(tikHubFirstValue(source, "docID", "app_msg_id"))
		if !ok {
			return nil, tikHubContractMismatch()
		}
		if index, valid := tikHubString(source["idx"]); valid {
			id += "-" + index
		}
		item := map[string]any{"id": id, "type": "content", "platform": "wechat_mp"}
		tikHubCopyCleanString(item, "title", source, "title")
		tikHubCopyCleanString(item, "text", source, "digest", "desc")
		tikHubCopyString(item, "url", source, "url", "doc_url")
		if publishedAt, valid := tikHubTime(source, "create_time", "timestamp", "date"); valid {
			item["published_at"] = publishedAt
		}
		if author, valid := tikHubMap(source["source"]); valid {
			identity := map[string]any{}
			tikHubCopyCleanString(identity, "display_name", author, "title")
			if len(identity) > 0 {
				item["author"] = identity
			}
		}
		if media := tikHubMedia(source, "cover", "thumbUrl", "covers"); len(media) > 0 {
			item["media"] = media
		}
		result = append(result, item)
	}
	return tikHubList(result, cursorValue, hasMoreValue), nil
}

func normalizeTikHubYouTube(payload any, operationKey string) (map[string]any, error) {
	root, ok := payload.(map[string]any)
	if !ok {
		return nil, tikHubContractMismatch()
	}
	switch operationKey {
	case "social.account.get":
		return normalizeTikHubYouTubeAccount(root)
	case "social.account.contents.list":
		items, _ := tikHubArray(root["videos"])
		channel, _ := tikHubMap(root["channel"])
		return normalizeTikHubYouTubeContents(items, channel, root["continuation_token"])
	case "social.content.get":
		return normalizeTikHubYouTubeContent(root)
	case "social.content.search":
		items, _ := tikHubArray(root["videos"])
		return normalizeTikHubYouTubeContents(items, nil, root["continuation_token"])
	case "social.comment.list", "social.comment.replies.list":
		items, _ := tikHubArray(root["comments"])
		return normalizeTikHubYouTubeComments(items, root["continuation_token"])
	default:
		return nil, tikHubContractMismatch()
	}
}

func normalizeTikHubYouTubeAccount(source map[string]any) (map[string]any, error) {
	id, ok := tikHubString(source["channel_id"])
	if !ok {
		return nil, tikHubContractMismatch()
	}
	result := map[string]any{
		"id": id, "type": "account", "platform": "youtube",
		"url": "https://www.youtube.com/channel/" + id,
	}
	tikHubCopyString(result, "display_name", source, "title")
	tikHubCopyString(result, "bio", source, "description")
	if verified, valid := source["verified"].(bool); valid {
		result["verified"] = verified
	}
	if count, valid := tikHubCount(source["subscriber_count"]); valid {
		result["follower_count"] = count
	}
	if count, valid := tikHubCount(source["video_count"]); valid {
		result["content_count"] = count
	}
	if avatar := tikHubFirstMediaURL(source["avatar"]); avatar != "" {
		result["avatar_url"] = avatar
	}
	return result, nil
}

func normalizeTikHubYouTubeContents(items []any, channel map[string]any, cursorValue any) (map[string]any, error) {
	result := make([]any, 0, len(items))
	for _, raw := range items {
		source, ok := tikHubMap(raw)
		if !ok {
			return nil, tikHubContractMismatch()
		}
		id, ok := tikHubString(source["video_id"])
		if !ok {
			return nil, tikHubContractMismatch()
		}
		item := map[string]any{"id": id, "type": "content", "platform": "youtube"}
		tikHubCopyString(item, "url", source, "url")
		if _, exists := item["url"]; !exists {
			item["url"] = "https://www.youtube.com/watch?v=" + id
		}
		tikHubCopyString(item, "title", source, "title")
		tikHubCopyString(item, "text", source, "description", "description_snippet")
		tikHubCopyString(item, "published_at", source, "published_time")
		item["author"] = normalizeTikHubYouTubeAuthor(source, channel)
		if media := tikHubMedia(source, "thumbnails", "thumbnail"); len(media) > 0 {
			item["media"] = media
		}
		if count, valid := tikHubCount(source["view_count"]); valid {
			item["metrics"] = map[string]any{"view_count": count}
		}
		result = append(result, item)
	}
	return tikHubList(result, cursorValue, nil), nil
}

func normalizeTikHubYouTubeContent(source map[string]any) (map[string]any, error) {
	id, ok := tikHubString(source["video_id"])
	if !ok {
		return nil, tikHubContractMismatch()
	}
	result := map[string]any{"id": id, "type": "content", "platform": "youtube"}
	tikHubCopyString(result, "url", source, "video_url", "url")
	if _, exists := result["url"]; !exists {
		result["url"] = "https://www.youtube.com/watch?v=" + id
	}
	tikHubCopyString(result, "title", source, "title")
	tikHubCopyString(result, "text", source, "description")
	tikHubCopyString(result, "published_at", source, "upload_date", "publish_date", "date_text")
	result["author"] = normalizeTikHubYouTubeAuthor(source, nil)
	if media := tikHubMedia(source, "thumbnails", "thumbnail_url"); len(media) > 0 {
		result["media"] = media
	}
	metrics := map[string]any{}
	for outputName, sourceName := range map[string]string{
		"view_count": "view_count", "like_count": "like_count", "comment_count": "comment_count",
	} {
		if count, valid := tikHubCount(source[sourceName]); valid {
			metrics[outputName] = count
		}
	}
	if len(metrics) > 0 {
		result["metrics"] = metrics
	}
	return result, nil
}

func normalizeTikHubYouTubeComments(items []any, cursorValue any) (map[string]any, error) {
	result := make([]any, 0, len(items))
	for _, raw := range items {
		source, ok := tikHubMap(raw)
		if !ok {
			return nil, tikHubContractMismatch()
		}
		id, ok := tikHubString(source["comment_id"])
		if !ok {
			return nil, tikHubContractMismatch()
		}
		item := map[string]any{"id": id, "type": "comment", "platform": "youtube"}
		tikHubCopyString(item, "text", source, "content")
		tikHubCopyString(item, "published_at", source, "published_time")
		tikHubCopyString(item, "reply_cursor", source, "reply_continuation_token")
		if author, valid := tikHubMap(source["author"]); valid {
			item["author"] = normalizeTikHubPublicIdentity(author)
		}
		if count, valid := tikHubCount(source["like_count"]); valid {
			item["like_count"] = count
		}
		if count, valid := tikHubCount(source["reply_count"]); valid {
			item["reply_count"] = count
		}
		result = append(result, item)
	}
	return tikHubList(result, cursorValue, nil), nil
}

func normalizeTikHubShop(payload any, operationKey string) (map[string]any, error) {
	root, ok := tikHubMap(payload)
	if !ok {
		return nil, tikHubContractMismatch()
	}
	for depth := 0; depth < 3; depth++ {
		nested, exists := root["data"]
		if !exists {
			break
		}
		data, valid := tikHubMap(nested)
		if !valid {
			break
		}
		root = data
	}
	switch operationKey {
	case "commerce.product.search":
		items, _ := tikHubFindArray(root, "products")
		result, err := normalizeTikHubProducts(items)
		if err != nil {
			return nil, err
		}
		cursor := tikHubNestedValue(root, "load_more_params", "page_token")
		return tikHubList(result, cursor, root["has_more"]), nil
	case "commerce.product.get":
		product, valid := tikHubFindProduct(root)
		if !valid {
			return nil, tikHubContractMismatch()
		}
		return normalizeTikHubProduct(product)
	case "commerce.product.reviews.list":
		items, _ := tikHubFindArray(root, "product_reviews", "reviews", "review_list", "comments")
		result, err := normalizeTikHubReviews(items)
		if err != nil {
			return nil, err
		}
		cursor := tikHubFirstValue(root, "next_page_token", "page_token", "cursor")
		return tikHubList(result, cursor, root["has_more"]), nil
	default:
		return nil, tikHubContractMismatch()
	}
}

func normalizeTikHubProducts(items []any) ([]any, error) {
	result := make([]any, 0, len(items))
	for _, raw := range items {
		source, ok := tikHubMap(raw)
		if !ok {
			return nil, tikHubContractMismatch()
		}
		item, err := normalizeTikHubProduct(source)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func normalizeTikHubProduct(source map[string]any) (map[string]any, error) {
	idValue := tikHubFirstValue(source, "product_id", "productId", "sku_id")
	id, ok := tikHubString(idValue)
	if !ok {
		return nil, tikHubContractMismatch()
	}
	result := map[string]any{"id": id, "type": "product", "platform": "tiktok_shop"}
	tikHubCopyString(result, "title", source, "title", "product_name", "name")
	if description, valid := tikHubProductDescription(tikHubFirstValue(source, "description", "product_description")); valid {
		result["text"] = description
	}
	tikHubCopyString(result, "url", source, "seo_url", "product_url", "url")
	if seller, valid := tikHubMap(tikHubFirstValue(source, "seller_info", "seller")); valid {
		result["author"] = normalizeTikHubPublicIdentity(seller)
	}
	if media := tikHubMedia(source, "images", "image", "product_images"); len(media) > 0 {
		result["media"] = media
	}
	metrics := map[string]any{}
	for _, name := range []string{"rating", "review_count", "sold_count"} {
		if value, exists := source[name]; exists {
			metrics[name] = value
		}
	}
	for _, containerName := range []string{"rate_info", "sold_info"} {
		if container, valid := tikHubMap(source[containerName]); valid {
			for _, name := range []string{"rating", "review_count", "sold_count"} {
				if value, exists := container[name]; exists {
					metrics[name] = value
				}
			}
		}
	}
	if len(metrics) > 0 {
		result["metrics"] = metrics
	}
	if price, valid := tikHubMap(tikHubFirstValue(source, "product_price_info", "price_info", "price")); valid {
		result["price"] = price
	}
	return result, nil
}

func tikHubProductDescription(value any) (string, bool) {
	raw, ok := tikHubString(value)
	if !ok {
		return "", false
	}
	if !strings.HasPrefix(raw, "[") {
		return raw, true
	}
	var blocks []map[string]any
	if err := common.UnmarshalJsonStr(raw, &blocks); err != nil {
		return raw, true
	}
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if text, valid := tikHubString(block["text"]); valid {
			parts = append(parts, text)
		}
		if items, valid := tikHubArray(block["content"]); valid {
			for _, item := range items {
				if text, itemValid := tikHubString(item); itemValid {
					parts = append(parts, text)
				}
			}
		}
	}
	return strings.Join(parts, "\n"), len(parts) > 0
}

func normalizeTikHubReviews(items []any) ([]any, error) {
	result := make([]any, 0, len(items))
	for _, raw := range items {
		source, ok := tikHubMap(raw)
		if !ok {
			return nil, tikHubContractMismatch()
		}
		id, ok := tikHubString(tikHubFirstValue(source, "review_id", "comment_id", "id"))
		if !ok {
			return nil, tikHubContractMismatch()
		}
		item := map[string]any{"id": id, "type": "review", "platform": "tiktok_shop"}
		tikHubCopyString(item, "text", source, "content", "text", "review_text")
		if publishedAt, valid := tikHubTime(source, "review_time", "create_time", "created_at"); valid {
			item["published_at"] = publishedAt
		}
		if author, valid := tikHubMap(tikHubFirstValue(source, "author", "user_info", "user")); valid {
			item["author"] = normalizeTikHubPublicIdentity(author)
		} else if authorID, valid := tikHubString(source["reviewer_id"]); valid {
			author := map[string]any{"id": authorID}
			tikHubCopyString(author, "display_name", source, "reviewer_name")
			tikHubCopyString(author, "avatar_url", source, "reviewer_avatar_url")
			item["author"] = author
		}
		if count, valid := tikHubCount(source["like_count"]); valid {
			item["like_count"] = count
		}
		result = append(result, item)
	}
	return result, nil
}

func tikHubCopyCleanString(target map[string]any, targetName string, source map[string]any, sourceNames ...string) {
	for _, sourceName := range sourceNames {
		value, ok := tikHubString(source[sourceName])
		if !ok {
			continue
		}
		value = strings.ReplaceAll(value, `<em class="highlight">`, "")
		value = strings.ReplaceAll(value, "</em>", "")
		target[targetName] = html.UnescapeString(value)
		return
	}
}

func tikHubTime(source map[string]any, names ...string) (string, bool) {
	value := tikHubFirstValue(source, names...)
	if raw, ok := tikHubString(value); ok {
		if unixValue, err := strconv.ParseInt(raw, 10, 64); err == nil {
			if unixValue > 10_000_000_000 {
				unixValue /= 1_000
			}
			return time.Unix(unixValue, 0).UTC().Format(time.RFC3339), true
		}
		return raw, true
	}
	return "", false
}

func tikHubTruthy(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case float64:
		return typed != 0
	case string:
		return typed != "" && typed != "0" && !strings.EqualFold(typed, "false")
	default:
		return false
	}
}

func normalizeTikHubYouTubeAuthor(source, fallback map[string]any) map[string]any {
	author := map[string]any{}
	if fallback != nil {
		tikHubCopyString(author, "id", fallback, "id", "channel_id")
		tikHubCopyString(author, "display_name", fallback, "name", "title")
		tikHubCopyString(author, "url", fallback, "url", "channel_url")
	}
	tikHubCopyString(author, "id", source, "channel_id")
	tikHubCopyString(author, "display_name", source, "author")
	tikHubCopyString(author, "url", source, "channel_url", "owner_profile_url")
	return author
}

func normalizeTikHubPublicIdentity(source map[string]any) map[string]any {
	result := map[string]any{}
	tikHubCopyString(result, "id", source, "channel_id", "user_id", "seller_id", "id")
	tikHubCopyString(result, "display_name", source, "display_name", "shop_name", "nickname", "name")
	tikHubCopyString(result, "url", source, "channel_url", "url")
	tikHubCopyString(result, "avatar_url", source, "avatar_url", "avatar")
	return result
}

func tikHubList(items []any, cursorValue, hasMoreValue any) map[string]any {
	cursor, hasCursor := tikHubString(cursorValue)
	pageCursor := any(nil)
	if hasCursor {
		pageCursor = cursor
	}
	hasMore, validHasMore := hasMoreValue.(bool)
	if !validHasMore {
		hasMore = hasCursor
	}
	return map[string]any{
		"items": items,
		"page":  map[string]any{"cursor": pageCursor, "has_more": hasMore},
	}
}

func tikHubMap(value any) (map[string]any, bool) {
	result, ok := value.(map[string]any)
	return result, ok
}

func tikHubArray(value any) ([]any, bool) {
	result, ok := value.([]any)
	return result, ok
}

func tikHubString(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		return trimmed, trimmed != ""
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return "", false
		}
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	default:
		return "", false
	}
}

func tikHubCount(value any) (int64, bool) {
	if number, ok := value.(float64); ok {
		if number < 0 || math.IsNaN(number) || math.IsInf(number, 0) || number > float64(1<<63-1) {
			return 0, false
		}
		return int64(math.Round(number)), true
	}
	raw, ok := tikHubString(value)
	if !ok {
		return 0, false
	}
	raw = strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(raw, ",", ""), " ", ""))
	multiplier := float64(1)
	if len(raw) > 1 {
		switch raw[len(raw)-1] {
		case 'K':
			multiplier, raw = 1_000, raw[:len(raw)-1]
		case 'M':
			multiplier, raw = 1_000_000, raw[:len(raw)-1]
		case 'B':
			multiplier, raw = 1_000_000_000, raw[:len(raw)-1]
		}
	}
	number, err := strconv.ParseFloat(raw, 64)
	if err != nil || number < 0 || math.IsNaN(number) || math.IsInf(number, 0) || number > float64(1<<63-1)/multiplier {
		return 0, false
	}
	return int64(math.Round(number * multiplier)), true
}

func tikHubCopyString(target map[string]any, targetName string, source map[string]any, sourceNames ...string) {
	for _, sourceName := range sourceNames {
		if value, ok := tikHubString(source[sourceName]); ok {
			target[targetName] = value
			return
		}
	}
}

func tikHubFirstValue(source map[string]any, names ...string) any {
	for _, name := range names {
		if value, exists := source[name]; exists && value != nil {
			return value
		}
	}
	return nil
}

func tikHubNestedValue(source map[string]any, path ...string) any {
	current := source
	for index, name := range path {
		value, exists := current[name]
		if !exists {
			return nil
		}
		if index == len(path)-1 {
			return value
		}
		next, ok := tikHubMap(value)
		if !ok {
			return nil
		}
		current = next
	}
	return nil
}

func tikHubFindArray(source map[string]any, names ...string) ([]any, bool) {
	for _, name := range names {
		if value, ok := tikHubArray(source[name]); ok {
			return value, true
		}
	}
	for _, value := range source {
		if nested, ok := tikHubMap(value); ok {
			if result, found := tikHubFindArray(nested, names...); found {
				return result, true
			}
		}
	}
	return nil, false
}

func tikHubFindEntity(source map[string]any, idNames ...string) (map[string]any, bool) {
	for _, name := range idNames {
		if _, ok := tikHubString(source[name]); ok {
			return source, true
		}
	}
	for _, value := range source {
		if nested, ok := tikHubMap(value); ok {
			if result, found := tikHubFindEntity(nested, idNames...); found {
				return result, true
			}
			continue
		}
		if items, ok := tikHubArray(value); ok {
			for _, item := range items {
				if nested, valid := tikHubMap(item); valid {
					if result, found := tikHubFindEntity(nested, idNames...); found {
						return result, true
					}
				}
			}
		}
	}
	return nil, false
}

func tikHubFindProduct(source map[string]any) (map[string]any, bool) {
	if product, valid := tikHubMap(source["product_model"]); valid {
		if _, hasID := tikHubString(tikHubFirstValue(product, "product_id", "productId")); hasID {
			merged := make(map[string]any, len(product)+4)
			for name, value := range product {
				merged[name] = value
			}
			if seller, exists := tikHubMap(source["seller_model"]); exists {
				merged["seller_info"] = seller
			}
			if promotion, exists := tikHubMap(source["promotion_model"]); exists {
				if price, priceExists := tikHubMap(tikHubNestedValue(promotion, "promotion_product_price", "min_price")); priceExists {
					merged["product_price_info"] = price
				}
			}
			if reviews, exists := tikHubMap(source["review_model"]); exists {
				merged["rating"] = reviews["product_overall_score"]
				merged["review_count"] = reviews["product_review_count"]
			}
			return merged, true
		}
	}
	for _, value := range source {
		if nested, valid := tikHubMap(value); valid {
			if product, found := tikHubFindProduct(nested); found {
				return product, true
			}
			continue
		}
		if items, valid := tikHubArray(value); valid {
			for _, item := range items {
				if nested, itemValid := tikHubMap(item); itemValid {
					if product, found := tikHubFindProduct(nested); found {
						return product, true
					}
				}
			}
		}
	}
	for _, name := range []string{"product_id", "productId"} {
		if _, valid := tikHubString(source[name]); valid {
			return source, true
		}
	}
	return nil, false
}

func tikHubFirstMediaURL(value any) string {
	if raw, ok := tikHubString(value); ok {
		return raw
	}
	if object, ok := tikHubMap(value); ok {
		if raw, valid := tikHubString(object["url"]); valid {
			return raw
		}
		if raw := tikHubFirstMediaURL(object["url_list"]); raw != "" {
			return raw
		}
		if raw, valid := tikHubString(object["uri"]); valid {
			return raw
		}
	}
	if items, ok := tikHubArray(value); ok {
		for _, item := range items {
			if raw := tikHubFirstMediaURL(item); raw != "" {
				return raw
			}
		}
	}
	return ""
}

func tikHubMedia(source map[string]any, names ...string) []any {
	for _, name := range names {
		value, exists := source[name]
		if !exists {
			continue
		}
		if items, ok := tikHubArray(value); ok {
			result := make([]any, 0, len(items))
			for _, item := range items {
				if raw := tikHubFirstMediaURL(item); raw != "" {
					result = append(result, map[string]any{"url": raw})
				}
			}
			if len(result) > 0 {
				return result
			}
		}
		if raw := tikHubFirstMediaURL(value); raw != "" {
			return []any{map[string]any{"url": raw}}
		}
	}
	return nil
}

func tikHubContractMismatch() error {
	return newConnectorError("UPSTREAM_CONTRACT_MISMATCH", http.StatusBadGateway, "上游服务返回的数据不符合能力约定。")
}
