/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/

const PLATFORM_LABELS: Record<string, string> = {
  douyin: '抖音',
  instagram: 'Instagram',
  linkedin: 'LinkedIn',
  reddit: 'Reddit',
  taobao: '淘宝',
  tiktok: 'TikTok',
  tiktok_shop: 'TikTok Shop',
  wechat_channels: '微信视频号',
  wechat_mp: '微信公众号',
  weibo: '微博',
  xiaohongshu: '小红书',
  youtube: 'YouTube',
}

export function formatVSearchPlatformLabel(platform: string) {
  const normalized = platform.trim().toLocaleLowerCase()
  return PLATFORM_LABELS[normalized] || platform.trim().replaceAll('_', ' ')
}
