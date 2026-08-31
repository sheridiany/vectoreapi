import { Citrus } from 'lucide-react'
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
import type { ComponentType } from 'react'
import { useTranslation } from 'react-i18next'
import { FaLinkedinIn } from 'react-icons/fa6'
import {
  SiBilibili,
  SiInstagram,
  SiKuaishou,
  SiReddit,
  SiSinaweibo,
  SiTiktok,
  SiWechat,
  SiX,
  SiXiaohongshu,
  SiYoutube,
  SiZhihu,
} from 'react-icons/si'

import { AnimateInView } from '@/components/animate-in-view'

type Platform = {
  name: string
  tags: readonly string[]
  icon: ComponentType<{ className?: string }>
  color: string
}

const PLATFORM_ROWS: readonly (readonly Platform[])[] = [
  [
    {
      name: 'TikTok API',
      tags: ['Video', 'Users', 'Trending'],
      icon: SiTiktok,
      color: '#ff0050',
    },
    {
      name: 'Douyin API',
      tags: ['Video', 'Search', 'Live'],
      icon: SiTiktok,
      color: '#25f4ee',
    },
    {
      name: 'Instagram API',
      tags: ['Posts', 'Reels', 'Stories'],
      icon: SiInstagram,
      color: '#e4405f',
    },
    {
      name: 'YouTube API',
      tags: ['Video', 'Channels', 'Shorts'],
      icon: SiYoutube,
      color: '#ff0033',
    },
    {
      name: 'X / Twitter API',
      tags: ['Posts', 'Profiles', 'Media'],
      icon: SiX,
      color: '#8c8276',
    },
    {
      name: 'Rednote API',
      tags: ['Posts', 'Users', 'Comments'],
      icon: SiXiaohongshu,
      color: '#ff2442',
    },
    {
      name: 'Bilibili API',
      tags: ['Video', 'Users', 'Comments'],
      icon: SiBilibili,
      color: '#00aeec',
    },
  ],
  [
    {
      name: 'Weibo API',
      tags: ['Posts', 'Users', 'Topics'],
      icon: SiSinaweibo,
      color: '#e6162d',
    },
    {
      name: 'Kuaishou API',
      tags: ['Video', 'Users', 'Live'],
      icon: SiKuaishou,
      color: '#ff4906',
    },
    {
      name: 'WeChat API',
      tags: ['Articles', 'Video'],
      icon: SiWechat,
      color: '#07c160',
    },
    {
      name: 'Lemon8 API',
      tags: ['Posts', 'Users', 'Trending'],
      icon: Citrus,
      color: '#e4b900',
    },
    {
      name: 'Zhihu API',
      tags: ['Articles', 'Answers'],
      icon: SiZhihu,
      color: '#0066ff',
    },
    {
      name: 'Reddit API',
      tags: ['Posts', 'Comments', 'Communities'],
      icon: SiReddit,
      color: '#ff4500',
    },
    {
      name: 'LinkedIn API',
      tags: ['Profiles', 'Posts', 'Jobs'],
      icon: FaLinkedinIn,
      color: '#0a66c2',
    },
  ],
] as const

function PlatformCard({ platform }: { platform: Platform }) {
  const { t } = useTranslation()
  const PlatformIcon = platform.icon

  return (
    <article className='home-vsearch-platform-card' aria-label={platform.name}>
      <div className='flex items-start gap-4'>
        <span
          className='home-vsearch-platform-logo'
          style={{ color: platform.color }}
          aria-hidden='true'
        >
          <PlatformIcon className='size-6' />
        </span>
        <div className='min-w-0'>
          <h3 className='truncate text-base font-semibold tracking-[-0.02em] text-[#211d18] dark:text-[#f5f1e9]'>
            {platform.name}
          </h3>
          <p className='mt-0.5 truncate text-xs text-[#91887d] dark:text-white/40'>
            {platform.tags.map((tag) => t(tag)).join(' · ')}
          </p>
        </div>
      </div>
    </article>
  )
}

export function VSearchCapabilities() {
  const { t } = useTranslation()

  return (
    <section
      id='vsearch'
      className='border-border/70 relative scroll-mt-28 overflow-hidden border-b bg-[#f6f6f5] py-20 md:py-28 dark:bg-[#11110f]'
    >
      <div
        aria-hidden='true'
        className='home-hero-diagonal-grid pointer-events-none absolute inset-[-28%] opacity-30 dark:opacity-15'
      />
      <div
        aria-hidden='true'
        className='pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_50%_48%,rgba(151,113,255,0.13),transparent_35%)] dark:bg-[radial-gradient(circle_at_50%_48%,rgba(183,121,31,0.10),transparent_38%)]'
      />

      <div className='relative'>
        <AnimateInView className='mx-auto max-w-4xl px-4 text-center sm:px-6'>
          <p className='text-[10px] font-semibold tracking-[0.2em] text-[#a46d1c] uppercase dark:text-amber-300'>
            {t('vSearch by Vector Epoch')}
          </p>
          <h2
            className='mt-5 font-serif text-[clamp(2.7rem,5vw,5rem)] leading-[0.98] font-medium tracking-[-0.045em] text-balance text-[#211d18] dark:text-[#f5f1e9]'
            aria-label={`${t('Real-time social data,')} ${t('built for intelligent agents.')}`}
          >
            <span className='block'>{t('Real-time social data,')}</span>
            <span className='block text-[#c87326] dark:text-orange-300'>
              {t('built for intelligent agents.')}
            </span>
          </h2>
          <p className='mx-auto mt-6 max-w-2xl text-sm leading-7 text-[#6f675d] md:text-base dark:text-white/60'>
            {t(
              'Unify public videos, profiles, comments, trends, and communities into stable data capabilities for intelligent agents.'
            )}
          </p>
        </AnimateInView>

        <AnimateInView delay={100} className='mt-14'>
          <p className='mb-6 text-center text-xs font-medium tracking-[0.12em] text-[#8a8176] uppercase dark:text-white/45'>
            {t('Supported social data platforms')}
          </p>
          <div className='home-vsearch-platform-viewport space-y-4'>
            {PLATFORM_ROWS.map((platforms, rowIndex) => (
              <div
                key={platforms[0].name}
                className={`home-vsearch-platform-track ${
                  rowIndex === 0
                    ? 'home-vsearch-platform-track--left'
                    : 'home-vsearch-platform-track--right'
                }`}
              >
                {[0, 1].map((copyIndex) => (
                  <div
                    key={copyIndex}
                    className='home-vsearch-platform-set'
                    aria-hidden={copyIndex === 1}
                  >
                    {platforms.map((platform) => (
                      <PlatformCard key={platform.name} platform={platform} />
                    ))}
                  </div>
                ))}
              </div>
            ))}
          </div>
        </AnimateInView>
      </div>
    </section>
  )
}
