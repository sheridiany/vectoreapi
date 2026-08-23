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
import { ArrowRight, Bot, Code2, Monitor, Smartphone } from 'lucide-react'

import {
  DocsCard,
  DocsCallout,
  DocsShell,
  DocsSectionTitle,
} from '../components/docs-shell'

const guideLinks = [
  {
    to: '/docs/guide/registration' as const,
    icon: Code2,
    title: '创建访问 Key',
    description: '了解企业 Key、权限边界、命名方式和轮换建议。',
  },
  {
    to: '/docs/guide/chat-clients' as const,
    icon: Monitor,
    title: '聊天客户端',
    description: '配置桌面端、IDE 插件和移动端客户端。',
  },
]

export function DocsGuidePage() {
  return (
    <DocsShell
      eyebrow='接入指南 / Guide'
      title='按你要做的事情，选择下一步。'
      description='文档不要求从头读到尾。先判断你是配置一个客户端、接入服务端，还是排查一条错误，再进入对应页面。'
      sections={[
        { id: 'choose', label: '选择入口' },
        { id: 'principles', label: '接入原则' },
        { id: 'support', label: '需要协助时' },
      ]}
    >
      <div className='space-y-12'>
        <section id='choose' className='scroll-mt-28 space-y-5'>
          <DocsSectionTitle id='choose'>选择入口</DocsSectionTitle>
          <div className='grid gap-3 sm:grid-cols-2'>
            {guideLinks.map((item) => {
              const Icon = item.icon
              return (
                <Link
                  key={item.to}
                  to={item.to}
                  className='group rounded-xl border border-[#e4e4e7] bg-[#fafafa] p-5 transition-colors hover:border-[#a1a1aa] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#1683c5] dark:border-white/10 dark:bg-white/[0.035] dark:hover:border-white/25'
                >
                  <div className='flex items-start justify-between'>
                    <span className='grid size-9 place-items-center rounded-lg bg-[#f4f4f5] text-[#1683c5] dark:bg-white/[0.08]'>
                      <Icon className='size-4.5' />
                    </span>
                    <ArrowRight className='size-4 text-[#a1a1aa] transition-transform group-hover:translate-x-1 group-hover:text-[#1683c5]' />
                  </div>
                  <h3 className='mt-8 text-lg font-semibold'>{item.title}</h3>
                  <p className='mt-2 text-sm leading-6 text-[#71717a] dark:text-[#a1a1aa]'>
                    {item.description}
                  </p>
                </Link>
              )
            })}
          </div>
          <div className='rounded-2xl border border-dashed border-[#cfc4b5] p-5 dark:border-white/15'>
            <div className='flex items-center gap-3'>
              <Bot className='size-5 text-[#1683c5]' />
              <div>
                <h3 className='text-sm font-semibold'>服务端开发者</h3>
                <p className='mt-1 text-xs text-[#71717a]'>
                  直接查看{' '}
                  <Link
                    to='/docs/api'
                    className='text-[#1683c5] hover:underline'
                  >
                    API 总览
                  </Link>
                  ，从协议和最小请求开始。
                </p>
              </div>
            </div>
          </div>
        </section>

        <section id='principles' className='scroll-mt-28 space-y-5'>
          <DocsSectionTitle id='principles'>接入原则</DocsSectionTitle>
          <div className='grid gap-3 sm:grid-cols-3'>
            <DocsCard>
              <span className='font-mono text-xs text-[#1683c5]'>01</span>
              <h3 className='mt-4 text-sm font-semibold'>先小后大</h3>
              <p className='mt-2 text-xs leading-5 text-[#71717a]'>
                先用短文本验证 Key、模型和返回格式，再接入长上下文。
              </p>
            </DocsCard>
            <DocsCard>
              <span className='font-mono text-xs text-[#1683c5]'>02</span>
              <h3 className='mt-4 text-sm font-semibold'>先流式</h3>
              <p className='mt-2 text-xs leading-5 text-[#71717a]'>
                交互式应用优先开启流式，降低首字延迟和网关超时风险。
              </p>
            </DocsCard>
            <DocsCard>
              <span className='font-mono text-xs text-[#1683c5]'>03</span>
              <h3 className='mt-4 text-sm font-semibold'>留痕迹</h3>
              <p className='mt-2 text-xs leading-5 text-[#71717a]'>
                生产环境按应用拆分 Key，便于企业管理员看用量和停用。
              </p>
            </DocsCard>
          </div>
        </section>

        <section id='support' className='scroll-mt-28 space-y-5'>
          <DocsSectionTitle id='support'>需要协助时</DocsSectionTitle>
          <DocsCallout tone='muted' title='一次把上下文带齐'>
            请提供客户端名称、Base
            URL、模型名、是否流式、完整报错和大致任务类型。不要发送完整 API
            Key；保留前后几位即可。
          </DocsCallout>
          <Link
            to='/docs/troubleshooting'
            className='inline-flex items-center gap-2 text-sm font-medium text-[#1683c5] hover:underline'
          >
            先看常见错误排查 <ArrowRight className='size-4' />
          </Link>
        </section>
      </div>
    </DocsShell>
  )
}

export function DocsRegistrationPage() {
  return (
    <DocsShell
      eyebrow='接入指南 / Access Key'
      title='把 Key 当作企业边界来管理。'
      description='一个 Key 对应一个真实的使用场景。命名、权限和轮换做好，管理员才能看懂用量，也能在出现异常时快速止损。'
      sections={[
        { id: 'create', label: '创建建议' },
        { id: 'rotate', label: '轮换与停用' },
      ]}
    >
      <div className='space-y-12'>
        <section id='create' className='scroll-mt-28 space-y-5'>
          <DocsSectionTitle id='create'>创建建议</DocsSectionTitle>
          <div className='space-y-3 text-sm leading-7 text-[#52525b] dark:text-[#c4c4cc]'>
            <p>
              在控制台进入“API 密钥”，为应用创建独立 Key。建议使用{' '}
              <code className='font-mono text-xs'>team-app-env</code>{' '}
              的命名方式，例如{' '}
              <code className='font-mono text-xs'>product-assistant-prod</code>
              。
            </p>
            <p>
              同一个企业下，普通成员只能看到自己被授权的
              Key；企业管理员可以从企业用量、成员和审计页面追踪使用情况。
            </p>
          </div>
          <DocsCallout title='不要共享管理员账号'>
            管理员账号用于成员、策略和审计管理，不应该直接填进客户端。客户端统一使用企业访问
            Key。
          </DocsCallout>
        </section>
        <section id='rotate' className='scroll-mt-28 space-y-5'>
          <DocsSectionTitle id='rotate'>轮换与停用</DocsSectionTitle>
          <ol className='space-y-4 text-sm leading-7 text-[#52525b] dark:text-[#c4c4cc]'>
            <li className='flex gap-3'>
              <span className='font-mono text-xs text-[#1683c5]'>01</span>
              <span>先创建新 Key，并在服务端配置中完成切换。</span>
            </li>
            <li className='flex gap-3'>
              <span className='font-mono text-xs text-[#1683c5]'>02</span>
              <span>发送一条最小请求，确认新 Key 已经生效。</span>
            </li>
            <li className='flex gap-3'>
              <span className='font-mono text-xs text-[#1683c5]'>03</span>
              <span>确认旧 Key 没有流量后再停用，保留审计记录。</span>
            </li>
          </ol>
        </section>
      </div>
    </DocsShell>
  )
}

export function DocsChatClientsPage() {
  return (
    <DocsShell
      eyebrow='接入指南 / Clients'
      title='客户端只需要三项配置。'
      description='无论是桌面聊天、IDE 插件还是移动端，先填 Base URL、API Key 和模型名。其余选项按客户端能力逐步打开。'
      sections={[
        { id: 'fields', label: '三项配置' },
        { id: 'clients', label: '客户端路径' },
      ]}
    >
      <div className='space-y-12'>
        <section id='fields' className='scroll-mt-28 space-y-5'>
          <DocsSectionTitle id='fields'>三项配置</DocsSectionTitle>
          <div className='overflow-hidden rounded-xl border border-[#e4e4e7] dark:border-white/10'>
            <div className='grid grid-cols-[9rem_1fr] border-b border-[#e4e4e7] bg-[#f4f4f5] px-4 py-3 text-xs font-semibold dark:border-white/10 dark:bg-white/[0.05]'>
              <span>配置项</span>
              <span>填写内容</span>
            </div>
            <div className='grid grid-cols-[9rem_1fr] border-b border-[#e4e4e7] px-4 py-4 text-sm dark:border-white/10'>
              <code className='font-mono text-xs text-[#1683c5]'>Base URL</code>
              <span className='text-[#52525b] dark:text-[#c4c4cc]'>
                https://gate.vectorepoch.com/v1
              </span>
            </div>
            <div className='grid grid-cols-[9rem_1fr] border-b border-[#e4e4e7] px-4 py-4 text-sm dark:border-white/10'>
              <code className='font-mono text-xs text-[#1683c5]'>API Key</code>
              <span className='text-[#52525b] dark:text-[#c4c4cc]'>
                控制台创建的企业访问 Key
              </span>
            </div>
            <div className='grid grid-cols-[9rem_1fr] px-4 py-4 text-sm'>
              <code className='font-mono text-xs text-[#1683c5]'>Model</code>
              <span className='text-[#52525b] dark:text-[#c4c4cc]'>
                从 <code className='font-mono text-xs'>GET /v1/models</code>{' '}
                返回中选择
              </span>
            </div>
          </div>
        </section>
        <section id='clients' className='scroll-mt-28 space-y-5'>
          <DocsSectionTitle id='clients'>客户端路径</DocsSectionTitle>
          <div className='grid gap-3 sm:grid-cols-3'>
            <DocsCard>
              <Monitor className='size-5 text-[#1683c5]' />
              <h3 className='mt-4 text-sm font-semibold'>桌面端 / IDE</h3>
              <p className='mt-2 text-xs leading-5 text-[#71717a]'>
                优先使用 OpenAI Compatible / OpenAI API 类型。
              </p>
            </DocsCard>
            <DocsCard>
              <Smartphone className='size-5 text-[#1683c5]' />
              <h3 className='mt-4 text-sm font-semibold'>移动端</h3>
              <p className='mt-2 text-xs leading-5 text-[#71717a]'>
                填写同一组 Base URL 和 Key，关闭不必要的高级参数。
              </p>
            </DocsCard>
            <DocsCard>
              <Code2 className='size-5 text-[#1683c5]' />
              <h3 className='mt-4 text-sm font-semibold'>服务端 SDK</h3>
              <p className='mt-2 text-xs leading-5 text-[#71717a]'>
                使用 OpenAI SDK 时，将 Base URL 指向 Relay 的{' '}
                <code className='font-mono'>/v1</code>。
              </p>
            </DocsCard>
          </div>
        </section>
      </div>
    </DocsShell>
  )
}
