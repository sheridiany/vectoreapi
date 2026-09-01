/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import {
  BookOpen,
  Braces,
  CircleDot,
  Radio,
  type LucideIcon,
} from 'lucide-react'

export type DocsRoute =
  | '/docs'
  | '/docs/getting-started'
  | '/docs/guide'
  | '/docs/guide/registration'
  | '/docs/guide/chat-clients'
  | '/docs/api'
  | '/docs/api/chat-completions'
  | '/docs/api/vsearch'
  | '/docs/troubleshooting'

export type DocsNavItem = {
  label: string
  description?: string
  to: DocsRoute
}

type DocsNavGroup = {
  label: string
  icon: LucideIcon
  items: readonly DocsNavItem[]
}

export type DocsSearchItem = DocsNavItem & { group: string }

export const docsNavGroups: readonly DocsNavGroup[] = [
  {
    label: '开始使用',
    icon: BookOpen,
    items: [
      {
        label: '快速开始（必读）',
        description: '先了解 Relay 的接入方式',
        to: '/docs',
      },
      {
        label: '五分钟完成调用',
        description: '拿到 Key，发出第一条请求',
        to: '/docs/getting-started',
      },
    ],
  },
  {
    label: '接入指南',
    icon: Radio,
    items: [
      {
        label: '接入向导',
        description: '按场景选择配置路径',
        to: '/docs/guide',
      },
      {
        label: '创建访问 Key',
        description: '控制台里的密钥与权限',
        to: '/docs/guide/registration',
      },
      {
        label: '聊天客户端',
        description: 'Cherry Studio、IDE 与移动端',
        to: '/docs/guide/chat-clients',
      },
    ],
  },
  {
    label: 'API 参考',
    icon: Braces,
    items: [
      {
        label: '接口总览',
        description: '协议、认证与端点索引',
        to: '/docs/api',
      },
      {
        label: 'Chat Completions',
        description: 'OpenAI 兼容对话接口',
        to: '/docs/api/chat-completions',
      },
      {
        label: 'vSearch 数据能力',
        description: '平台、能力、密钥与 MCP 调用',
        to: '/docs/api/vsearch',
      },
    ],
  },
  {
    label: '问题排查',
    icon: CircleDot,
    items: [
      {
        label: '常见错误',
        description: '401、403、429、503 快速定位',
        to: '/docs/troubleshooting',
      },
    ],
  },
] as const

const searchItems: DocsSearchItem[] = docsNavGroups.flatMap((group) =>
  group.items.map((item) => ({ ...item, group: group.label }))
)

export function filterDocsSearchItems(query: string): DocsSearchItem[] {
  const normalizedQuery = query.trim().toLowerCase()
  if (!normalizedQuery) return searchItems.slice(0, 6)

  return searchItems.filter((item) =>
    `${item.label} ${item.description} ${item.group}`
      .toLowerCase()
      .includes(normalizedQuery)
  )
}
