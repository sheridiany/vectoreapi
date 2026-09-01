/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { Check, KeyRound, Search, ShieldCheck, Workflow } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'

import {
  DocsCallout,
  DocsCodeBlock,
  DocsShell,
  DocsSectionTitle,
} from '../components/docs-shell'

const platforms = [
  'Bilibili',
  'Douyin',
  'Instagram',
  'Kuaishou',
  'LinkedIn',
  'Reddit',
  'TikTok',
  'TikTok Shop',
  'X / Twitter',
  'WeChat Channels',
  'WeChat MP',
  'Weibo',
  'Xiaohongshu',
  'YouTube',
  'Zhihu',
] as const

const capabilityKeys = [
  'Social account details',
  'Public account content',
  'Social content details',
  'Social content search',
  'Content comments',
  'Comment replies',
  'Platform trends',
  'Product search',
  'Product details',
  'Product reviews',
  'Job search',
  'Job details',
] as const

const workflowSteps = [
  {
    title: 'Discover a capability',
    description:
      'Find the matching serviceId from a complete natural-language request.',
  },
  {
    title: 'Describe exact parameters',
    description:
      'Read the current input schema, supported platforms, and price before execution.',
  },
  {
    title: 'Execute the capability',
    description:
      'Call the selected serviceId with canonical parameters and an idempotency key.',
  },
] as const

export function DocsVSearchPage() {
  const { t } = useTranslation()

  return (
    <DocsShell
      eyebrow={t('API reference / vSearch')}
      title={t('Search public data through one stable contract.')}
      description={t(
        'vSearch standardizes public web, social, commerce, and job data without exposing upstream providers to your application.'
      )}
      sections={[
        { id: 'quick-start', label: t('Quick start') },
        { id: 'platforms', label: t('Supported platforms') },
        { id: 'capabilities', label: t('Standard capabilities') },
        { id: 'workflow', label: t('Call workflow') },
        { id: 'safety', label: t('Security and billing') },
      ]}
    >
      <div className='space-y-12'>
        <section id='quick-start' className='scroll-mt-28 space-y-5'>
          <DocsSectionTitle id='quick-start'>
            {t('Quick start')}
          </DocsSectionTitle>
          <div className='grid gap-3 sm:grid-cols-2'>
            <div className='rounded-xl border border-[#e4e4e7] p-5 dark:border-white/10'>
              <KeyRound className='size-5 text-[#1683c5]' aria-hidden='true' />
              <p className='mt-4 text-sm font-semibold'>
                {t('Create a dedicated vSearch key')}
              </p>
              <p className='mt-2 text-sm leading-6 text-[#52525b] dark:text-[#c4c4cc]'>
                {t(
                  'Create the key from the vSearch key page. Model API keys cannot call vSearch.'
                )}
              </p>
            </div>
            <div className='rounded-xl border border-[#e4e4e7] p-5 dark:border-white/10'>
              <Workflow className='size-5 text-[#1683c5]' aria-hidden='true' />
              <p className='mt-4 text-sm font-semibold'>
                {t('Use the MCP endpoint')}
              </p>
              <p className='mt-2 text-sm leading-6 text-[#52525b] dark:text-[#c4c4cc]'>
                {t(
                  'Send JSON-RPC requests to the single endpoint and keep the full key on the server.'
                )}
              </p>
            </div>
          </div>
          <DocsCodeBlock
            title='Endpoint'
            language='text'
            code={`POST https://gate.vectorepoch.com/v1/mcp
Authorization: Bearer $VSEARCH_KEY
Content-Type: application/json`}
          />
          <DocsCallout title={t('Separate authorization')} tone='muted'>
            {t(
              'A vSearch key starts with vr_live_ and is separate from the model Relay API key.'
            )}
          </DocsCallout>
        </section>

        <section id='platforms' className='scroll-mt-28 space-y-5'>
          <DocsSectionTitle id='platforms'>
            {t('Supported platforms')}
          </DocsSectionTitle>
          <p className='text-sm leading-7 text-[#52525b] dark:text-[#c4c4cc]'>
            {t(
              '15 platforms and 86 verified interfaces are currently available.'
            )}
          </p>
          <div className='flex flex-wrap gap-2'>
            {platforms.map((platform) => (
              <Badge
                key={platform}
                variant='outline'
                className='rounded-full px-3 py-1.5'
              >
                {platform}
              </Badge>
            ))}
          </div>
        </section>

        <section id='capabilities' className='scroll-mt-28 space-y-5'>
          <DocsSectionTitle id='capabilities'>
            {t('Standard capabilities')}
          </DocsSectionTitle>
          <p className='text-sm leading-7 text-[#52525b] dark:text-[#c4c4cc]'>
            {t(
              '12 stable capabilities hide provider-specific paths and response formats.'
            )}
          </p>
          <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-3'>
            {capabilityKeys.map((capability) => (
              <div
                key={capability}
                className='flex items-center gap-3 rounded-xl border border-[#e4e4e7] p-4 dark:border-white/10'
              >
                <Check
                  className='size-4 shrink-0 text-[#2c9a70]'
                  aria-hidden='true'
                />
                <span className='text-sm font-medium'>{t(capability)}</span>
              </div>
            ))}
          </div>
        </section>

        <section id='workflow' className='scroll-mt-28 space-y-5'>
          <DocsSectionTitle id='workflow'>
            {t('Call workflow')}
          </DocsSectionTitle>
          <div className='grid gap-3 sm:grid-cols-3'>
            {workflowSteps.map((step, index) => (
              <div
                key={step.title}
                className='rounded-xl border border-[#e4e4e7] p-5 dark:border-white/10'
              >
                <span className='font-mono text-xs text-[#1683c5]'>
                  0{index + 1}
                </span>
                <p className='mt-3 text-sm font-semibold'>{t(step.title)}</p>
                <p className='mt-2 text-xs leading-6 text-[#71717a]'>
                  {t(step.description)}
                </p>
              </div>
            ))}
          </div>
          <DocsCodeBlock
            title='1. vector_relay_find_tools'
            language='json'
            code={`{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "vector_relay_find_tools",
    "arguments": {"query": "搜索 YouTube 上关于人工智能的视频"}
  }
}`}
          />
          <DocsCodeBlock
            title='2. vector_relay_describe_tool'
            language='json'
            code={`{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "vector_relay_describe_tool",
    "arguments": {"serviceId": "vr_svc_xxxxxxxxxxxxxxxx"}
  }
}`}
          />
          <DocsCodeBlock
            title='3. vector_relay_call'
            language='json'
            code={`curl https://gate.vectorepoch.com/v1/mcp \\
  -H "Authorization: Bearer $VSEARCH_KEY" \\
  -H "Content-Type: application/json" \\
  -H "Idempotency-Key: your-unique-request-id" \\
  -d '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "tools/call",
    "params": {
      "name": "vector_relay_call",
      "arguments": {
        "serviceId": "vr_svc_xxxxxxxxxxxxxxxx",
        "params": {"platform": "youtube", "query": "artificial intelligence"}
      }
    }
  }'`}
          />
          <DocsCallout
            title={t('Do not hard-code serviceId values')}
            tone='muted'
          >
            {t(
              'Always discover and describe before calling because the published catalog is the source of truth.'
            )}
          </DocsCallout>
        </section>

        <section id='safety' className='scroll-mt-28 space-y-5'>
          <DocsSectionTitle id='safety'>
            {t('Security and billing')}
          </DocsSectionTitle>
          <div className='grid gap-3 sm:grid-cols-2'>
            <div className='rounded-xl border border-[#e4e4e7] p-5 dark:border-white/10'>
              <ShieldCheck
                className='size-5 text-[#2c9a70]'
                aria-hidden='true'
              />
              <p className='mt-4 text-sm font-semibold'>
                {t('Upstream details stay private')}
              </p>
              <p className='mt-2 text-sm leading-6 text-[#52525b] dark:text-[#c4c4cc]'>
                {t(
                  'Provider names, credentials, paths, and raw provider errors are not returned to vSearch users.'
                )}
              </p>
            </div>
            <div className='rounded-xl border border-[#e4e4e7] p-5 dark:border-white/10'>
              <Search className='size-5 text-[#1683c5]' aria-hidden='true' />
              <p className='mt-4 text-sm font-semibold'>
                {t('Usage is auditable')}
              </p>
              <p className='mt-2 text-sm leading-6 text-[#52525b] dark:text-[#c4c4cc]'>
                {t(
                  'Each call records status, latency, charge, and the vSearch key without storing the full secret.'
                )}
              </p>
            </div>
          </div>
          <ul className='space-y-3 text-sm leading-7 text-[#52525b] dark:text-[#c4c4cc]'>
            <li className='flex gap-2'>
              <Check
                className='mt-1 size-4 shrink-0 text-[#2c9a70]'
                aria-hidden='true'
              />
              {t('Use a unique Idempotency-Key for every logical execution.')}
            </li>
            <li className='flex gap-2'>
              <Check
                className='mt-1 size-4 shrink-0 text-[#2c9a70]'
                aria-hidden='true'
              />
              {t(
                'Retry only after checking whether the previous request reached a terminal state.'
              )}
            </li>
            <li className='flex gap-2'>
              <Check
                className='mt-1 size-4 shrink-0 text-[#2c9a70]'
                aria-hidden='true'
              />
              {t(
                'Review the live catalog and vSearch logs when an interface becomes unavailable.'
              )}
            </li>
          </ul>
        </section>
      </div>
    </DocsShell>
  )
}
