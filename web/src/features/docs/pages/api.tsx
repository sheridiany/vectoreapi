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
  Braces,
  Check,
  Image,
  List,
  Radio,
  Video,
} from 'lucide-react'

import { Badge } from '@/components/ui/badge'

import {
  DocsCallout,
  DocsCodeBlock,
  DocsShell,
  DocsSectionTitle,
} from '../components/docs-shell'

const endpointRows = [
  {
    method: 'GET',
    path: '/v1/models',
    label: '模型列表',
    description: '读取当前可用模型和能力标识。',
  },
  {
    method: 'POST',
    path: '/v1/chat/completions',
    label: 'Chat Completions',
    description: 'OpenAI 兼容的文本、多模态和流式对话。',
  },
  {
    method: 'POST',
    path: '/v1/responses',
    label: 'Responses API',
    description: '适合使用 OpenAI Responses 结构的应用。',
  },
  {
    method: 'POST',
    path: '/v1/messages',
    label: 'Claude Messages',
    description: '需要 Anthropic 原生格式时使用。',
  },
]

export function DocsApiOverviewPage() {
  return (
    <DocsShell
      eyebrow='API 参考 / Reference'
      title='一个入口，多种模型协议。'
      description='默认从 OpenAI 兼容接口开始。如果你的 SDK 需要原生 Responses、Claude Messages 或 Gemini 格式，再进入对应协议页面。'
      sections={[
        { id: 'base-url', label: 'Base URL' },
        { id: 'auth', label: '认证方式' },
        { id: 'endpoints', label: '端点索引' },
        { id: 'capability', label: '能力边界' },
      ]}
    >
      <div className='space-y-12'>
        <section id='base-url' className='scroll-mt-28 space-y-5'>
          <DocsSectionTitle id='base-url'>Base URL</DocsSectionTitle>
          <DocsCodeBlock
            title='OpenAI SDK'
            language='text'
            code='https://gate.vectorepoch.com/v1'
          />
          <p className='text-sm leading-7 text-[#52525b] dark:text-[#c4c4cc]'>
            OpenAI SDK 通常需要把 <code className='font-mono text-xs'>/v1</code>{' '}
            作为 Base URL 的结尾；直接拼接具体端点时，不要重复添加{' '}
            <code className='font-mono text-xs'>/v1</code>。
          </p>
        </section>

        <section id='auth' className='scroll-mt-28 space-y-5'>
          <DocsSectionTitle id='auth'>认证方式</DocsSectionTitle>
          <div className='grid gap-3 sm:grid-cols-2'>
            <div className='rounded-xl border border-[#e4e4e7] bg-[#fafafa] p-5 dark:border-white/10 dark:bg-white/[0.035]'>
              <Badge
                variant='outline'
                className='border-[#1683c5]/40 text-[#0f6da5]'
              >
                OpenAI 兼容
              </Badge>
              <p className='mt-4 text-sm text-[#52525b] dark:text-[#c4c4cc]'>
                在 Header 中传入：
              </p>
              <code className='mt-2 block rounded-lg bg-[#f4f4f5] px-3 py-2 font-mono text-xs dark:bg-white/10'>
                Authorization: Bearer $RELAY_API_KEY
              </code>
            </div>
            <div className='rounded-xl border border-[#e4e4e7] bg-[#fafafa] p-5 dark:border-white/10 dark:bg-white/[0.035]'>
              <Badge
                variant='outline'
                className='border-[#1683c5]/40 text-[#0f6da5]'
              >
                企业 Key
              </Badge>
              <p className='mt-4 text-sm leading-6 text-[#52525b] dark:text-[#c4c4cc]'>
                Key 由控制台创建，按企业权限和策略路由到可用渠道。
              </p>
            </div>
          </div>
          <DocsCallout title='不要把上游渠道凭据发给客户端' tone='muted'>
            客户端只使用 Relay Key。OpenAI、Anthropic 或其他渠道的上游 Key
            由网关侧托管和轮换。
          </DocsCallout>
        </section>

        <section id='endpoints' className='scroll-mt-28 space-y-5'>
          <DocsSectionTitle id='endpoints'>端点索引</DocsSectionTitle>
          <div className='overflow-hidden rounded-xl border border-[#e4e4e7] dark:border-white/10'>
            <div className='grid grid-cols-[4.5rem_1fr] gap-3 border-b border-[#e4e4e7] bg-[#f4f4f5] px-4 py-3 text-xs font-semibold sm:grid-cols-[5.5rem_1fr_9rem] dark:border-white/10 dark:bg-white/[0.05]'>
              <span>方法</span>
              <span>端点</span>
              <span className='hidden sm:block'>用途</span>
            </div>
            {endpointRows.map((row) => (
              <div
                key={row.path}
                className='grid grid-cols-[4.5rem_1fr] gap-3 border-b border-[#e4e4e7] px-4 py-4 last:border-0 sm:grid-cols-[5.5rem_1fr_9rem] dark:border-white/10'
              >
                <span className='font-mono text-[0.65rem] font-semibold text-[#1683c5]'>
                  {row.method}
                </span>
                <div>
                  <code className='font-mono text-xs'>{row.path}</code>
                  <p className='mt-1 text-xs leading-5 text-[#71717a] sm:hidden'>
                    {row.description}
                  </p>
                </div>
                <div className='hidden sm:block'>
                  <span className='block text-xs font-medium'>{row.label}</span>
                  <span className='mt-1 block text-[0.68rem] leading-4 text-[#71717a]'>
                    {row.description}
                  </span>
                </div>
              </div>
            ))}
          </div>
          <Link
            to='/docs/api/chat-completions'
            className='inline-flex items-center gap-2 text-sm font-medium text-[#1683c5] hover:underline'
          >
            查看 Chat Completions 详情 <ArrowRight className='size-4' />
          </Link>
        </section>

        <section id='capability' className='scroll-mt-28 space-y-5'>
          <DocsSectionTitle id='capability'>能力边界</DocsSectionTitle>
          <div className='grid gap-3 sm:grid-cols-4'>
            <div className='rounded-xl border border-[#e4e4e7] p-4 dark:border-white/10'>
              <Braces className='size-4 text-[#1683c5]' />
              <p className='mt-3 text-xs font-semibold'>文本与多模态</p>
            </div>
            <div className='rounded-xl border border-[#e4e4e7] p-4 dark:border-white/10'>
              <Radio className='size-4 text-[#1683c5]' />
              <p className='mt-3 text-xs font-semibold'>流式输出</p>
            </div>
            <div className='rounded-xl border border-[#e4e4e7] p-4 dark:border-white/10'>
              <Image className='size-4 text-[#1683c5]' />
              <p className='mt-3 text-xs font-semibold'>图片接口</p>
            </div>
            <div className='rounded-xl border border-[#e4e4e7] p-4 dark:border-white/10'>
              <Video className='size-4 text-[#1683c5]' />
              <p className='mt-3 text-xs font-semibold'>异步任务</p>
            </div>
          </div>
          <p className='flex items-start gap-2 text-xs leading-6 text-[#71717a]'>
            <List className='mt-1 size-3.5 shrink-0 text-[#1683c5]' />
            实际可用模型以 <code className='font-mono'>
              GET /v1/models
            </code>{' '}
            返回为准；模型名称和能力会随渠道策略变化。
          </p>
        </section>
      </div>
    </DocsShell>
  )
}

export function DocsChatCompletionsPage() {
  return (
    <DocsShell
      eyebrow='API 参考 / OpenAI Compatible'
      title='Chat Completions，默认的接入起点。'
      description='适合常规聊天、流式输出和大多数 OpenAI 兼容客户端。先确认模型列表，再根据客户端需要打开多模态或工具参数。'
      sections={[
        { id: 'request', label: '请求示例' },
        { id: 'stream', label: '流式输出' },
        { id: 'notes', label: '接入注意' },
      ]}
    >
      <div className='space-y-12'>
        <section id='request' className='scroll-mt-28 space-y-5'>
          <DocsSectionTitle id='request'>请求示例</DocsSectionTitle>
          <DocsCodeBlock
            title='POST /v1/chat/completions'
            language='json'
            code={`curl https://gate.vectorepoch.com/v1/chat/completions \\
  -H "Authorization: Bearer $RELAY_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-5.1",
    "messages": [
      {"role": "user", "content": "介绍一下向量纪元 Relay"}
    ],
    "temperature": 0.2
  }'`}
          />
        </section>
        <section id='stream' className='scroll-mt-28 space-y-5'>
          <DocsSectionTitle id='stream'>流式输出</DocsSectionTitle>
          <p className='text-sm leading-7 text-[#52525b] dark:text-[#c4c4cc]'>
            将 <code className='font-mono text-xs'>stream</code> 设置为{' '}
            <code className='font-mono text-xs'>true</code>，响应会以 SSE
            事件持续返回。客户端应持续读取直到收到结束事件，并处理网络中断后的重试策略。
          </p>
          <DocsCodeBlock
            title='stream'
            language='json'
            code={`{
  "model": "gpt-5.1",
  "messages": [{"role": "user", "content": "你好"}],
  "stream": true
}`}
          />
        </section>
        <section id='notes' className='scroll-mt-28 space-y-5'>
          <DocsSectionTitle id='notes'>接入注意</DocsSectionTitle>
          <ul className='space-y-3 text-sm leading-7 text-[#52525b] dark:text-[#c4c4cc]'>
            <li className='flex gap-2'>
              <Check className='mt-1 size-4 shrink-0 text-[#2c9a70]' />
              模型名必须来自当前{' '}
              <code className='font-mono text-xs'>/v1/models</code> 列表。
            </li>
            <li className='flex gap-2'>
              <Check className='mt-1 size-4 shrink-0 text-[#2c9a70]' />
              不要把客户端超时设置得过短；长上下文和思考模型需要更长等待时间。
            </li>
            <li className='flex gap-2'>
              <Check className='mt-1 size-4 shrink-0 text-[#2c9a70]' />
              遇到 503 时先查看错误响应和系统负载，不要无间隔高频重试。
            </li>
          </ul>
          <Link
            to='/docs/troubleshooting'
            className='inline-flex items-center gap-2 text-sm font-medium text-[#1683c5] hover:underline'
          >
            查看错误排查 <ArrowRight className='size-4' />
          </Link>
        </section>
      </div>
    </DocsShell>
  )
}
