# TikHub 平台能力候选：vSearch 标准合同映射

> 调研日期：2026-08-31
> 官方目录版本：TikHub OpenAPI V5.3.2（2026-06-22）
> 范围：微博、微信公众号、微信视频号、YouTube、Reddit、LinkedIn。
> 证据：只读取 [TikHub 官方 OpenAPI](https://api.tikhub.io/openapi.json) 与其中的接口说明；未写入密钥、未发起计费调用。

## 1. 结论

用户端的平台筛选建议为：

`全部｜微博｜微信公众号｜微信视频号｜YouTube｜Reddit｜LinkedIn`

这些平台在 TikHub 官方目录中均有候选接口，但**目录存在不等于 vSearch 已可调用**。在真实请求、字段契约、分页、价格和错误语义验证完成前，六个平台都只能在用户端标为“规划中”。

| 平台 | 目录覆盖 | 格式化质量候选 | 当前可发布状态 |
| --- | --- | --- | --- |
| 微博 | 账号、发文、搜索、详情、评论、回复、热搜 | 低至中：文档描述业务字段，但响应统一引用通用 `ResponseModel`，无格式化开关 | 规划中 |
| 微信公众号 | 账号、文章列表、综合搜索、文章详情、评论、回复 | **高**：官方提供 `raw=false` 的 snake_case 精简结构与 JSON Path | 规划中 |
| 微信视频号 | 账号、作品列表、搜索、详情、评论、回复 | **高**：官方提供 `raw=false` 的 snake_case 精简结构与 JSON Path | 规划中 |
| YouTube | 频道、频道视频、综合搜索、视频详情、评论、回复、趋势 | **高**：V2 多数接口提供 `need_format=true` 或明确标为 cleaned structured data | 规划中 |
| Reddit | 用户、用户帖子、动态搜索、帖子详情、评论、回复、趋势 | 中：有 `need_format` 开关，但官方描述没有为所有候选接口列出稳定的精简字段键 | 规划中 |
| LinkedIn | 用户/公司资料与帖子、职位搜索/详情、帖子详情、评论 | 中：V2 文档给出结构化业务字段，但没有通用格式化开关；部分接口提示资源池有限 | 规划中 |

## 2. 标准合同映射原则

- 对用户只暴露 `platform + canonical operation`，不暴露 TikHub operation、账号或密钥。
- 账号、内容、评论、回复和趋势使用独立合同；不要把不同实体混进一个“万能搜索”返回。
- 上游 cursor/token 只包装为当前 binding 的分页游标，不能跨平台或跨供应商续页。
- 所有 64 位平台 ID 以字符串传递。微信公众号和视频号文档明确提示其 ID 超出 JavaScript 安全整数范围。
- 有 `raw=false` / `need_format=true` 时优先用格式化结果，但仍需做真实契约测试，不能原样透传为 vSearch 标准响应。
- 下表“必选参数”按官方 OpenAPI及接口说明记录；“建议输出”是适合 vSearch 的最小公共字段，不代表上游完整字段。

## 3. 微博

官方候选采用 `Weibo-Web-V2-API`。该组文档未在接口说明中标出逐请求价格，且响应 schema 只引用通用 `ResponseModel`，因此首批全部保持规划中。

| vSearch 标准能力 | TikHub 接口 | Method | 必选参数 | 官方描述的主要返回信息 | 文档标价 |
| --- | --- | --- | --- | --- | --- |
| `social.account.get` | [`/api/v1/weibo/web_v2/fetch_user_info`](https://api.tikhub.io/#/Weibo-Web-V2-API/fetch_user_info_api_v1_weibo_web_v2_fetch_user_info_get) | GET | `uid` / `custom` 至少一个 | 昵称、头像、简介、关注数、粉丝数 | 未标 |
| `social.account.contents.list` | [`/api/v1/weibo/web_v2/fetch_user_posts`](https://api.tikhub.io/#/Weibo-Web-V2-API/fetch_user_posts_api_v1_weibo_web_v2_fetch_user_posts_get) | GET | `uid` | 微博正文、图片、视频、`since_id` | 未标 |
| `social.content.search` | [`/api/v1/weibo/web_v2/fetch_realtime_search`](https://api.tikhub.io/#/Weibo-Web-V2-API/fetch_realtime_search_api_v1_weibo_web_v2_fetch_realtime_search_get) | GET | `query` | 最新微博、作者、图片、视频、互动数据 | 未标 |
| `social.content.get` | [`/api/v1/weibo/web_v2/fetch_post_detail`](https://api.tikhub.io/#/Weibo-Web-V2-API/fetch_post_detail_api_v1_weibo_web_v2_fetch_post_detail_get) | GET | `id` | 全文、图片、视频、点赞/评论/转发数 | 未标 |
| `social.comment.list` | [`/api/v1/weibo/web_v2/fetch_post_comments`](https://api.tikhub.io/#/Weibo-Web-V2-API/fetch_post_comments_api_v1_weibo_web_v2_fetch_post_comments_get) | GET | `id` | 评论正文、评论者、点赞数、`max_id` | 未标 |
| `social.comment.replies.list` | [`/api/v1/weibo/web_v2/fetch_post_sub_comments`](https://api.tikhub.io/#/Weibo-Web-V2-API/fetch_post_sub_comments_api_v1_weibo_web_v2_fetch_post_sub_comments_get) | GET | `id`（主评论 ID） | 回复正文、回复者、点赞数、`max_id` | 未标 |
| `social.trend.list` | [`/api/v1/weibo/web_v2/fetch_hot_search_summary`](https://api.tikhub.io/#/Weibo-Web-V2-API/fetch_hot_search_summary_api_v1_weibo_web_v2_fetch_hot_search_summary_get) | GET | 无 | 50 条热搜；排名、关键词、标签、热度 | 未标 |

建议输出：账号统一为 `id/name/avatar/bio/follower_count/following_count`；内容统一为 `id/text/author/published_at/media/engagement/url`；趋势统一为 `title/rank/heat/tag/url`。这些字段必须通过实测样本确认 JSON Path 后再发布。

## 4. 微信公众号

微信公众号 V2 是最适合首批接入的候选之一：所有代表接口均明确标价 **$0.01/次**，并提供 `raw=false` 精简结构。

| vSearch 标准能力 | TikHub 接口 | Method | 必选参数 | `raw=false` 的主要字段 | 文档标价 |
| --- | --- | --- | --- | --- | --- |
| `social.account.get` | [`/api/v1/wechat_mp/v2/fetch_account_profile`](https://api.tikhub.io/#/WeChat-Media-Platform-V2-API/fetch_account_profile_api_v1_wechat_mp_v2_fetch_account_profile_post) | POST | body `username`（`gh_...`） | `user_name/nick_name/signature/head_url/service_type/ban_type` | $0.01 |
| `social.account.contents.list` | [`/api/v1/wechat_mp/v2/fetch_account_articles`](https://api.tikhub.io/#/WeChat-Media-Platform-V2-API/fetch_account_articles_api_v1_wechat_mp_v2_fetch_account_articles_post) | POST | body `username` | `articles[].app_msg_id/title/digest/url/cover/create_time`、`next_offset/is_end` | $0.01 |
| `social.content.search` | [`/api/v1/wechat_search/v2/fetch_search`](https://api.tikhub.io/#/WeChat-Search-V2-API/fetch_search_api_v1_wechat_search_v2_fetch_search_post) | POST | body `keyword` | `items[].title/desc/docID/jumpInfo`、`cursor/continue_flag` | $0.01 |
| `social.content.get` | [`/api/v1/wechat_mp/v2/fetch_article_detail`](https://api.tikhub.io/#/WeChat-Media-Platform-V2-API/fetch_article_detail_api_v1_wechat_mp_v2_fetch_article_detail_post) | POST | body `url` | 标题、公众号、作者、摘要、发布时间、封面、`content_text`、合集 | $0.01 |
| `social.comment.list` | [`/api/v1/wechat_mp/v2/fetch_article_comments`](https://api.tikhub.io/#/WeChat-Media-Platform-V2-API/fetch_article_comments_api_v1_wechat_mp_v2_fetch_article_comments_post) | POST | body `url` | `comments[].content_id/nick_name/content/like_num/create_time/ip_wording/reply_total`、`buffer/has_more` | $0.01 |
| `social.comment.replies.list` | [`/api/v1/wechat_mp/v2/fetch_comment_replies`](https://api.tikhub.io/#/WeChat-Media-Platform-V2-API/fetch_comment_replies_api_v1_wechat_mp_v2_fetch_comment_replies_post) | POST | body `url`；vSearch 建议同时要求 `content_id` | `replies[].reply_id/nick_name/content/create_time/ip_wording`、`next_offset/has_more` | $0.01 |

搜索时应按标准能力固定 `business_type`：公众号账号搜索用 `account`，文章搜索用 `article`。不要把 `all/live_stream/moments/news/book/image/...` 全部塞进首版内容合同。

## 5. 微信视频号

视频号 V2 同样有高质量精简结构，所有代表接口明确标价 **$0.01/次**。大整数 `object_id/comment_id` 必须作为字符串。

| vSearch 标准能力 | TikHub 接口 | Method | 必选参数 | `raw=false` 的主要字段 | 文档标价 |
| --- | --- | --- | --- | --- | --- |
| `social.account.get` | [`/api/v1/wechat_channels/v2/fetch_user_profile`](https://api.tikhub.io/#/WeChat-Channels-V2-API/fetch_user_profile_api_v1_wechat_channels_v2_fetch_user_profile_post) | POST | body `username`（`v2_...@finder`） | 昵称、签名、头像、地区、粉丝/作品/获赞/收藏/转发数、认证、关联公众号 | $0.01 |
| `social.account.contents.list` | [`/api/v1/wechat_channels/v2/fetch_user_videos`](https://api.tikhub.io/#/WeChat-Channels-V2-API/fetch_user_videos_api_v1_wechat_channels_v2_fetch_user_videos_post) | POST | body `username` | `videos[].id/title/read_count/like_count/fav_count/forward_count/comment_count/create_time/location`、`last_buffer` | $0.01 |
| `social.content.search` | [`/api/v1/wechat_search/v2/fetch_search_videos`](https://api.tikhub.io/#/WeChat-Search-V2-API/fetch_search_videos_api_v1_wechat_search_v2_fetch_search_videos_post) | POST | body `keyword` | 视频搜索项、`exportId/feedNonceId`、`cursor/continue_flag` | $0.01 |
| `social.account.contents.search` | [`/api/v1/wechat_channels/v2/fetch_search_channel_videos`](https://api.tikhub.io/#/WeChat-Channels-V2-API/fetch_search_channel_videos_api_v1_wechat_channels_v2_fetch_search_channel_videos_post) | POST | body `username`、`keyword` | 指定账号内的视频搜索结果 | $0.01 |
| `social.content.get` | [`/api/v1/wechat_channels/v2/fetch_video_detail`](https://api.tikhub.io/#/WeChat-Channels-V2-API/fetch_video_detail_api_v1_wechat_channels_v2_fetch_video_detail_post) | POST | `object_id` / `export_id` / `share_url` 至少一个 | `id/username/nickname/title`、互动计数、发布时间、类型、位置、媒体对象 | $0.01 |
| `social.comment.list` | [`/api/v1/wechat_channels/v2/fetch_video_comments`](https://api.tikhub.io/#/WeChat-Channels-V2-API/fetch_video_comments_api_v1_wechat_channels_v2_fetch_video_comments_post) | POST | body `object_id` | `comments[].comment_id/nickname/username/content/like_count/reply_count/create_time/ip_region`、`last_buffer` | $0.01 |
| `social.comment.replies.list` | 同上 `fetch_video_comments` | POST | body `object_id`、`comment_id` | 同一精简评论结构；`comment_id` 指定父评论 | $0.01 |

视频搜索只返回临时 `exportId` 时，需要立即调用详情接口换取稳定 `object_id` 与标准字段。媒体下载地址和 `decode_key` 不应默认进入公共 vSearch 合同；它们具有短时效与额外安全责任。

## 6. YouTube

YouTube V2 多个接口直接提供清洗后的返回，适合标准化。但下列代表接口的说明未给出明确单价，必须通过 TikHub 账户价格页或真实账单确认后才能设置用户价格。

| vSearch 标准能力 | TikHub 接口 | Method | 必选参数 | 格式化返回候选 | 文档标价 |
| --- | --- | --- | --- | --- | --- |
| `social.account.get` | [`/api/v1/youtube/web/get_channel_info`](https://api.tikhub.io/#/YouTube-Web-API/get_channel_info_api_v1_youtube_web_get_channel_info_get) | GET | `channel_id` | 频道信息；具体键需实测 | 未标 |
| `social.account.contents.list` | [`/api/v1/youtube/web_v2/get_channel_videos`](https://api.tikhub.io/#/YouTube-Web-V2-API/get_channel_videos_api_v1_youtube_web_v2_get_channel_videos_get) | GET | `channel_id` | `need_format=true`：频道资料、`videos[].video_id/title/thumbnail/duration/view_count/published_time/url`、`continuation_token` | 未标 |
| `social.content.search` | [`/api/v1/youtube/web_v2/get_general_search_v2`](https://api.tikhub.io/#/YouTube-Web-V2-API/get_general_search_v2_api_v1_youtube_web_v2_get_general_search_v2_get) | GET | 首次 `keyword`；翻页 `continuation_token` | cleaned：`videos/shorts/channels/playlists` 与下一页 token | 未标 |
| `social.content.get` | [`/api/v1/youtube/web_v2/get_video_info_v2`](https://api.tikhub.io/#/YouTube-Web-V2-API/get_video_info_v2_api_v1_youtube_web_v2_get_video_info_v2_get) | GET | `video_id` / `video_url` 至少一个 | `need_format=true`：标题、描述、频道、发布时间、时长、播放/点赞/评论数、分类、缩略图、字幕、章节 | 未标 |
| `social.comment.list` | [`/api/v1/youtube/web_v2/get_video_comments`](https://api.tikhub.io/#/YouTube-Web-V2-API/get_video_comments_api_v1_youtube_web_v2_get_video_comments_get) | GET | `video_id` | `need_format=true`：评论 ID/正文/时间/点赞/回复数/作者、`continuation_token` | 未标 |
| `social.comment.replies.list` | [`/api/v1/youtube/web_v2/get_video_comment_replies`](https://api.tikhub.io/#/YouTube-Web-V2-API/get_video_comment_replies_api_v1_youtube_web_v2_get_video_comment_replies_get) | GET | `continuation_token` | `need_format=true`：与评论相同，并标记 reply level | 未标 |
| `social.trend.list` | [`/api/v1/youtube/web/get_trending_videos`](https://api.tikhub.io/#/YouTube-Web-API/get_trending_videos_api_v1_youtube_web_get_trending_videos_get) | GET | 无 | 趋势视频；支持地区、语言、Now/Music/Gaming/Movies | 未标 |

`get_general_search_v2` 同时返回多种实体。vSearch 首版应固定 `type=video`，只映射视频；频道搜索另建账号搜索合同，避免一个结果数组混合多种实体。

## 7. Reddit

Reddit APP 代表接口均带 `need_format` 参数，但官方文档没有像 YouTube/微信那样完整列出格式化后的稳定键，因此只能作为中等质量候选。下列接口未在说明中标出逐请求价格。

| vSearch 标准能力 | TikHub 接口 | Method | 必选参数 | 官方描述的主要返回信息 | 文档标价 |
| --- | --- | --- | --- | --- | --- |
| `social.account.get` | [`/api/v1/reddit/app/fetch_user_profile`](https://api.tikhub.io/#/Reddit-APP-API/fetch_user_profile_api_v1_reddit_app_fetch_user_profile_get) | GET | `username` | 用户名/ID、创建时间、帖子/评论 Karma、头像、简介、认证、徽章、关注者数 | 未标 |
| `social.account.contents.list` | [`/api/v1/reddit/app/fetch_user_posts`](https://api.tikhub.io/#/Reddit-APP-API/fetch_user_posts_api_v1_reddit_app_fetch_user_posts_get) | GET | `username` | 标题、正文、时间、Subreddit、点赞/评论数、内容类型、媒体、分页 | 未标 |
| `social.content.search` | [`/api/v1/reddit/app/fetch_dynamic_search`](https://api.tikhub.io/#/Reddit-APP-API/fetch_dynamic_search_api_v1_reddit_app_fetch_dynamic_search_get) | GET | `query` | 帖子/社区/评论/媒体/用户搜索与分页 | 未标 |
| `social.content.get` | [`/api/v1/reddit/app/fetch_post_details`](https://api.tikhub.io/#/Reddit-APP-API/fetch_post_details_api_v1_reddit_app_fetch_post_details_get) | GET | `post_id`（`t3_` 前缀） | 标题、正文、作者、互动、Subreddit、媒体、奖励 | 未标 |
| `social.comment.list` | [`/api/v1/reddit/app/fetch_post_comments`](https://api.tikhub.io/#/Reddit-APP-API/fetch_post_comments_api_v1_reddit_app_fetch_post_comments_get) | GET | `post_id` | 评论树；正文、作者、点赞，`more.cursor` 用于更多回复 | 未标 |
| `social.comment.replies.list` | [`/api/v1/reddit/app/fetch_comment_replies`](https://api.tikhub.io/#/Reddit-APP-API/fetch_comment_replies_api_v1_reddit_app_fetch_comment_replies_get) | GET | `post_id`、`cursor` | 子评论正文、作者、点赞与分页 | 未标 |
| `social.trend.list` | [`/api/v1/reddit/app/fetch_trending_searches`](https://api.tikhub.io/#/Reddit-APP-API/fetch_trending_searches_api_v1_reddit_app_fetch_trending_searches_get) | GET | 无 | 热门关键词、趋势话题、热度/搜索量、关联帖子预览 | 未标 |

首版搜索固定 `search_type=post`、`allow_nsfw=0`。社区、评论、媒体和用户搜索应分别进入独立合同，不能混用同一个 items schema。

## 8. LinkedIn

LinkedIn V2 目前适合账号、公司、帖子和职位实体，不应声称支持通用社交内容搜索、评论回复或趋势榜。文档未标出以下接口的逐请求价格；用户帖子、公司帖子、职位搜索/详情和评论等接口还提示资源池有限，可能暂时返回 400，需要有限重试。

| vSearch 标准能力 | TikHub 接口 | Method | 必选参数 | 官方描述的主要返回信息 | 文档标价 |
| --- | --- | --- | --- | --- | --- |
| `social.account.get` | [`/api/v1/linkedin/web_v2/get_user_profile`](https://api.tikhub.io/#/LinkedIn-Web-V2-API/get_user_profile_api_v1_linkedin_web_v2_get_user_profile_get) | GET | `url` | 姓名、当前公司、简介、地点、粉丝、经历、教育 | 未标 |
| `social.account.contents.list` | [`/api/v1/linkedin/web_v2/get_user_posts`](https://api.tikhub.io/#/LinkedIn-Web-V2-API/get_user_posts_api_v1_linkedin_web_v2_get_user_posts_get) | GET | `url` | 帖子列表与 `paging` | 未标 |
| `organization.get` | [`/api/v1/linkedin/web_v2/get_company_profile`](https://api.tikhub.io/#/LinkedIn-Web-V2-API/get_company_profile_api_v1_linkedin_web_v2_get_company_profile_get) | GET | `url` | 公司名称、简介、规模、行业、总部、粉丝、关联页面 | 未标 |
| `organization.contents.list` | [`/api/v1/linkedin/web_v2/get_company_posts`](https://api.tikhub.io/#/LinkedIn-Web-V2-API/get_company_posts_api_v1_linkedin_web_v2_get_company_posts_get) | GET | `url` | 公司帖子与 `paging` | 未标 |
| `job.search` | [`/api/v1/linkedin/web_v2/search_jobs`](https://api.tikhub.io/#/LinkedIn-Web-V2-API/search_jobs_api_v1_linkedin_web_v2_search_jobs_get) | GET | OpenAPI 无硬性必选；vSearch 建议要求 `keywords` | 公司、标题、地点、薪资、发布时间、总数 | 未标 |
| `job.get` | [`/api/v1/linkedin/web_v2/get_job_detail`](https://api.tikhub.io/#/LinkedIn-Web-V2-API/get_job_detail_api_v1_linkedin_web_v2_get_job_detail_get) | GET | `url` | 标题、描述、公司、薪资、地点、技能 | 未标 |
| `social.content.get` | [`/api/v1/linkedin/web_v2/get_post_detail`](https://api.tikhub.io/#/LinkedIn-Web-V2-API/get_post_detail_api_v1_linkedin_web_v2_get_post_detail_get) | GET | `url` | 正文、作者、时间、话题、图片/视频、互动数 | 未标 |
| `social.comment.list` | [`/api/v1/linkedin/web_v2/get_post_comments`](https://api.tikhub.io/#/LinkedIn-Web-V2-API/get_post_comments_api_v1_linkedin_web_v2_get_post_comments_get) | GET | `urn` | 评论人、正文、时间、置顶、内嵌回复、下一页 token | 未标 |

LinkedIn 的公司和职位不应伪装成普通社交账号/内容。建议在 vSearch 标准层显式增加 `organization.*` 与 `job.*` 合同；如果首版只做 social，则这些能力先只展示为平台规划信息，不发布调用入口。

## 9. 推荐验证顺序

1. **微信公众号**：`raw=false` 契约和 $0.01 定价最明确；先验证账号、文章列表、详情、评论链路。
2. **微信视频号**：`raw=false` 契约明确；先验证搜索 → 详情 → 评论，以及 64 位 ID 字符串处理。
3. **YouTube**：优先 V2 + `need_format=true`，先验证搜索、详情、频道视频、评论链路并查实单价。
4. **微博**：先验证热搜和实时搜索两个最小合同，再扩账号/评论。
5. **Reddit**：验证 `need_format=true` 的真实字段、NSFW 默认过滤和 `t3_` ID 语义。
6. **LinkedIn**：先确认资源池稳定性和逐接口价格，再分别验证账号、公司、职位、帖子实体。

每个 operation 满足以下条件后，才从“规划中”改为“可用”：真实请求成功；标准响应字段完整；分页可继续；空结果/限流/失败可区分；实际扣费与配置成本一致；敏感上游信息不会出现在用户响应或日志中。
