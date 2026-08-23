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
import { ArrowRight, Check, KeyRound, LockKeyhole } from 'lucide-react'

import {
  DocsCallout,
  DocsCodeBlock,
  DocsShell,
  DocsStep,
} from '../components/docs-shell'

export function DocsGettingStartedPage() {
  return (
    <DocsShell
      eyebrow='开始使用 / Quick start'
      title='五分钟完成第一次调用。'
      description='这页只解决三件事：创建访问 Key、确认 Base URL、发出一条最小请求。完成后再去选择客户端或协议。'
      sections={[
        { id: 'prepare', label: '准备工作' },
        { id: 'steps', label: '三步接入' },
        { id: 'verify', label: '验证响应' },
      ]}
    >
      <div className='space-y-12'>
        <section id='prepare' className='scroll-mt-28 space-y-5'>
          <div className='grid gap-3 sm:grid-cols-2'>
            <div className='rounded-xl border border-[#e4e4e7] bg-[#fafafa] p-5 dark:border-white/10 dark:bg-white/[0.035]'>
              <KeyRound className='size-5 text-[#1683c5]' />
              <h2 className='mt-5 text-base font-semibold'>一个企业访问 Key</h2>
              <p className='mt-2 text-sm leading-6 text-[#71717a] dark:text-[#a1a1aa]'>
                在控制台的 API 密钥页面创建。不要把管理员密码、渠道 Key
                当成客户端 Key 使用。
              </p>
            </div>
            <div className='rounded-xl border border-[#e4e4e7] bg-[#fafafa] p-5 dark:border-white/10 dark:bg-white/[0.035]'>
              <LockKeyhole className='size-5 text-[#1683c5]' />
              <h2 className='mt-5 text-base font-semibold'>
                一个稳定的 Base URL
              </h2>
              <p className='mt-2 text-sm leading-6 text-[#71717a] dark:text-[#a1a1aa]'>
                OpenAI 兼容 SDK 使用{' '}
                <code className='font-mono text-xs'>
                  https://gate.vectorepoch.com/v1
                </code>{' '}
                作为 Base URL。
              </p>
            </div>
          </div>
          <DocsCallout title='Key 只在创建时完整展示'>
            Key
            属于企业访问凭证。请保存到密码管理器或服务端密钥系统，不要提交到代码仓库，也不要截图发到群里。
          </DocsCallout>
        </section>

        <section id='steps' className='scroll-mt-28 space-y-8'>
          <h2
            id='steps-title'
            className='border-t border-[#e4e4e7] pt-8 text-2xl font-semibold tracking-[-0.025em] dark:border-white/10'
          >
            三步接入
          </h2>
          <div className='space-y-8'>
            <DocsStep number='01' title='登录控制台并选择企业'>
              企业成员使用自己的账号登录。企业管理员可以在企业管理中查看成员、Key
              和用量；普通成员只使用分配给自己的权限范围。
            </DocsStep>
            <DocsStep number='02' title='创建或领取访问 Key'>
              打开“API 密钥”，创建一枚用于当前应用的 Key。建议按应用命名，例如{' '}
              <code className='font-mono text-xs'>marketing-bot-prod</code>
              ，方便后续审计和停用。
            </DocsStep>
            <DocsStep number='03' title='发送一条最小请求'>
              先用短文本、低并发请求验证链路；确认返回后，再逐步打开流式输出、工具调用、视觉输入或更长上下文。
            </DocsStep>
          </div>
        </section>

        <section id='verify' className='scroll-mt-28 space-y-5'>
          <h2 className='border-t border-[#e4e4e7] pt-8 text-2xl font-semibold tracking-[-0.025em] dark:border-white/10'>
            验证响应
          </h2>
          <DocsCodeBlock
            language='bash'
            title='curl'
            code={`export RELAY_API_KEY="replace-with-your-key"

curl https://gate.vectorepoch.com/v1/chat/completions \\
  -H "Authorization: Bearer $RELAY_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-5.1",
    "messages": [{"role":"user","content":"只回复：接入成功"}],
    "stream": false
  }'`}
          />
          <div className='grid gap-3 text-sm sm:grid-cols-3'>
            <div className='rounded-xl border border-[#e4e4e7] p-4 dark:border-white/10'>
              <Check className='mb-2 size-4 text-[#2c9a70]' />
              <strong>200</strong>
              <p className='mt-1 text-xs leading-5 text-[#71717a]'>
                请求成功，查看返回内容。
              </p>
            </div>
            <div className='rounded-xl border border-[#e4e4e7] p-4 dark:border-white/10'>
              <Check className='mb-2 size-4 text-[#1683c5]' />
              <strong>401 / 403</strong>
              <p className='mt-1 text-xs leading-5 text-[#71717a]'>
                先检查 Key、企业权限和状态。
              </p>
            </div>
            <div className='rounded-xl border border-[#e4e4e7] p-4 dark:border-white/10'>
              <Check className='mb-2 size-4 text-[#1683c5]' />
              <strong>429 / 503</strong>
              <p className='mt-1 text-xs leading-5 text-[#71717a]'>
                查看限流、渠道和系统负载。
              </p>
            </div>
          </div>
          <Link
            to='/docs/guide'
            className='inline-flex items-center gap-2 text-sm font-medium text-[#1683c5] hover:underline'
          >
            继续选择你的接入方式 <ArrowRight className='size-4' />
          </Link>
        </section>
      </div>
    </DocsShell>
  )
}
