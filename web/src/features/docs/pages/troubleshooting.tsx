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
  AlertTriangle,
  ArrowRight,
  CheckCircle2,
  ClipboardList,
} from 'lucide-react'

import {
  DocsCallout,
  DocsShell,
  DocsSectionTitle,
} from '../components/docs-shell'

const errors = [
  {
    code: '401',
    title: 'Key 无效或未传入',
    reason: 'Authorization Header 缺失、Key 粘贴不完整、Key 已停用。',
    action: '重新复制企业访问 Key，并确认使用 Bearer 格式。',
  },
  {
    code: '403',
    title: '没有权限使用当前资源',
    reason: '企业策略、模型权限或成员状态不允许当前请求。',
    action: '让企业管理员确认成员、Key 和模型策略。',
  },
  {
    code: '429',
    title: '触发限流或并发上限',
    reason: '短时间请求过多，或企业 / Key / 上游渠道达到并发边界。',
    action: '降低并发，使用指数退避，并检查是否存在重复重试。',
  },
  {
    code: '503',
    title: '当前没有可用渠道或系统过载',
    reason: '网关正在保护系统，或目标模型暂时没有可调度的渠道。',
    action: '等待后重试；持续出现时记录时间、模型和完整响应体。',
  },
]

export function DocsTroubleshootingPage() {
  return (
    <DocsShell
      eyebrow='问题排查 / Troubleshooting'
      title='先看现象，再定位链路。'
      description='错误码只告诉你结果，排查需要把客户端、Key、模型、请求规模和网关状态放在一起看。'
      sections={[
        { id: 'first', label: '先做三件事' },
        { id: 'errors', label: '状态码速查' },
        { id: 'report', label: '反馈时带上这些' },
      ]}
    >
      <div className='space-y-12'>
        <section id='first' className='scroll-mt-28 space-y-5'>
          <DocsSectionTitle id='first'>先做三件事</DocsSectionTitle>
          <div className='grid gap-3 sm:grid-cols-3'>
            <div className='rounded-xl border border-[#e4e4e7] bg-[#fafafa] p-5 dark:border-white/10 dark:bg-white/[0.035]'>
              <span className='font-mono text-xs text-[#1683c5]'>01</span>
              <h3 className='mt-4 text-sm font-semibold'>重试一次</h3>
              <p className='mt-2 text-xs leading-5 text-[#71717a]'>
                先排除一次性的网络抖动或上游瞬态异常。
              </p>
            </div>
            <div className='rounded-xl border border-[#e4e4e7] bg-[#fafafa] p-5 dark:border-white/10 dark:bg-white/[0.035]'>
              <span className='font-mono text-xs text-[#1683c5]'>02</span>
              <h3 className='mt-4 text-sm font-semibold'>新建请求</h3>
              <p className='mt-2 text-xs leading-5 text-[#71717a]'>
                避免旧会话上下文、失效连接或客户端缓存干扰。
              </p>
            </div>
            <div className='rounded-xl border border-[#e4e4e7] bg-[#fafafa] p-5 dark:border-white/10 dark:bg-white/[0.035]'>
              <span className='font-mono text-xs text-[#1683c5]'>03</span>
              <h3 className='mt-4 text-sm font-semibold'>缩小请求</h3>
              <p className='mt-2 text-xs leading-5 text-[#71717a]'>
                先用短文本、单张图片和较小输出验证链路。
              </p>
            </div>
          </div>
        </section>

        <section id='errors' className='scroll-mt-28 space-y-5'>
          <DocsSectionTitle id='errors'>状态码速查</DocsSectionTitle>
          <div className='space-y-3'>
            {errors.map((error) => (
              <div
                key={error.code}
                className='grid gap-4 rounded-xl border border-[#e4e4e7] bg-[#fafafa] p-5 sm:grid-cols-[4rem_1fr_auto] sm:items-start dark:border-white/10 dark:bg-white/[0.035]'
              >
                <span className='inline-flex w-fit items-center gap-2 rounded-lg bg-[#f4f4f5] px-2.5 py-1 font-mono text-xs font-semibold dark:bg-white/10'>
                  <AlertTriangle className='size-3.5 text-[#1683c5]' />
                  {error.code}
                </span>
                <div>
                  <h3 className='text-sm font-semibold'>{error.title}</h3>
                  <p className='mt-1.5 text-xs leading-5 text-[#71717a]'>
                    {error.reason}
                  </p>
                </div>
                <p className='text-xs leading-5 text-[#52525b] sm:max-w-[13rem] dark:text-[#c4c4cc]'>
                  {error.action}
                </p>
              </div>
            ))}
          </div>
          <DocsCallout title='503 不是“Key 一定错了”'>
            503 可能来自网关保护、系统 CPU
            过载、上游暂时不可用或没有可调度渠道。请优先记录响应体中的错误码、时间和模型。
          </DocsCallout>
        </section>

        <section id='report' className='scroll-mt-28 space-y-5'>
          <DocsSectionTitle id='report'>反馈时带上这些</DocsSectionTitle>
          <div className='rounded-xl border border-[#e4e4e7] bg-[#fafafa] p-5 dark:border-white/10 dark:bg-white/[0.035]'>
            <div className='mb-4 flex items-center gap-3'>
              <ClipboardList className='size-5 text-[#1683c5]' />
              <h3 className='text-sm font-semibold'>最小排查上下文</h3>
            </div>
            <ul className='grid gap-3 text-sm leading-6 text-[#52525b] sm:grid-cols-2 dark:text-[#c4c4cc]'>
              <li className='flex gap-2'>
                <CheckCircle2 className='mt-1 size-4 shrink-0 text-[#2c9a70]' />
                客户端名称与设备
              </li>
              <li className='flex gap-2'>
                <CheckCircle2 className='mt-1 size-4 shrink-0 text-[#2c9a70]' />
                Base URL 与模型名
              </li>
              <li className='flex gap-2'>
                <CheckCircle2 className='mt-1 size-4 shrink-0 text-[#2c9a70]' />
                是否使用流式输出
              </li>
              <li className='flex gap-2'>
                <CheckCircle2 className='mt-1 size-4 shrink-0 text-[#2c9a70]' />
                完整报错文本或截图
              </li>
            </ul>
          </div>
          <Link
            to='/docs/api'
            className='inline-flex items-center gap-2 text-sm font-medium text-[#1683c5] hover:underline'
          >
            回到 API 总览 <ArrowRight className='size-4' />
          </Link>
        </section>
      </div>
    </DocsShell>
  )
}
