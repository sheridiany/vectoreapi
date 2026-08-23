import { Link } from '@tanstack/react-router'
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
  ArrowRight,
  BarChart3,
  Braces,
  Check,
  KeyRound,
  Network,
  ShieldCheck,
} from 'lucide-react'

import { Badge } from '@/components/ui/badge'

import {
  DocsCard,
  DocsCodeBlock,
  DocsShell,
  DocsSectionTitle,
} from '../components/docs-shell'

const quickLinks = [
  {
    to: '/docs/getting-started' as const,
    icon: KeyRound,
    label: '快速开始',
    description: '创建访问 Key，完成第一条 API 请求。',
  },
  {
    to: '/docs/guide' as const,
    icon: Network,
    label: '接入向导',
    description: '按客户端、IDE 或服务端场景选择配置。',
  },
  {
    to: '/docs/api' as const,
    icon: Braces,
    label: 'API 总览',
    description: '查看协议、鉴权方式与可用端点。',
  },
  {
    to: '/docs/troubleshooting' as const,
    icon: ShieldCheck,
    label: '问题排查',
    description: '遇到错误时，按状态码快速定位。',
  },
]

export function DocsHomePage() {
  return (
    <DocsShell
      eyebrow='快速开始（必读）'
      title='快速开始（必读）'
      description='第一次使用向量纪元 Relay 时，先创建企业访问 Key，再按目标选择下一篇文档。'
      sections={[
        { id: 'start', label: '从这里开始' },
        { id: 'capabilities', label: 'Relay 能做什么' },
        { id: 'request', label: '最小请求' },
      ]}
    >
      <div className='space-y-12'>
        <section id='start' className='scroll-mt-28'>
          <div className='mb-5 flex items-end justify-between gap-4'>
            <div>
              <div className='mb-2 text-[0.68rem] font-semibold tracking-[0.18em] text-[#1683c5] uppercase'>
                Start here
              </div>
              <h2 className='text-2xl font-semibold tracking-[-0.025em]'>
                你现在要做什么？
              </h2>
            </div>
            <Badge
              variant='outline'
              className='hidden border-[#e4e4e7] text-[#71717a] sm:inline-flex'
            >
              4 条清晰路径
            </Badge>
          </div>
          <div className='grid gap-3 sm:grid-cols-2'>
            {quickLinks.map((item) => {
              const Icon = item.icon
              return (
                <Link
                  key={item.to}
                  to={item.to}
                  className='group rounded-xl border border-[#e4e4e7] bg-[#fafafa] p-5 transition-colors hover:border-[#a1a1aa] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#1683c5] dark:border-white/10 dark:bg-white/[0.035] dark:hover:border-white/25'
                >
                  <div className='mb-8 flex items-center justify-between'>
                    <span className='grid size-9 place-items-center rounded-lg bg-[#f4f4f5] text-[#1683c5] dark:bg-white/[0.08]'>
                      <Icon className='size-[1.1rem]' />
                    </span>
                    <ArrowRight className='size-4 text-[#a1a1aa] transition-transform group-hover:translate-x-1 group-hover:text-[#1683c5]' />
                  </div>
                  <h3 className='text-lg font-semibold'>{item.label}</h3>
                  <p className='mt-1.5 text-sm leading-6 text-[#71717a] dark:text-[#a1a1aa]'>
                    {item.description}
                  </p>
                </Link>
              )
            })}
          </div>
        </section>

        <section id='capabilities' className='scroll-mt-28 space-y-5'>
          <DocsSectionTitle id='capabilities'>
            一层网关，收拢复杂度
          </DocsSectionTitle>
          <p className='max-w-2xl text-sm leading-7 text-[#52525b] dark:text-[#c4c4cc]'>
            Relay 对外提供稳定的 API
            入口，对内负责模型路由、渠道切换、访问控制与用量记录。客户端只需要记住一组
            Base URL 和一个 Key。
          </p>
          <div className='grid gap-3 sm:grid-cols-3'>
            <DocsCard>
              <Network className='size-5 text-[#1683c5]' />
              <h3 className='mt-5 text-sm font-semibold'>统一入口</h3>
              <p className='mt-2 text-xs leading-5 text-[#71717a]'>
                OpenAI 兼容接口优先，减少客户端切换成本。
              </p>
            </DocsCard>
            <DocsCard>
              <BarChart3 className='size-5 text-[#1683c5]' />
              <h3 className='mt-5 text-sm font-semibold'>可见用量</h3>
              <p className='mt-2 text-xs leading-5 text-[#71717a]'>
                请求、Token、模型与企业用量统一记录。
              </p>
            </DocsCard>
            <DocsCard>
              <ShieldCheck className='size-5 text-[#1683c5]' />
              <h3 className='mt-5 text-sm font-semibold'>企业边界</h3>
              <p className='mt-2 text-xs leading-5 text-[#71717a]'>
                Key、成员和消耗按企业隔离，方便治理。
              </p>
            </DocsCard>
          </div>
        </section>

        <section id='request' className='scroll-mt-28 space-y-5'>
          <DocsSectionTitle id='request'>用一条请求验证接入</DocsSectionTitle>
          <p className='text-sm leading-7 text-[#52525b] dark:text-[#c4c4cc]'>
            将示例中的{' '}
            <code className='rounded bg-[#f4f4f5] px-1.5 py-0.5 font-mono text-xs dark:bg-white/10'>
              $RELAY_API_KEY
            </code>{' '}
            替换为控制台创建的 Key，即可验证网络、鉴权和模型路由。
          </p>
          <DocsCodeBlock
            language='bash'
            title='最小 Chat Completions 请求'
            code={`curl https://gate.vectorepoch.com/v1/chat/completions \\
  -H "Authorization: Bearer $RELAY_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-5.1",
    "messages": [{"role": "user", "content": "你好"}]
  }'`}
          />
          <div className='flex flex-wrap items-center gap-x-5 gap-y-2 text-xs text-[#71717a]'>
            <span className='inline-flex items-center gap-1.5'>
              <Check className='size-3.5 text-[#2c9a70]' />
              返回 200 即表示链路正常
            </span>
            <Link
              to='/docs/troubleshooting'
              className='inline-flex items-center gap-1.5 font-medium text-[#1683c5] hover:underline'
            >
              遇到错误？看排查指南 <ArrowRight className='size-3.5' />
            </Link>
          </div>
        </section>

        <div className='border-t border-[#e4e4e7] pt-6 text-xs leading-6 text-[#71717a] dark:border-white/10'>
          文档会随 Relay 的接口和模型能力持续更新。生产环境接入前，请先在测试
          Key 下验证模型、超时和流式响应。
        </div>
      </div>
    </DocsShell>
  )
}
