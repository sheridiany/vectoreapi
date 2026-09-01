package vsearch

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

var tikHubGenericIDAliases = []string{
	"comment_id", "review_id", "tweet_id", "status_id", "post_id", "aweme_id", "photo_id", "photoId",
	"note_id", "media_id", "object_id", "objectId", "answer_id", "question_id", "pin_id", "video_id",
	"bvid", "bv_id", "aid", "id_str", "idstr", "rest_id", "job_id", "urn", "code", "param", "id", "uid", "user_id", "pk", "user_name", "username",
}

func normalizeTikHubGeneric(payload any, operation ProviderOperation) (map[string]any, error) {
	kind, list := tikHubGenericKind(operation.OperationKey)
	if kind == "" {
		return nil, tikHubContractMismatch()
	}
	if !list {
		candidate, score := tikHubBestEntity(payload, kind)
		if score == 0 {
			return nil, tikHubContractMismatch()
		}
		entity, ok := normalizeTikHubGenericEntity(candidate, operation.Platform, kind, 0)
		if !ok {
			return nil, tikHubContractMismatch()
		}
		return entity, nil
	}

	items, score := tikHubBestEntityArray(payload, kind)
	if score == 0 {
		return nil, tikHubContractMismatch()
	}
	result := make([]any, 0, len(items))
	for index, raw := range items {
		candidate, candidateScore := tikHubBestEntity(raw, kind)
		if candidateScore == 0 {
			continue
		}
		entity, ok := normalizeTikHubGenericEntity(candidate, operation.Platform, kind, index)
		if ok {
			result = append(result, entity)
		}
	}
	if len(result) == 0 && len(items) > 0 {
		return nil, tikHubContractMismatch()
	}
	cursor := tikHubFindFirstNamedValue(payload, "next_cursor", "nextCursor", "cursor", "pcursor", "next_offset", "continuation_token", "last_buffer")
	hasMore := tikHubFindFirstNamedValue(payload, "has_more", "hasMore", "more", "is_end")
	if name, value, exists := tikHubFindFirstNamedEntry(payload, "is_end"); exists && name == "is_end" {
		hasMore = !tikHubTruthy(value)
	}
	return tikHubList(result, cursor, hasMore), nil
}

func tikHubGenericKind(operationKey string) (string, bool) {
	switch operationKey {
	case "social.account.get":
		return "account", false
	case "social.account.contents.list", "social.content.search":
		return "content", true
	case "social.content.get":
		return "content", false
	case "social.comment.list", "social.comment.replies.list":
		return "comment", true
	case "social.trend.list":
		return "trend", true
	case "commerce.product.search":
		return "product", true
	case "commerce.product.get":
		return "product", false
	case "commerce.product.reviews.list":
		return "review", true
	case "job.search":
		return "job", true
	case "job.get":
		return "job", false
	default:
		return "", false
	}
}

func tikHubBestEntityArray(value any, kind string) ([]any, int) {
	bestScore := 0
	var best []any
	var walk func(any, int)
	walk = func(current any, depth int) {
		if depth > 12 {
			return
		}
		switch typed := current.(type) {
		case []any:
			score := 0
			limit := len(typed)
			if limit > 8 {
				limit = 8
			}
			for _, item := range typed[:limit] {
				_, itemScore := tikHubBestEntity(item, kind)
				score += itemScore
			}
			if limit > 0 {
				score = score*10/limit + min(limit, 5)
			}
			if score > bestScore {
				bestScore, best = score, typed
			}
			for _, item := range typed {
				walk(item, depth+1)
			}
		case map[string]any:
			for _, nested := range typed {
				walk(nested, depth+1)
			}
		}
	}
	walk(value, 0)
	return best, bestScore
}

func tikHubBestEntity(value any, kind string) (map[string]any, int) {
	bestScore := 0
	var best map[string]any
	var walk func(any, int)
	walk = func(current any, depth int) {
		if depth > 10 {
			return
		}
		switch typed := current.(type) {
		case map[string]any:
			score := tikHubEntityScore(typed, kind)
			if score > bestScore {
				bestScore, best = score, typed
			}
			for _, nested := range typed {
				walk(nested, depth+1)
			}
		case []any:
			for _, nested := range typed {
				walk(nested, depth+1)
			}
		}
	}
	walk(value, 0)
	return best, bestScore
}

func tikHubEntityScore(source map[string]any, kind string) int {
	score := 0
	if _, ok := tikHubString(tikHubFirstValue(source, tikHubGenericIDAliases...)); ok {
		score += 4
	}
	switch kind {
	case "account":
		if tikHubHasString(source, "nickname", "nick_name", "name", "display_name", "screen_name", "user_name", "username", "unique_id", "full_name") {
			score += 6
		}
		if tikHubHasAny(source, "follower_count", "followers_count", "fans_count", "avatar", "profile_image_url") {
			score += 2
		}
	case "content", "job", "product":
		if tikHubHasString(source, "title", "job_title", "product_name", "desc", "description", "job_description", "text", "full_text", "content", "caption", "postTitle", "name") {
			score += 6
		}
		if tikHubHasAny(source, "author", "user", "owner", "authorInfo", "creator", "channel") {
			score += 2
		}
	case "comment", "review":
		if tikHubHasString(source, "content", "text", "comment", "comment_text", "review_text", "message") {
			score += 7
		}
	case "trend":
		if tikHubHasString(source, "word", "keyword", "title", "name", "query", "rawQuery", "displayQuery", "search_word", "trendingSearchWord", "display_name") {
			score += 8
		}
	}
	return score
}

func normalizeTikHubGenericEntity(source map[string]any, platform, kind string, index int) (map[string]any, bool) {
	id, hasID := tikHubString(tikHubFirstValue(source, tikHubGenericIDAliases...))
	title, hasTitle := tikHubFirstString(source, "title", "job_title", "product_name", "postTitle", "name", "word", "keyword", "query", "rawQuery", "displayQuery", "search_word", "trendingSearchWord", "display_name")
	text, hasText := tikHubFirstString(source, "full_text", "text", "content", "desc", "description", "job_description", "caption", "comment_text", "review_text", "message")
	if !hasID {
		seed := title + "\x00" + text
		if seed == "\x00" {
			return nil, false
		}
		digest := sha256.Sum256([]byte(platform + "\x00" + kind + "\x00" + seed))
		id = hex.EncodeToString(digest[:12])
	}
	result := map[string]any{"id": id, "type": kind, "platform": platform}
	if kind == "trend" {
		if !hasTitle && hasText {
			title, hasTitle = text, true
		}
		if !hasTitle {
			return nil, false
		}
		result["title"] = title
		result["rank"] = index + 1
		if score, ok := tikHubNumber(tikHubFirstValue(source, "hot_value", "score", "trend_score", "view_count")); ok {
			result["score"] = score
		}
		return result, true
	}
	if kind == "account" {
		if displayName, ok := tikHubFirstString(source, "display_name", "nickname", "nick_name", "name", "screen_name", "user_name", "username", "unique_id", "full_name"); ok {
			result["display_name"] = displayName
		}
		if username, ok := tikHubFirstString(source, "username", "user_name", "screen_name", "unique_id", "url_token"); ok {
			result["username"] = username
		}
		if bio, ok := tikHubFirstString(source, "bio", "signature", "description", "profile_bio"); ok {
			result["bio"] = bio
		}
		if avatar := tikHubFirstMediaURL(tikHubFirstValue(source, "avatar", "avatar_url", "profile_image_url", "profile_pic_url", "headurl", "head_url")); avatar != "" {
			result["avatar_url"] = avatar
		}
		copyTikHubGenericCounts(result, source)
		return result, true
	}
	if hasTitle {
		result["title"] = title
	}
	if hasText {
		result["text"] = strings.TrimSpace(text)
	}
	if rawURL, ok := tikHubFirstString(source, "url", "job_url", "permalink", "share_url", "video_url", "post_url", "web_url", "uri", "scheme"); ok {
		result["url"] = rawURL
	}
	if publishedAt, ok := tikHubTime(source, "published_at", "created_at", "create_time", "timestamp", "time", "date", "createdAt"); ok {
		result["published_at"] = publishedAt
	}
	if author := tikHubGenericAuthor(source); len(author) > 0 {
		result["author"] = author
	}
	if media := tikHubMedia(source, "images", "image_list", "images_list", "cover", "cover_url", "thumbnail", "thumbnails", "media"); len(media) > 0 {
		result["media"] = media
	}
	if kind == "comment" || kind == "review" {
		if count, ok := tikHubCount(tikHubFirstValue(source, "like_count", "liked_count", "score")); ok {
			result["like_count"] = count
		}
		if count, ok := tikHubCount(tikHubFirstValue(source, "reply_count", "sub_comment_count", "child_comment_count")); ok {
			result["reply_count"] = count
		}
	} else {
		metrics := map[string]any{}
		for output, aliases := range map[string][]string{
			"view_count": {"view_count", "play_count", "playCount"}, "like_count": {"like_count", "liked_count", "digg_count", "attitudes_count"},
			"comment_count": {"comment_count", "comments_count"}, "share_count": {"share_count", "shared_count", "reposts_count"},
		} {
			if count, ok := tikHubCount(tikHubFirstValue(source, aliases...)); ok {
				metrics[output] = count
			}
		}
		if len(metrics) > 0 {
			result["metrics"] = metrics
		}
	}
	if kind == "product" && !hasTitle {
		return nil, false
	}
	return result, true
}

func tikHubGenericAuthor(source map[string]any) map[string]any {
	for _, name := range []string{"author", "user", "owner", "authorInfo", "creator", "channel", "user_info", "profile"} {
		if nested, ok := tikHubMap(source[name]); ok {
			return normalizeTikHubGenericIdentity(nested)
		}
	}
	return nil
}

func normalizeTikHubGenericIdentity(source map[string]any) map[string]any {
	result := map[string]any{}
	if id, ok := tikHubString(tikHubFirstValue(source, tikHubGenericIDAliases...)); ok {
		result["id"] = id
	}
	if name, ok := tikHubFirstString(source, "display_name", "nickname", "nick_name", "name", "screen_name", "user_name", "username", "unique_id", "full_name"); ok {
		result["display_name"] = name
	}
	if avatar := tikHubFirstMediaURL(tikHubFirstValue(source, "avatar", "avatar_url", "profile_image_url", "profile_pic_url", "headurl", "head_url")); avatar != "" {
		result["avatar_url"] = avatar
	}
	return result
}

func copyTikHubGenericCounts(target, source map[string]any) {
	for output, aliases := range map[string][]string{
		"follower_count":  {"follower_count", "followers_count", "fans_count", "fans"},
		"following_count": {"following_count", "friends_count", "follow_count"},
		"content_count":   {"content_count", "statuses_count", "post_count", "video_count", "aweme_count"},
	} {
		if count, ok := tikHubCount(tikHubFirstValue(source, aliases...)); ok {
			target[output] = count
		}
	}
}

func tikHubHasAny(source map[string]any, names ...string) bool {
	for _, name := range names {
		if value, exists := source[name]; exists && value != nil {
			return true
		}
	}
	return false
}

func tikHubHasString(source map[string]any, names ...string) bool {
	_, ok := tikHubFirstString(source, names...)
	return ok
}

func tikHubFirstString(source map[string]any, names ...string) (string, bool) {
	for _, name := range names {
		if value, ok := tikHubString(source[name]); ok {
			return value, true
		}
		if nested, ok := tikHubMap(source[name]); ok {
			if value, valid := tikHubFirstString(nested, "text", "title", "name"); valid {
				return value, true
			}
		}
	}
	return "", false
}

func tikHubNumber(value any) (float64, bool) {
	if count, ok := tikHubCount(value); ok {
		return float64(count), true
	}
	return 0, false
}

func tikHubFindFirstNamedValue(value any, names ...string) any {
	_, result, _ := tikHubFindFirstNamedEntry(value, names...)
	return result
}

func tikHubFindFirstNamedEntry(value any, names ...string) (string, any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		for _, name := range names {
			if result, exists := typed[name]; exists && result != nil {
				return name, result, true
			}
		}
		for _, nested := range typed {
			if name, result, ok := tikHubFindFirstNamedEntry(nested, names...); ok {
				return name, result, true
			}
		}
	case []any:
		for _, nested := range typed {
			if name, result, ok := tikHubFindFirstNamedEntry(nested, names...); ok {
				return name, result, true
			}
		}
	}
	return "", nil, false
}
