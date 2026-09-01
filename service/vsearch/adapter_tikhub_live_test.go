package vsearch

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTikHubPublishedBindingsLive is an opt-in paid integration test. It is the
// release gate for bindings marked ContractEquivalent and BillingReady: every
// published binding must dispatch three successful requests and return data
// that satisfies the public vSearch response schema.
func TestTikHubPublishedBindingsLive(t *testing.T) {
	secret := os.Getenv("TIKHUB_TEST_KEY")
	if secret == "" {
		t.Skip("TIKHUB_TEST_KEY is required for paid live verification")
	}
	adapter, err := NewTikHubAdapter(AdapterConfig{
		Provider: ProviderTikHub,
		Secret:   secret,
		Timeout:  90 * time.Second,
	})
	require.NoError(t, err)

	for _, definition := range standardCapabilityRegistry() {
		for _, operation := range definition.Bindings {
			operation := operation
			if !operation.ContractEquivalent || !operation.BillingReady {
				continue
			}
			t.Run(operation.Platform+"/"+operation.OperationID, func(t *testing.T) {
				params := tikHubLiveCanonicalParams(t, operation)
				for run := 1; run <= 3; run++ {
					ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
					result, executeErr := adapter.Execute(ctx, operation, CanonicalRequest{
						OperationKey: operation.OperationKey,
						Platform:     operation.Platform,
						Params:       params,
					})
					cancel()
					require.NoErrorf(t, executeErr, "live run %d failed", run)
					assert.Truef(t, result.Dispatched, "live run %d did not dispatch", run)
					assert.Equalf(t, 200, result.HTTPStatus, "live run %d returned unexpected HTTP status", run)
					require.NoErrorf(t, validateSchemaValue(result.Data, operation.OutputSchema, "$"), "live run %d violated the public schema", run)
				}
			})
		}
	}
}

func tikHubLiveCanonicalParams(t *testing.T, operation ProviderOperation) map[string]any {
	t.Helper()
	params := map[string]any{"platform": operation.Platform}
	for canonicalName := range operation.ParameterMap {
		switch canonicalName {
		case "query":
			if operation.OperationKey == "commerce.product.search" {
				params[canonicalName] = "手机壳"
			} else {
				params[canonicalName] = "人工智能"
			}
		case "account_ref":
			params[canonicalName] = tikHubLiveAccountRef(t, operation.OperationID)
		case "content_ref":
			params[canonicalName] = tikHubLiveContentRef(t, operation.OperationID)
		case "comment_ref":
			params[canonicalName] = tikHubLiveCommentRef(t, operation.OperationID)
		case "product_ref":
			params[canonicalName] = tikHubLiveProductRef(t, operation.OperationID)
		default:
			t.Fatalf("missing live sample for canonical parameter %q in %s", canonicalName, operation.OperationID)
		}
	}
	return params
}

func tikHubLiveAccountRef(t *testing.T, operationID string) string {
	t.Helper()
	refs := map[string]string{
		"fetch_user_profile_by_uid_api_v1_douyin_web_fetch_user_profile_by_uid_get": "68141954464",
		"fetch_user_post_videos_api_v1_douyin_web_fetch_user_post_videos_get":       "MS4wLjABAAAANXSltcLCzDGmdNFI2Q_QixVTr67NiYzjKOIP5s03CAE",
		"fetch_user_profile_api_v1_tiktok_web_fetch_user_profile_get":               "tiktok",
		"fetch_user_post_api_v1_tiktok_web_fetch_user_post_get":                     "MS4wLjABAAAAv7iSuuXDJGDvJkmH_vz1qkDZYo1apxgzaxdBSeIuPiM",
		"get_user_info_api_v1_xiaohongshu_app_v2_get_user_info_get":                 "61b46d790000000010008153",
		"get_user_posted_notes_api_v1_xiaohongshu_app_v2_get_user_posted_notes_get": "61b46d790000000010008153",
		"fetch_user_profile_api_v1_twitter_web_fetch_user_profile_get":              "elonmusk",
		"fetch_user_post_tweet_api_v1_twitter_web_fetch_user_post_tweet_get":        "44196397",
		"fetch_user_info_api_v1_weibo_app_fetch_user_info_get":                      "7648703289",
		"fetch_user_profile_feed_api_v1_weibo_app_fetch_user_profile_feed_get":      "6580994757",
		"fetch_user_profile_api_v1_reddit_app_fetch_user_profile_get":               "spez",
		"fetch_user_posts_api_v1_reddit_app_fetch_user_posts_get":                   "spez",
		"get_user_profile_api_v1_linkedin_web_v2_get_user_profile_get":              "https://www.linkedin.com/in/williamhgates/",
		"get_user_posts_api_v1_linkedin_web_v2_get_user_posts_get":                  "https://www.linkedin.com/in/williamhgates/",
		"get_user_profile_api_v1_instagram_v3_get_user_profile_get":                 "instagram",
		"get_user_posts_api_v1_instagram_v3_get_user_posts_get":                     "99brasil",
		"fetch_user_info_api_v1_bilibili_app_fetch_user_info_get":                   "203680252",
		"fetch_user_post_videos_api_v1_bilibili_web_fetch_user_post_videos_get":     "178360345",
		"fetch_user_info_api_v1_zhihu_web_fetch_user_info_get":                      "ji-qi-zhi-xin-65",
		"fetch_one_user_v2_api_v1_kuaishou_app_fetch_one_user_v2_get":               "3xz63mn6fngqtiq",
		"fetch_account_profile_api_v1_wechat_mp_v2_fetch_account_profile_post":      "gh_363b924965e9",
		"fetch_account_articles_api_v1_wechat_mp_v2_fetch_account_articles_post":    "gh_363b924965e9",
		"fetch_user_profile_api_v1_wechat_channels_v2_fetch_user_profile_post":      "v2_060000231003b20faec8c6e4811dc1d4c602ee30b0771bbcf220c67926bb76ab7702ac335a53@finder",
		"fetch_user_videos_api_v1_wechat_channels_v2_fetch_user_videos_post":        "v2_060000231003b20faec8c6e4811dc1d4c602ee30b0771bbcf220c67926bb76ab7702ac335a53@finder",
		"get_channel_info_api_v1_youtube_web_get_channel_info_get":                  "UCXuqSBlHAE6Xw-yeJA0Tunw",
		"get_channel_videos_api_v1_youtube_web_v2_get_channel_videos_get":           "UCJHBJ7F-nAIlMGolm0Hu4vg",
	}
	value, ok := refs[operationID]
	require.Truef(t, ok, "missing account sample for %s", operationID)
	return value
}

func tikHubLiveContentRef(t *testing.T, operationID string) string {
	t.Helper()
	refs := map[string]string{
		"fetch_item_base_api_v1_douyin_index_fetch_item_base_post":                        "7660090930283744575",
		"fetch_video_comments_api_v1_douyin_web_fetch_video_comments_get":                 "7372484719365098803",
		"fetch_video_comments_reply_api_v1_douyin_web_fetch_video_comment_replies_get":    "7354666303006723354",
		"fetch_one_video_v3_api_v1_tiktok_app_v3_fetch_one_video_v3_get":                  "7350810998023949599",
		"fetch_video_comments_api_v1_tiktok_app_v3_fetch_video_comments_get":              "7326156045968067873",
		"fetch_video_comments_reply_api_v1_tiktok_app_v3_fetch_video_comment_replies_get": "7326156045968067873",
		"get_image_note_detail_api_v1_xiaohongshu_app_v2_get_image_note_detail_get":       "697c0eee000000000a03c308",
		"get_note_comments_api_v1_xiaohongshu_app_v2_get_note_comments_get":               "697c0eee000000000a03c308",
		"get_note_sub_comments_api_v1_xiaohongshu_app_v2_get_note_sub_comments_get":       "699916e6000000001d0253da",
		"fetch_tweet_detail_api_v1_twitter_web_fetch_tweet_detail_get":                    "1808168603721650364",
		"fetch_post_comments_api_v1_twitter_web_fetch_post_comments_get":                  "1835124037934367098",
		"fetch_status_detail_api_v1_weibo_app_fetch_status_detail_get":                    "5016922058656962",
		"fetch_status_comments_api_v1_weibo_app_fetch_status_comments_get":                "5258708168476831",
		"fetch_post_sub_comments_api_v1_weibo_web_v2_fetch_post_sub_comments_get":         "5258708168476831",
		"fetch_post_details_api_v1_reddit_app_fetch_post_details_get":                     "t3_1ojnh50",
		"fetch_post_comments_api_v1_reddit_app_fetch_post_comments_get":                   "t3_1ojnvca",
		"fetch_comment_replies_api_v1_reddit_app_fetch_comment_replies_get":               "t3_1qmup73",
		"get_post_comments_api_v1_linkedin_web_v2_get_post_comments_get":                  "7267273010393358336",
		"get_post_info_by_code_api_v1_instagram_v3_get_post_info_by_code_get":             "DUajw4YkorV",
		"get_post_comments_api_v1_instagram_v3_get_post_comments_get":                     "DUajw4YkorV",
		"fetch_comment_replies_api_v1_instagram_v2_fetch_comment_replies_get":             "DRhvwVLAHAG",
		"fetch_one_video_api_v1_bilibili_app_fetch_one_video_get":                         "117191353569485",
		"fetch_collect_folders_api_v1_bilibili_web_fetch_video_comments_get":              "BV1M1421t7hT",
		"fetch_collect_folders_api_v1_bilibili_web_fetch_comment_reply_get":               "BV1M1421t7hT",
		"fetch_answer_detail_api_v1_zhihu_web_fetch_answer_detail_get":                    "2149271734",
		"fetch_comment_v5_api_v1_zhihu_web_fetch_comment_v5_get":                          "2149271734",
		"fetch_sub_comment_v5_api_v1_zhihu_web_fetch_sub_comment_v5_get":                  "2149271734",
		"fetch_one_video_v1_api_v1_kuaishou_app_fetch_one_video_get":                      "3xhpk3xcf6e4iac",
		"fetch_video_comment_api_v1_kuaishou_app_fetch_video_comment_get":                 "3x7gxp2zhgjv832",
		"fetch_video_sub_comments_api_v1_kuaishou_app_fetch_video_sub_comments_get":       "5218546261880462502",
		"fetch_article_detail_api_v1_wechat_mp_v2_fetch_article_detail_post":              "https://mp.weixin.qq.com/s/TSNQKkRpN1qbKsT7BvzqIw",
		"fetch_article_comments_api_v1_wechat_mp_v2_fetch_article_comments_post":          "https://mp.weixin.qq.com/s/TSNQKkRpN1qbKsT7BvzqIw",
		"fetch_comment_replies_api_v1_wechat_mp_v2_fetch_comment_replies_post":            "http://mp.weixin.qq.com/s?__biz=Mzk3NTA0MzM5NA==&mid=2247483745&idx=1&sn=3f34e768cf457a501038991ed30be1f4#rd",
		"fetch_video_detail_api_v1_wechat_channels_v2_fetch_video_detail_post":            "14941130915890399732",
		"fetch_video_comments_api_v1_wechat_channels_v2_fetch_video_comments_post":        "14941130915890399732",
		"get_video_info_v2_api_v1_youtube_web_v2_get_video_info_v2_get":                   "dQw4w9WgXcQ",
		"get_video_comments_api_v1_youtube_web_v2_get_video_comments_get":                 "LuIL5JATZsc",
		"get_video_comment_replies_api_v1_youtube_web_v2_get_video_comment_replies_get":   "LuIL5JATZsc",
		"get_job_detail_api_v1_linkedin_web_v2_get_job_detail_get":                        "https://www.linkedin.com/jobs/view/software-engineer-at-pave-4310512612/",
	}
	value, ok := refs[operationID]
	require.Truef(t, ok, "missing content sample for %s", operationID)
	return value
}

func tikHubLiveCommentRef(t *testing.T, operationID string) string {
	t.Helper()
	refs := map[string]string{
		"fetch_video_comments_reply_api_v1_douyin_web_fetch_video_comment_replies_get":    "7354669356632638218",
		"fetch_video_comments_reply_api_v1_tiktok_app_v3_fetch_video_comment_replies_get": "7327061675382260482",
		"get_note_sub_comments_api_v1_xiaohongshu_app_v2_get_note_sub_comments_get":       "699fb9930000000008030db6",
		"fetch_post_sub_comments_api_v1_weibo_web_v2_fetch_post_sub_comments_get":         "5283574244704555",
		"fetch_comment_replies_api_v1_reddit_app_fetch_comment_replies_get":               "commenttree:ex:(RjiJd",
		"fetch_comment_replies_api_v1_instagram_v2_fetch_comment_replies_get":             "18067775592012345",
		"fetch_collect_folders_api_v1_bilibili_web_fetch_comment_reply_get":               "237109455120",
		"fetch_sub_comment_v5_api_v1_zhihu_web_fetch_sub_comment_v5_get":                  "1733086022",
		"fetch_video_sub_comments_api_v1_kuaishou_app_fetch_video_sub_comments_get":       "14000000123456789",
		"fetch_comment_replies_api_v1_wechat_mp_v2_fetch_comment_replies_post":            "12109128638545265979",
		"fetch_video_comments_api_v1_wechat_channels_v2_fetch_video_comments_post":        "0",
		"get_video_comment_replies_api_v1_youtube_web_v2_get_video_comment_replies_get":   "Eg0SC0x1SUw1SkFUWnNjGAYyDyIPIgtMdUlMNUpBVFpjYzAAeAEoFA",
	}
	value, ok := refs[operationID]
	require.Truef(t, ok, "missing comment sample for %s", operationID)
	return value
}

func tikHubLiveProductRef(t *testing.T, operationID string) string {
	t.Helper()
	refs := map[string]string{
		"fetch_product_detail_v3_api_v1_tiktok_shop_web_fetch_product_detail_v3_get":   "1731709434595676197",
		"fetch_product_reviews_v2_api_v1_tiktok_shop_web_fetch_product_reviews_v2_get": "1731709434595676197",
		"get_product_detail_api_v1_xiaohongshu_app_v2_get_product_detail_get":          "669ddd44e05f3700011067ed",
		"get_product_reviews_api_v1_xiaohongshu_app_v2_get_product_reviews_get":        "669ddd44e05f3700011067ed",
	}
	value, ok := refs[operationID]
	require.Truef(t, ok, "missing product sample for %s", operationID)
	return value
}
