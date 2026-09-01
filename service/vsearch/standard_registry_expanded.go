package vsearch

// standardCapabilityRegistry is intentionally curated around read-only public
// data operations. Every binding in this registry has a canonical request and
// response contract; provider-specific pagination and optional filters remain
// private until they are represented by that contract.
func standardCapabilityRegistry() []standardCapabilityDefinition {
	accountOutput := entityOutputSchema("account")
	contentOutput := entityOutputSchema("content")
	contentListOutput := listOutputSchema("content")
	commentListOutput := listOutputSchema("comment")
	trendOutput := trendListOutputSchema()
	productOutput := entityOutputSchema("product")
	productListOutput := listOutputSchema("product")
	reviewListOutput := listOutputSchema("review")
	jobOutput := entityOutputSchema("job")
	jobListOutput := listOutputSchema("job")

	accountBindings := []ProviderOperation{
		verifiedProviderGET("social.account.get", "douyin", "fetch_user_profile_by_uid_api_v1_douyin_web_fetch_user_profile_by_uid_get", "/api/v1/douyin/web/fetch_user_profile_by_uid", map[string]string{"account_ref": "uid"}, nil, accountOutput, 1_000),
		verifiedProviderGET("social.account.get", "tiktok", "fetch_user_profile_api_v1_tiktok_web_fetch_user_profile_get", "/api/v1/tiktok/web/fetch_user_profile", map[string]string{"account_ref": "uniqueId"}, nil, accountOutput, 1_000),
		verifiedProviderGET("social.account.get", "xiaohongshu", "get_user_info_api_v1_xiaohongshu_app_v2_get_user_info_get", "/api/v1/xiaohongshu/app_v2/get_user_info", map[string]string{"account_ref": "user_id"}, nil, accountOutput, 1_000),
		verifiedProviderGET("social.account.get", "twitter", "fetch_user_profile_api_v1_twitter_web_fetch_user_profile_get", "/api/v1/twitter/web/fetch_user_profile", map[string]string{"account_ref": "screen_name"}, nil, accountOutput, 1_000),
		verifiedProviderGET("social.account.get", "weibo", "fetch_user_info_api_v1_weibo_app_fetch_user_info_get", "/api/v1/weibo/app/fetch_user_info", map[string]string{"account_ref": "uid"}, nil, accountOutput, 1_000),
		verifiedProviderGET("social.account.get", "reddit", "fetch_user_profile_api_v1_reddit_app_fetch_user_profile_get", "/api/v1/reddit/app/fetch_user_profile", map[string]string{"account_ref": "username"}, nil, accountOutput, 1_000),
		verifiedProviderGET("social.account.get", "linkedin", "get_user_profile_api_v1_linkedin_web_v2_get_user_profile_get", "/api/v1/linkedin/web_v2/get_user_profile", map[string]string{"account_ref": "url"}, nil, accountOutput, 1_000),
		verifiedProviderGET("social.account.get", "instagram", "get_user_profile_api_v1_instagram_v3_get_user_profile_get", "/api/v1/instagram/v3/get_user_profile", map[string]string{"account_ref": "username"}, nil, accountOutput, 1_000),
		verifiedProviderGET("social.account.get", "bilibili", "fetch_user_info_api_v1_bilibili_app_fetch_user_info_get", "/api/v1/bilibili/app/fetch_user_info", map[string]string{"account_ref": "user_id"}, nil, accountOutput, 1_000),
		verifiedProviderGET("social.account.get", "zhihu", "fetch_user_info_api_v1_zhihu_web_fetch_user_info_get", "/api/v1/zhihu/web/fetch_user_info", map[string]string{"account_ref": "user_url_token"}, nil, accountOutput, 1_000),
		verifiedProviderGET("social.account.get", "kuaishou", "fetch_one_user_v2_api_v1_kuaishou_app_fetch_one_user_v2_get", "/api/v1/kuaishou/app/fetch_one_user_v2", map[string]string{"account_ref": "user_id"}, nil, accountOutput, 1_000),
		verifiedProviderPOST("social.account.get", "wechat_mp", "fetch_account_profile_api_v1_wechat_mp_v2_fetch_account_profile_post", "/api/v1/wechat_mp/v2/fetch_account_profile", map[string]string{"account_ref": "username"}, map[string]any{"raw": false}, accountOutput, 10_000),
		verifiedProviderPOST("social.account.get", "wechat_channels", "fetch_user_profile_api_v1_wechat_channels_v2_fetch_user_profile_post", "/api/v1/wechat_channels/v2/fetch_user_profile", map[string]string{"account_ref": "username"}, map[string]any{"raw": false}, accountOutput, 10_000),
		verifiedProviderGET("social.account.get", "youtube", "get_channel_info_api_v1_youtube_web_get_channel_info_get", "/api/v1/youtube/web/get_channel_info", map[string]string{"account_ref": "channel_id"}, nil, accountOutput, 1_000),
	}

	contentsBindings := []ProviderOperation{
		verifiedProviderGET("social.account.contents.list", "douyin", "fetch_user_post_videos_api_v1_douyin_web_fetch_user_post_videos_get", "/api/v1/douyin/web/fetch_user_post_videos", map[string]string{"account_ref": "sec_user_id"}, map[string]any{"max_cursor": 0, "count": 20}, contentListOutput, 1_000),
		verifiedProviderGET("social.account.contents.list", "tiktok", "fetch_user_post_api_v1_tiktok_web_fetch_user_post_get", "/api/v1/tiktok/web/fetch_user_post", map[string]string{"account_ref": "secUid"}, map[string]any{"count": 20, "cursor": 0}, contentListOutput, 1_000),
		verifiedProviderGET("social.account.contents.list", "xiaohongshu", "get_user_posted_notes_api_v1_xiaohongshu_app_v2_get_user_posted_notes_get", "/api/v1/xiaohongshu/app_v2/get_user_posted_notes", map[string]string{"account_ref": "user_id"}, nil, contentListOutput, 1_000),
		verifiedProviderGET("social.account.contents.list", "twitter", "fetch_user_post_tweet_api_v1_twitter_web_fetch_user_post_tweet_get", "/api/v1/twitter/web/fetch_user_post_tweet", map[string]string{"account_ref": "rest_id"}, nil, contentListOutput, 1_000),
		verifiedProviderGET("social.account.contents.list", "weibo", "fetch_user_profile_feed_api_v1_weibo_app_fetch_user_profile_feed_get", "/api/v1/weibo/app/fetch_user_profile_feed", map[string]string{"account_ref": "uid"}, nil, contentListOutput, 1_000),
		verifiedProviderGET("social.account.contents.list", "reddit", "fetch_user_posts_api_v1_reddit_app_fetch_user_posts_get", "/api/v1/reddit/app/fetch_user_posts", map[string]string{"account_ref": "username"}, map[string]any{"need_format": true}, contentListOutput, 1_000),
		verifiedProviderGET("social.account.contents.list", "linkedin", "get_user_posts_api_v1_linkedin_web_v2_get_user_posts_get", "/api/v1/linkedin/web_v2/get_user_posts", map[string]string{"account_ref": "url"}, nil, contentListOutput, 1_000),
		verifiedProviderGET("social.account.contents.list", "instagram", "get_user_posts_api_v1_instagram_v3_get_user_posts_get", "/api/v1/instagram/v3/get_user_posts", map[string]string{"account_ref": "username"}, nil, contentListOutput, 1_000),
		verifiedProviderGET("social.account.contents.list", "bilibili", "fetch_user_post_videos_api_v1_bilibili_web_fetch_user_post_videos_get", "/api/v1/bilibili/web/fetch_user_post_videos", map[string]string{"account_ref": "uid"}, map[string]any{"pn": 1, "ps": 20, "order": "pubdate"}, contentListOutput, 1_000),
		verifiedProviderPOST("social.account.contents.list", "wechat_mp", "fetch_account_articles_api_v1_wechat_mp_v2_fetch_account_articles_post", "/api/v1/wechat_mp/v2/fetch_account_articles", map[string]string{"account_ref": "username"}, map[string]any{"raw": false}, contentListOutput, 10_000),
		verifiedProviderPOST("social.account.contents.list", "wechat_channels", "fetch_user_videos_api_v1_wechat_channels_v2_fetch_user_videos_post", "/api/v1/wechat_channels/v2/fetch_user_videos", map[string]string{"account_ref": "username"}, map[string]any{"raw": false}, contentListOutput, 10_000),
		verifiedProviderGET("social.account.contents.list", "youtube", "get_channel_videos_api_v1_youtube_web_v2_get_channel_videos_get", "/api/v1/youtube/web_v2/get_channel_videos", map[string]string{"account_ref": "channel_id"}, map[string]any{"need_format": true}, contentListOutput, 1_000),
	}

	contentBindings := []ProviderOperation{
		verifiedProviderPOSTQuery("social.content.get", "douyin", "fetch_item_base_api_v1_douyin_index_fetch_item_base_post", "/api/v1/douyin/index/fetch_item_base", map[string]string{"content_ref": "item_id"}, nil, contentOutput, 1_000),
		verifiedProviderGET("social.content.get", "tiktok", "fetch_one_video_v3_api_v1_tiktok_app_v3_fetch_one_video_v3_get", "/api/v1/tiktok/app/v3/fetch_one_video_v3", map[string]string{"content_ref": "aweme_id"}, map[string]any{"region": "US"}, contentOutput, 1_000),
		verifiedProviderGET("social.content.get", "xiaohongshu", "get_image_note_detail_api_v1_xiaohongshu_app_v2_get_image_note_detail_get", "/api/v1/xiaohongshu/app_v2/get_image_note_detail", map[string]string{"content_ref": "note_id"}, nil, contentOutput, 1_000),
		verifiedProviderGET("social.content.get", "twitter", "fetch_tweet_detail_api_v1_twitter_web_fetch_tweet_detail_get", "/api/v1/twitter/web/fetch_tweet_detail", map[string]string{"content_ref": "tweet_id"}, nil, contentOutput, 1_000),
		verifiedProviderGET("social.content.get", "weibo", "fetch_status_detail_api_v1_weibo_app_fetch_status_detail_get", "/api/v1/weibo/app/fetch_status_detail", map[string]string{"content_ref": "status_id"}, nil, contentOutput, 1_000),
		verifiedProviderGET("social.content.get", "reddit", "fetch_post_details_api_v1_reddit_app_fetch_post_details_get", "/api/v1/reddit/app/fetch_post_details", map[string]string{"content_ref": "post_id"}, map[string]any{"need_format": true}, contentOutput, 1_000),
		verifiedProviderGET("social.content.get", "instagram", "get_post_info_by_code_api_v1_instagram_v3_get_post_info_by_code_get", "/api/v1/instagram/v3/get_post_info_by_code", map[string]string{"content_ref": "code"}, nil, contentOutput, 1_000),
		verifiedProviderGET("social.content.get", "bilibili", "fetch_one_video_api_v1_bilibili_app_fetch_one_video_get", "/api/v1/bilibili/app/fetch_one_video", map[string]string{"content_ref": "av_id"}, nil, contentOutput, 1_000),
		verifiedProviderGET("social.content.get", "zhihu", "fetch_answer_detail_api_v1_zhihu_web_fetch_answer_detail_get", "/api/v1/zhihu/web/fetch_answer_detail", map[string]string{"content_ref": "answer_id"}, nil, contentOutput, 1_000),
		verifiedProviderGET("social.content.get", "kuaishou", "fetch_one_video_v1_api_v1_kuaishou_app_fetch_one_video_get", "/api/v1/kuaishou/app/fetch_one_video", map[string]string{"content_ref": "photo_id"}, nil, contentOutput, 1_000),
		verifiedProviderPOST("social.content.get", "wechat_channels", "fetch_video_detail_api_v1_wechat_channels_v2_fetch_video_detail_post", "/api/v1/wechat_channels/v2/fetch_video_detail", map[string]string{"content_ref": "object_id"}, map[string]any{"raw": false}, contentOutput, 10_000),
		verifiedProviderGET("social.content.get", "youtube", "get_video_info_v2_api_v1_youtube_web_v2_get_video_info_v2_get", "/api/v1/youtube/web_v2/get_video_info_v2", map[string]string{"content_ref": "video_id"}, map[string]any{"need_format": true}, contentOutput, 1_000),
	}

	searchBindings := []ProviderOperation{
		verifiedProviderPOSTQuery("social.content.search", "douyin", "fetch_item_query_api_v1_douyin_index_fetch_item_query_post", "/api/v1/douyin/index/fetch_item_query", map[string]string{"query": "query"}, nil, contentListOutput, 1_000),
		verifiedProviderGET("social.content.search", "tiktok", "fetch_video_search_result_api_v1_tiktok_app_v3_fetch_video_search_result_get", "/api/v1/tiktok/app/v3/fetch_video_search_result", map[string]string{"query": "keyword"}, nil, contentListOutput, 1_000),
		verifiedProviderGET("social.content.search", "xiaohongshu", "search_notes_api_v1_xiaohongshu_app_v2_search_notes_get", "/api/v1/xiaohongshu/app_v2/search_notes", map[string]string{"query": "keyword"}, map[string]any{"page": 1, "ai_mode": 0}, contentListOutput, 1_000),
		verifiedProviderGET("social.content.search", "twitter", "fetch_search_timeline_api_v1_twitter_web_fetch_search_timeline_get", "/api/v1/twitter/web/fetch_search_timeline", map[string]string{"query": "keyword"}, nil, contentListOutput, 1_000),
		verifiedProviderGET("social.content.search", "weibo", "fetch_search_all_api_v1_weibo_app_fetch_search_all_get", "/api/v1/weibo/app/fetch_search_all", map[string]string{"query": "query"}, nil, contentListOutput, 1_000),
		verifiedProviderGET("social.content.search", "reddit", "fetch_dynamic_search_api_v1_reddit_app_fetch_dynamic_search_get", "/api/v1/reddit/app/fetch_dynamic_search", map[string]string{"query": "query"}, nil, contentListOutput, 1_000),
		verifiedProviderGET("social.content.search", "instagram", "general_search_api_v1_instagram_v3_general_search_get", "/api/v1/instagram/v3/general_search", map[string]string{"query": "query"}, nil, contentListOutput, 1_000),
		verifiedProviderGET("social.content.search", "bilibili", "fetch_search_all_api_v1_bilibili_app_fetch_search_all_get", "/api/v1/bilibili/app/fetch_search_all", map[string]string{"query": "keyword"}, nil, contentListOutput, 1_000),
		verifiedProviderGET("social.content.search", "zhihu", "fetch_article_search_v3_api_v1_zhihu_web_fetch_article_search_v3_get", "/api/v1/zhihu/web/fetch_article_search_v3", map[string]string{"query": "keyword"}, nil, contentListOutput, 1_000),
		verifiedProviderGET("social.content.search", "kuaishou", "search_video_v2_api_v1_kuaishou_app_search_video_v2_get", "/api/v1/kuaishou/app/search_video_v2", map[string]string{"query": "keyword"}, nil, contentListOutput, 1_000),
		verifiedProviderGET("social.content.search", "youtube", "get_general_search_v2_api_v1_youtube_web_v2_get_general_search_v2_get", "/api/v1/youtube/web_v2/get_general_search_v2", map[string]string{"query": "keyword"}, map[string]any{"need_format": true, "type": "video"}, contentListOutput, 2_000),
	}

	commentBindings := []ProviderOperation{
		verifiedProviderGET("social.comment.list", "douyin", "fetch_video_comments_api_v1_douyin_web_fetch_video_comments_get", "/api/v1/douyin/web/fetch_video_comments", map[string]string{"content_ref": "aweme_id"}, map[string]any{"cursor": 0, "count": 20}, commentListOutput, 1_000),
		verifiedProviderGET("social.comment.list", "tiktok", "fetch_video_comments_api_v1_tiktok_app_v3_fetch_video_comments_get", "/api/v1/tiktok/app/v3/fetch_video_comments", map[string]string{"content_ref": "aweme_id"}, nil, commentListOutput, 1_000),
		verifiedProviderGET("social.comment.list", "xiaohongshu", "get_note_comments_api_v1_xiaohongshu_app_v2_get_note_comments_get", "/api/v1/xiaohongshu/app_v2/get_note_comments", map[string]string{"content_ref": "note_id"}, nil, commentListOutput, 1_000),
		verifiedProviderGET("social.comment.list", "twitter", "fetch_post_comments_api_v1_twitter_web_fetch_post_comments_get", "/api/v1/twitter/web/fetch_post_comments", map[string]string{"content_ref": "tweet_id"}, nil, commentListOutput, 1_000),
		verifiedProviderGET("social.comment.list", "weibo", "fetch_status_comments_api_v1_weibo_app_fetch_status_comments_get", "/api/v1/weibo/app/fetch_status_comments", map[string]string{"content_ref": "status_id"}, nil, commentListOutput, 1_000),
		verifiedProviderGET("social.comment.list", "reddit", "fetch_post_comments_api_v1_reddit_app_fetch_post_comments_get", "/api/v1/reddit/app/fetch_post_comments", map[string]string{"content_ref": "post_id"}, nil, commentListOutput, 1_000),
		verifiedProviderGET("social.comment.list", "linkedin", "get_post_comments_api_v1_linkedin_web_v2_get_post_comments_get", "/api/v1/linkedin/web_v2/get_post_comments", map[string]string{"content_ref": "urn"}, nil, commentListOutput, 1_000),
		verifiedProviderGET("social.comment.list", "instagram", "get_post_comments_api_v1_instagram_v3_get_post_comments_get", "/api/v1/instagram/v3/get_post_comments", map[string]string{"content_ref": "code"}, nil, commentListOutput, 1_000),
		verifiedProviderGET("social.comment.list", "bilibili", "fetch_collect_folders_api_v1_bilibili_web_fetch_video_comments_get", "/api/v1/bilibili/web/fetch_video_comments", map[string]string{"content_ref": "bv_id"}, map[string]any{"pn": 1}, commentListOutput, 1_000),
		verifiedProviderGET("social.comment.list", "zhihu", "fetch_comment_v5_api_v1_zhihu_web_fetch_comment_v5_get", "/api/v1/zhihu/web/fetch_comment_v5", map[string]string{"content_ref": "answer_id"}, map[string]any{"order_by": "score", "limit": 20, "offset": 0}, commentListOutput, 1_000),
		verifiedProviderGET("social.comment.list", "kuaishou", "fetch_video_comment_api_v1_kuaishou_app_fetch_video_comment_get", "/api/v1/kuaishou/app/fetch_video_comment", map[string]string{"content_ref": "photo_id"}, nil, commentListOutput, 1_000),
		verifiedProviderPOST("social.comment.list", "wechat_channels", "fetch_video_comments_api_v1_wechat_channels_v2_fetch_video_comments_post", "/api/v1/wechat_channels/v2/fetch_video_comments", map[string]string{"content_ref": "object_id"}, map[string]any{"raw": false}, commentListOutput, 10_000),
		verifiedProviderGET("social.comment.list", "youtube", "get_video_comments_api_v1_youtube_web_v2_get_video_comments_get", "/api/v1/youtube/web_v2/get_video_comments", map[string]string{"content_ref": "video_id"}, map[string]any{"need_format": true}, commentListOutput, 1_000),
	}

	replyBindings := []ProviderOperation{
		verifiedProviderGET("social.comment.replies.list", "douyin", "fetch_video_comments_reply_api_v1_douyin_web_fetch_video_comment_replies_get", "/api/v1/douyin/web/fetch_video_comment_replies", map[string]string{"content_ref": "item_id", "comment_ref": "comment_id"}, map[string]any{"cursor": 0, "count": 20}, commentListOutput, 1_000),
		verifiedProviderGET("social.comment.replies.list", "tiktok", "fetch_video_comments_reply_api_v1_tiktok_app_v3_fetch_video_comment_replies_get", "/api/v1/tiktok/app/v3/fetch_video_comment_replies", map[string]string{"content_ref": "item_id", "comment_ref": "comment_id"}, nil, commentListOutput, 1_000),
		verifiedProviderGET("social.comment.replies.list", "xiaohongshu", "get_note_sub_comments_api_v1_xiaohongshu_app_v2_get_note_sub_comments_get", "/api/v1/xiaohongshu/app_v2/get_note_sub_comments", map[string]string{"content_ref": "note_id", "comment_ref": "comment_id"}, nil, commentListOutput, 1_000),
		verifiedProviderGET("social.comment.replies.list", "weibo", "fetch_post_sub_comments_api_v1_weibo_web_v2_fetch_post_sub_comments_get", "/api/v1/weibo/web_v2/fetch_post_sub_comments", map[string]string{"comment_ref": "id"}, nil, commentListOutput, 1_000),
		verifiedProviderGET("social.comment.replies.list", "reddit", "fetch_comment_replies_api_v1_reddit_app_fetch_comment_replies_get", "/api/v1/reddit/app/fetch_comment_replies", map[string]string{"content_ref": "post_id", "comment_ref": "cursor"}, nil, commentListOutput, 1_000),
		verifiedProviderGET("social.comment.replies.list", "bilibili", "fetch_collect_folders_api_v1_bilibili_web_fetch_comment_reply_get", "/api/v1/bilibili/web/fetch_comment_reply", map[string]string{"content_ref": "bv_id", "comment_ref": "rpid"}, map[string]any{"pn": 1}, commentListOutput, 1_000),
		verifiedProviderGET("social.comment.replies.list", "zhihu", "fetch_sub_comment_v5_api_v1_zhihu_web_fetch_sub_comment_v5_get", "/api/v1/zhihu/web/fetch_sub_comment_v5", map[string]string{"comment_ref": "comment_id"}, map[string]any{"order_by": "score", "limit": 20, "offset": 0}, commentListOutput, 1_000),
		verifiedProviderGET("social.comment.replies.list", "youtube", "get_video_comment_replies_api_v1_youtube_web_v2_get_video_comment_replies_get", "/api/v1/youtube/web_v2/get_video_comment_replies", map[string]string{"comment_ref": "continuation_token"}, map[string]any{"need_format": true}, commentListOutput, 1_000),
	}

	trendBindings := []ProviderOperation{
		verifiedProviderGET("social.trend.list", "douyin", "fetch_hot_search_result_api_v1_douyin_web_fetch_hot_search_result_get", "/api/v1/douyin/web/fetch_hot_search_result", nil, nil, trendOutput, 1_000),
		verifiedProviderGET("social.trend.list", "tiktok", "fetch_trending_searchwords_api_v1_tiktok_web_fetch_trending_searchwords_get", "/api/v1/tiktok/web/fetch_trending_searchwords", nil, nil, trendOutput, 1_000),
		verifiedProviderGET("social.trend.list", "twitter", "fetch_trending_api_v1_twitter_web_fetch_trending_get", "/api/v1/twitter/web/fetch_trending", nil, nil, trendOutput, 1_000),
		verifiedProviderGET("social.trend.list", "weibo", "fetch_hot_search_api_v1_weibo_app_fetch_hot_search_get", "/api/v1/weibo/app/fetch_hot_search", nil, nil, trendOutput, 1_000),
		verifiedProviderGET("social.trend.list", "reddit", "fetch_trending_searches_api_v1_reddit_app_fetch_trending_searches_get", "/api/v1/reddit/app/fetch_trending_searches", nil, nil, trendOutput, 1_000),
		verifiedProviderGET("social.trend.list", "bilibili", "fetch_hot_search_api_v1_bilibili_web_fetch_hot_search_get", "/api/v1/bilibili/web/fetch_hot_search", nil, map[string]any{"limit": 20}, trendOutput, 1_000),
		verifiedProviderGET("social.trend.list", "kuaishou", "fetch_kuaishou_hot_list_v2_api_v1_kuaishou_web_fetch_kuaishou_hot_list_v2_get", "/api/v1/kuaishou/web/fetch_kuaishou_hot_list_v2", nil, nil, trendOutput, 1_000),
		verifiedProviderGET("social.trend.list", "zhihu", "fetch_hot_recommend_api_v1_zhihu_web_fetch_hot_recommend_get", "/api/v1/zhihu/web/fetch_hot_recommend", nil, nil, trendOutput, 1_000),
	}

	definitions := []standardCapabilityDefinition{
		standardDefinition("social.account.get", "社交账号详情", "社交媒体", "按平台和公开账号标识获取标准化账号资料。", "account_ref", accountOutput, accountBindings),
		standardDefinition("social.account.contents.list", "账号公开内容", "社交媒体", "获取公开账号发布的内容列表。", "account_ref", contentListOutput, contentsBindings),
		standardDefinition("social.content.get", "社交内容详情", "社交媒体", "获取公开帖子、笔记或视频的标准化详情。", "content_ref", contentOutput, contentBindings),
		standardDefinition("social.content.search", "社交内容搜索", "社交媒体", "按平台搜索公开内容。", "query", contentListOutput, searchBindings),
		standardDefinition("social.comment.list", "内容评论", "社交媒体", "获取公开内容的一级评论列表。", "content_ref", commentListOutput, commentBindings),
		standardReplyDefinition(commentListOutput, replyBindings),
		standardPlatformOnlyDefinition("social.trend.list", "平台趋势榜", "社交媒体", "获取指定平台的实时公开趋势榜单。", trendOutput, trendBindings),
		standardDefinition("commerce.product.search", "商品搜索", "电商", "按平台和关键词搜索公开商品。", "query", productListOutput, []ProviderOperation{
			verifiedProviderGET("commerce.product.search", "tiktok_shop", "fetch_search_products_list_api_v1_tiktok_shop_web_fetch_search_products_list_get", "/api/v1/tiktok/shop/web/fetch_search_products_list", map[string]string{"query": "search_word"}, map[string]any{"offset": 0, "region": "US"}, productListOutput, 1_000),
			verifiedProviderGET("commerce.product.search", "xiaohongshu", "search_products_api_v1_xiaohongshu_app_v2_search_products_get", "/api/v1/xiaohongshu/app_v2/search_products", map[string]string{"query": "keyword"}, map[string]any{"page": 1}, productListOutput, 1_000),
		}),
		standardDefinition("commerce.product.get", "商品详情", "电商", "获取公开商品的标准化详情。", "product_ref", productOutput, []ProviderOperation{
			verifiedProviderGET("commerce.product.get", "tiktok_shop", "fetch_product_detail_v3_api_v1_tiktok_shop_web_fetch_product_detail_v3_get", "/api/v1/tiktok/shop/web/fetch_product_detail_v3", map[string]string{"product_ref": "product_id"}, map[string]any{"region": "US"}, productOutput, 1_000),
			verifiedProviderGET("commerce.product.get", "xiaohongshu", "get_product_detail_api_v1_xiaohongshu_app_v2_get_product_detail_get", "/api/v1/xiaohongshu/app_v2/get_product_detail", map[string]string{"product_ref": "sku_id"}, nil, productOutput, 1_000),
		}),
		standardDefinition("commerce.product.reviews.list", "商品评价", "电商", "获取公开商品评价列表。", "product_ref", reviewListOutput, []ProviderOperation{
			verifiedProviderGET("commerce.product.reviews.list", "tiktok_shop", "fetch_product_reviews_v2_api_v1_tiktok_shop_web_fetch_product_reviews_v2_get", "/api/v1/tiktok/shop/web/fetch_product_reviews_v2", map[string]string{"product_ref": "product_id"}, map[string]any{"page_start": 1, "sort_rule": 2, "filter_type": 1, "filter_value": 6, "region": "US"}, reviewListOutput, 1_000),
			verifiedProviderGET("commerce.product.reviews.list", "xiaohongshu", "get_product_reviews_api_v1_xiaohongshu_app_v2_get_product_reviews_get", "/api/v1/xiaohongshu/app_v2/get_product_reviews", map[string]string{"product_ref": "sku_id"}, nil, reviewListOutput, 1_000),
		}),
		standardDefinition("job.search", "职位搜索", "职位与机会", "按关键词搜索公开职位。", "query", jobListOutput, []ProviderOperation{
			verifiedProviderGET("job.search", "linkedin", "search_jobs_api_v1_linkedin_web_v2_search_jobs_get", "/api/v1/linkedin/web_v2/search_jobs", map[string]string{"query": "keywords"}, nil, jobListOutput, 1_000),
		}),
		standardDefinition("job.get", "职位详情", "职位与机会", "获取公开职位详情。", "content_ref", jobOutput, []ProviderOperation{
			verifiedProviderGET("job.get", "linkedin", "get_job_detail_api_v1_linkedin_web_v2_get_job_detail_get", "/api/v1/linkedin/web_v2/get_job_detail", map[string]string{"content_ref": "url"}, nil, jobOutput, 1_000),
		}),
	}
	return definitions
}

func standardDefinition(operationKey, name, category, description, parameterName string, outputSchema map[string]any, bindings []ProviderOperation) standardCapabilityDefinition {
	properties := map[string]any{
		"platform":    stringSchema(bindingPlatforms(bindings)),
		parameterName: stringSchema(nil),
	}
	return standardCapabilityDefinition{
		OperationKey: operationKey, ContractVersion: "v1", Name: name, Category: category, Description: description,
		InputSchema: objectInputSchema(properties, []any{"platform", parameterName}), OutputSchema: outputSchema, Bindings: bindings,
	}
}

func standardReplyDefinition(outputSchema map[string]any, bindings []ProviderOperation) standardCapabilityDefinition {
	return standardCapabilityDefinition{
		OperationKey: "social.comment.replies.list", ContractVersion: "v1", Name: "评论回复", Category: "社交媒体",
		Description: "获取公开评论的回复列表。",
		InputSchema: objectInputSchema(map[string]any{
			"platform": stringSchema(bindingPlatforms(bindings)), "content_ref": stringSchema(nil), "comment_ref": stringSchema(nil),
		}, []any{"platform", "content_ref", "comment_ref"}),
		OutputSchema: outputSchema, Bindings: bindings,
	}
}

func standardPlatformOnlyDefinition(operationKey, name, category, description string, outputSchema map[string]any, bindings []ProviderOperation) standardCapabilityDefinition {
	return standardCapabilityDefinition{
		OperationKey: operationKey, ContractVersion: "v1", Name: name, Category: category, Description: description,
		InputSchema:  objectInputSchema(map[string]any{"platform": stringSchema(bindingPlatforms(bindings))}, []any{"platform"}),
		OutputSchema: outputSchema, Bindings: bindings,
	}
}

func bindingPlatforms(bindings []ProviderOperation) []any {
	platforms := make([]any, 0, len(bindings))
	for _, binding := range bindings {
		platforms = append(platforms, binding.Platform)
	}
	return platforms
}
