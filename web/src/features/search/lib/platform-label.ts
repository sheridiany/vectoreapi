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
  taobao: '淘宝',
  tiktok: 'TikTok',
  tiktok_shop: 'TikTok Shop',
  xiaohongshu: '小红书',
}

export function formatVSearchPlatformLabel(platform: string) {
  const normalized = platform.trim().toLocaleLowerCase()
  return PLATFORM_LABELS[normalized] || platform.trim().replaceAll('_', ' ')
}
