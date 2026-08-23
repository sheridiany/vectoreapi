import { Link, useLocation } from '@tanstack/react-router'
/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import {
  ArrowUpRight,
  Check,
  ChevronDown,
  ChevronRight,
  Copy,
  Menu,
  PanelLeftClose,
  Search,
  X,
} from 'lucide-react'
import { useEffect, useMemo, useState, type ReactNode } from 'react'

import { ThemeSwitch } from '@/components/theme-switch'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { cn } from '@/lib/utils'

import { docsNavGroups, filterDocsSearchItems } from '../lib/search'

export type DocsSection = {
  id: string
  label: string
}

type DocsShellProps = {
  children: ReactNode
  eyebrow: string
  title: string
  description: string
  sections?: readonly DocsSection[]
}

function PageActions() {
  const [copied, setCopied] = useState(false)

  const copyLink = async () => {
    try {
      await navigator.clipboard.writeText(window.location.href)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1600)
    } catch {
      setCopied(false)
    }
  }

  return (
    <div className='mt-7 flex flex-wrap items-center gap-2'>
      <Button
        type='button'
        variant='outline'
        className='h-9 gap-2 rounded-lg px-3 text-sm'
        onClick={copyLink}
      >
        {copied ? <Check className='size-4' /> : <Copy className='size-4' />}
        {copied ? '已复制链接' : '复制链接'}
      </Button>
      <DropdownMenu modal={false}>
        <DropdownMenuTrigger
          render={
            <Button
              type='button'
              variant='outline'
              className='h-9 gap-2 rounded-lg px-3 text-sm'
            />
          }
        >
          打开
          <ChevronDown className='size-4' />
        </DropdownMenuTrigger>
        <DropdownMenuContent align='start' className='min-w-40'>
          <DropdownMenuItem
            onClick={() =>
              window.open(window.location.href, '_blank', 'noopener,noreferrer')
            }
          >
            新标签页打开
            <ArrowUpRight className='ms-auto size-3.5' />
          </DropdownMenuItem>
          <DropdownMenuItem render={<Link to='/dashboard' />}>
            进入控制台
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}

export function DocsShell(props: DocsShellProps) {
  const location = useLocation()
  const [mobileOpen, setMobileOpen] = useState(false)
  const [searchOpen, setSearchOpen] = useState(false)
  const [searchValue, setSearchValue] = useState('')
  const [openGroups, setOpenGroups] = useState<Record<string, boolean>>({})

  useEffect(() => {
    setMobileOpen(false)
  }, [location.pathname])

  useEffect(() => {
    const openSearch = () => setSearchOpen(true)
    const handleKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault()
        setSearchOpen(true)
      }
      if (event.key === 'Escape') setSearchOpen(false)
    }

    window.addEventListener('docs:open-search', openSearch)
    window.addEventListener('keydown', handleKeyDown)
    return () => {
      window.removeEventListener('docs:open-search', openSearch)
      window.removeEventListener('keydown', handleKeyDown)
    }
  }, [])

  const filteredSearchItems = useMemo(() => {
    return filterDocsSearchItems(searchValue)
  }, [searchValue])

  const closeSearch = () => {
    setSearchOpen(false)
    setSearchValue('')
  }

  const currentPath = location.pathname.replace(/\/$/, '') || '/'
  const activeGroup = docsNavGroups.find((group) =>
    group.items.some((item) => item.to === currentPath)
  )

  return (
    <div className='min-h-svh bg-white text-[#18181b] dark:bg-[#18181b] dark:text-[#f4f4f5]'>
      <div className='fixed inset-x-0 top-0 z-50 flex h-14 items-center border-b border-[#e4e4e7] bg-white/95 px-4 backdrop-blur lg:hidden dark:border-white/10 dark:bg-[#18181b]/95'>
        <Button
          type='button'
          variant='ghost'
          size='icon'
          className='size-8'
          aria-label={mobileOpen ? '关闭文档导航' : '打开文档导航'}
          onClick={() => setMobileOpen((open) => !open)}
        >
          {mobileOpen ? <X /> : <Menu />}
        </Button>
        <Link
          to='/docs'
          className='ml-2 flex items-center gap-2 font-semibold tracking-tight'
        >
          <img
            src='/vector-epoch-relay-mark.png'
            alt='向量纪元 Relay'
            className='size-5 object-contain'
          />
          向量纪元 Relay
        </Link>
      </div>

      <aside
        className={cn(
          'fixed inset-y-0 left-0 z-40 flex w-[18rem] flex-col border-r border-[#e4e4e7] bg-[#f7f7f7] px-3 py-4 transition-transform lg:translate-x-0 dark:border-white/10 dark:bg-[#202020]',
          'pt-[4.5rem] lg:pt-4',
          mobileOpen ? 'translate-x-0' : '-translate-x-full'
        )}
      >
        <div className='flex items-center justify-between px-2'>
          <Link
            to='/docs'
            className='flex min-w-0 items-center gap-2.5 rounded-md font-semibold tracking-tight focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-[#1683c5]'
          >
            <span className='grid size-7 shrink-0 place-items-center rounded-md bg-[#1683c5]/10'>
              <img
                src='/vector-epoch-relay-mark.png'
                alt=''
                className='size-6 object-contain'
              />
            </span>
            <span className='truncate'>向量纪元 Relay</span>
          </Link>
          <Button
            type='button'
            variant='ghost'
            size='icon-sm'
            className='hidden size-7 lg:inline-flex'
            aria-label='收起文档导航'
          >
            <PanelLeftClose className='size-4' />
          </Button>
        </div>

        <Button
          type='button'
          variant='outline'
          className='mt-6 h-10 w-full justify-start gap-2 rounded-lg bg-white px-3 text-sm font-normal text-[#71717a] dark:bg-[#181818] dark:text-[#a1a1aa]'
          aria-label='打开文档搜索'
          onClick={() => setSearchOpen(true)}
        >
          <Search className='size-4' />
          <span>搜索文档</span>
          <kbd className='ms-auto rounded border border-[#d4d4d8] px-1.5 py-0.5 font-mono text-[0.65rem] dark:border-white/15'>
            ⌘ K
          </kbd>
        </Button>

        <nav
          aria-label='文档导航'
          className='mt-8 min-h-0 flex-1 overflow-y-auto'
        >
          <div className='space-y-5'>
            {docsNavGroups.map((group, groupIndex) => {
              const isActiveGroup = activeGroup?.label === group.label
              const isOpen = openGroups[group.label] ?? isActiveGroup
              const navId = `docs-nav-group-${groupIndex}`
              return (
                <div key={group.label}>
                  <button
                    type='button'
                    className='flex w-full items-center gap-1.5 rounded-md px-2 py-1 text-left text-xs font-semibold text-[#71717a] transition-colors hover:text-[#18181b] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#1683c5] dark:text-[#a1a1aa] dark:hover:text-white'
                    aria-expanded={isOpen}
                    aria-controls={navId}
                    onClick={() =>
                      setOpenGroups((groups) => ({
                        ...groups,
                        [group.label]: !isOpen,
                      }))
                    }
                  >
                    <ChevronDown
                      className={cn(
                        'size-3.5 transition-transform',
                        !isOpen && '-rotate-90'
                      )}
                    />
                    {group.label}
                  </button>
                  <div
                    id={navId}
                    className={cn('mt-1 space-y-0.5', !isOpen && 'hidden')}
                  >
                    {group.items.map((item) => {
                      const active = item.to === currentPath
                      return (
                        <Link
                          key={item.to}
                          to={item.to}
                          onClick={() => setMobileOpen(false)}
                          className={cn(
                            'block rounded-md px-3 py-2 text-sm transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#1683c5]',
                            active
                              ? 'bg-[#e4e4e7] font-medium text-[#18181b] dark:bg-white/10 dark:text-white'
                              : 'text-[#52525b] hover:bg-[#ebebec] hover:text-[#18181b] dark:text-[#a1a1aa] dark:hover:bg-white/[0.06] dark:hover:text-white'
                          )}
                        >
                          {item.label}
                        </Link>
                      )
                    })}
                  </div>
                </div>
              )
            })}
          </div>
        </nav>

        <div className='mt-5 border-t border-[#dedee2] pt-4 dark:border-white/10'>
          <Link
            to='/dashboard'
            className='flex items-center justify-between rounded-md px-2 py-2 text-sm text-[#52525b] transition-colors hover:bg-[#ebebec] hover:text-[#18181b] dark:text-[#a1a1aa] dark:hover:bg-white/[0.06] dark:hover:text-white'
          >
            进入控制台
            <ArrowUpRight className='size-3.5' />
          </Link>
          <div className='mt-2 flex items-center justify-between px-2 text-xs text-[#71717a] dark:text-[#a1a1aa]'>
            <span>文档 v1</span>
            <ThemeSwitch compact />
          </div>
        </div>
      </aside>

      {mobileOpen && (
        <button
          type='button'
          className='fixed inset-0 top-14 z-30 bg-black/20 lg:hidden dark:bg-black/55'
          aria-label='关闭文档导航'
          onClick={() => setMobileOpen(false)}
        />
      )}

      <main className='min-h-svh lg:pl-[18rem]'>
        <div className='mx-auto grid max-w-[1440px] grid-cols-1 xl:grid-cols-[minmax(0,1fr)_13rem]'>
          <article className='min-w-0 px-6 pt-20 pb-24 sm:px-10 lg:px-16 lg:pt-16'>
            <div className='max-w-[58rem]'>
              <header className='border-b border-[#e4e4e7] pb-8 dark:border-white/10'>
                <div className='mb-4 text-xs font-medium text-[#71717a] dark:text-[#a1a1aa]'>
                  {props.eyebrow}
                </div>
                <h1 className='max-w-3xl text-4xl leading-[1.08] font-bold tracking-[-0.045em] text-balance sm:text-5xl'>
                  {props.title}
                </h1>
                <p className='mt-4 max-w-3xl text-lg leading-8 text-[#71717a] dark:text-[#a1a1aa]'>
                  {props.description}
                </p>
                <PageActions />
              </header>
              <div className='pt-10'>{props.children}</div>
            </div>
          </article>

          <aside className='hidden border-l border-[#e4e4e7] px-6 py-16 xl:block dark:border-white/10'>
            <div className='sticky top-12'>
              <div className='mb-3 text-xs font-semibold text-[#71717a] dark:text-[#a1a1aa]'>
                本页导航
              </div>
              <nav className='space-y-2 border-l border-[#e4e4e7] pl-3 dark:border-white/10'>
                {props.sections?.map((section) => (
                  <a
                    key={section.id}
                    href={`#${section.id}`}
                    className='block text-xs leading-5 text-[#71717a] transition-colors hover:text-[#18181b] dark:text-[#a1a1aa] dark:hover:text-white'
                  >
                    {section.label}
                  </a>
                ))}
              </nav>
            </div>
          </aside>
        </div>
      </main>

      {searchOpen && (
        <div className='fixed inset-0 z-[60] flex items-start justify-center bg-black/20 px-4 pt-[12vh] backdrop-blur-sm dark:bg-black/60'>
          <div
            role='dialog'
            aria-modal='true'
            aria-label='搜索文档'
            className='w-full max-w-xl overflow-hidden rounded-xl border border-[#dedee2] bg-white shadow-2xl dark:border-white/10 dark:bg-[#202020]'
          >
            <div className='flex items-center gap-3 border-b border-[#e4e4e7] px-4 dark:border-white/10'>
              <Search className='size-4 text-[#71717a]' />
              <input
                autoFocus
                value={searchValue}
                onChange={(event) => setSearchValue(event.target.value)}
                placeholder='搜索文档…'
                aria-label='搜索文档'
                className='h-14 min-w-0 flex-1 bg-transparent text-sm outline-none placeholder:text-[#a1a1aa]'
              />
              <Button
                type='button'
                aria-label='关闭搜索'
                variant='ghost'
                size='icon-xs'
                onClick={closeSearch}
              >
                <X />
              </Button>
            </div>
            <div className='max-h-[52vh] overflow-y-auto p-2'>
              {filteredSearchItems.length === 0 ? (
                <div className='px-3 py-10 text-center text-sm text-[#71717a]'>
                  没有找到匹配的文档
                </div>
              ) : (
                filteredSearchItems.map((item) => (
                  <Link
                    key={item.to}
                    to={item.to}
                    onClick={closeSearch}
                    className='flex items-center gap-3 rounded-lg px-3 py-3 transition-colors hover:bg-[#f4f4f5] dark:hover:bg-white/[0.06]'
                  >
                    <span className='grid size-8 shrink-0 place-items-center rounded-md bg-[#f4f4f5] text-[#71717a] dark:bg-white/[0.08]'>
                      <ChevronRight className='size-4' />
                    </span>
                    <span className='min-w-0'>
                      <span className='block text-sm font-medium'>
                        {item.label}
                      </span>
                      <span className='block truncate text-xs text-[#71717a] dark:text-[#a1a1aa]'>
                        {item.description}
                      </span>
                    </span>
                    <span className='ml-auto text-[0.65rem] text-[#a1a1aa]'>
                      {item.group}
                    </span>
                  </Link>
                ))
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export function DocsSectionTitle(props: { id: string; children: ReactNode }) {
  return (
    <h2
      id={props.id}
      className='scroll-mt-28 border-t border-[#e4e4e7] pt-8 text-2xl font-semibold tracking-[-0.025em] first:border-t-0 first:pt-0 dark:border-white/10'
    >
      {props.children}
    </h2>
  )
}

export function DocsCard(props: { children: ReactNode; className?: string }) {
  return (
    <div
      className={cn(
        'rounded-xl border border-[#e4e4e7] bg-[#fafafa] p-5 dark:border-white/10 dark:bg-white/[0.035]',
        props.className
      )}
    >
      {props.children}
    </div>
  )
}

export function DocsCallout(props: {
  tone?: 'accent' | 'muted'
  title: string
  children: ReactNode
}) {
  return (
    <div
      className={cn(
        'rounded-xl border p-5',
        props.tone === 'muted'
          ? 'border-[#e4e4e7] bg-[#fafafa] dark:border-white/10 dark:bg-white/[0.045]'
          : 'border-[#f5c78e] bg-[#fff8eb] dark:border-[#c76425]/40 dark:bg-[#c76425]/10'
      )}
    >
      <div className='mb-2 flex items-center gap-2 text-sm font-semibold'>
        <Badge
          variant='outline'
          className='border-[#e5a95d]/60 text-[#a15c10] dark:text-[#e8a875]'
        >
          {props.tone === 'muted' ? '说明' : '建议'}
        </Badge>
        {props.title}
      </div>
      <div className='text-sm leading-7 text-[#52525b] dark:text-[#c4c4cc]'>
        {props.children}
      </div>
    </div>
  )
}

export function DocsCodeBlock(props: {
  code: string
  language?: string
  title?: string
}) {
  const [copied, setCopied] = useState(false)

  const copyCode = async () => {
    await navigator.clipboard.writeText(props.code)
    setCopied(true)
    window.setTimeout(() => setCopied(false), 1600)
  }

  return (
    <div className='overflow-hidden rounded-xl border border-[#27272a] bg-[#18181b] text-[#f4f4f5] shadow-[0_14px_35px_-25px_rgba(24,24,27,0.8)]'>
      <div className='flex items-center justify-between border-b border-white/10 px-4 py-2.5 text-[0.68rem] tracking-[0.12em] text-[#a1a1aa] uppercase'>
        <span>{props.title ?? props.language ?? 'request'}</span>
        <button
          type='button'
          onClick={copyCode}
          className='inline-flex items-center gap-1.5 rounded-md px-2 py-1 transition-colors hover:bg-white/10 hover:text-white focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#f1b27c]'
        >
          {copied ? (
            <Check className='size-3.5' />
          ) : (
            <Copy className='size-3.5' />
          )}
          {copied ? '已复制' : '复制'}
        </button>
      </div>
      <pre className='overflow-x-auto p-4 text-[0.78rem] leading-6 text-[#e4e4e7]'>
        <code>{props.code}</code>
      </pre>
    </div>
  )
}

export function DocsStep(props: {
  number: string
  title: string
  children: ReactNode
}) {
  return (
    <div className='relative pl-12'>
      <span className='absolute top-0 left-0 grid size-8 place-items-center rounded-full border border-[#e4e4e7] bg-white font-mono text-xs font-semibold text-[#1683c5] dark:border-white/10 dark:bg-[#18181b]'>
        {props.number}
      </span>
      <h3 className='text-base font-semibold'>{props.title}</h3>
      <div className='mt-2 text-sm leading-7 text-[#52525b] dark:text-[#c4c4cc]'>
        {props.children}
      </div>
    </div>
  )
}
