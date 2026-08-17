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
import { Link, useNavigate, useRouterState } from '@tanstack/react-router'
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { LanguageSwitcher } from '@/components/language-switcher'
import { NotificationPopover } from '@/components/notification-popover'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useNotifications } from '@/hooks/use-notifications'
import { useSystemConfig } from '@/hooks/use-system-config'
import { useTopNavLinks } from '@/hooks/use-top-nav-links'
import { PUBLIC_INTERFACE_LANGUAGE_OPTIONS } from '@/i18n/languages'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import { defaultTopNavLinks } from '../config/top-nav.config'
import type { TopNavLink } from '../types'
import { HeaderLogo } from './header-logo'

const AUTH_PROMPT_SECONDS = 5

type AuthPromptTarget = {
  title: string
  href: string
}

export interface PublicHeaderProps {
  navLinks?: TopNavLink[]
  mobileLinks?: TopNavLink[]
  navContent?: React.ReactNode
  showThemeSwitch?: boolean
  showLanguageSwitcher?: boolean
  logo?: React.ReactNode
  siteName?: string
  homeUrl?: string
  leftContent?: React.ReactNode
  rightContent?: React.ReactNode
  showNavigation?: boolean
  showAuthButtons?: boolean
  showNotifications?: boolean
  variant?: 'default' | 'editorial'
  useDynamicNavLinks?: boolean
  className?: string
}

export function PublicHeader(props: PublicHeaderProps) {
  const {
    navLinks = defaultTopNavLinks,
    showThemeSwitch = true,
    showLanguageSwitcher = true,
    logo: customLogo,
    siteName: customSiteName,
    homeUrl = '/',
    showAuthButtons = true,
    showNotifications = true,
    variant = 'default',
    useDynamicNavLinks = true,
  } = props

  const { t } = useTranslation()
  const navigate = useNavigate()
  const [scrolled, setScrolled] = useState(false)
  const [mobileOpen, setMobileOpen] = useState(false)
  const [authPromptTarget, setAuthPromptTarget] =
    useState<AuthPromptTarget | null>(null)
  const [authPromptSecondsLeft, setAuthPromptSecondsLeft] =
    useState(AUTH_PROMPT_SECONDS)
  const { auth } = useAuthStore()
  const {
    systemName,
    logo: systemLogo,
    loading,
    logoLoaded,
  } = useSystemConfig()
  const dynamicLinks = useTopNavLinks()
  const notifications = useNotifications()
  const routerState = useRouterState()
  const pathname = routerState.location.pathname

  const user = auth.user
  const isAuthenticated = !!user
  const displaySiteName = customSiteName || systemName
  const isEditorial = variant === 'editorial'
  const links =
    useDynamicNavLinks && dynamicLinks.length > 0 ? dynamicLinks : navLinks
  const getLinkLabel = (title: string) =>
    isEditorial && title === 'FAQ' ? title : t(title)

  const headerContainerClass = cn(
    'pointer-events-auto mx-auto transition-all duration-700 ease-[cubic-bezier(0.16,1,0.3,1)]',
    isEditorial &&
      'relative w-full max-w-7xl px-4 pt-3 sm:px-6 md:pt-4 lg:px-8',
    !isEditorial && scrolled && 'max-w-[52rem] px-3 pt-3',
    !isEditorial && !scrolled && 'max-w-7xl px-4 pt-0 md:px-6'
  )
  const navClassName = cn(
    'transition-all duration-700 ease-[cubic-bezier(0.16,1,0.3,1)]',
    isEditorial &&
      'relative mx-auto flex w-full items-center gap-4 px-4 py-2.5 lg:grid lg:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] xl:gap-6 xl:px-5 xl:py-3',
    isEditorial &&
      scrolled &&
      'rounded-full border border-[#d8d1c4] bg-white/70 shadow-[0_8px_30px_-20px_rgba(92,82,74,0.35)] backdrop-blur-md dark:border-white/10 dark:bg-[#11110f]/75',
    !isEditorial && 'flex items-center justify-between',
    !isEditorial &&
      scrolled &&
      'bg-background/60 ring-border/50 h-12 rounded-2xl pr-1.5 pl-4 shadow-[0_2px_16px_-6px_rgba(0,0,0,0.08),0_0_0_0.5px_rgba(0,0,0,0.02)] ring-[0.5px] backdrop-blur-2xl dark:shadow-[0_2px_16px_-6px_rgba(0,0,0,0.4)]',
    !isEditorial && !scrolled && 'h-16 px-2'
  )

  let logoContent: React.ReactNode
  if (loading) {
    logoContent = <Skeleton className='size-full rounded-lg' />
  } else if (customLogo) {
    logoContent = customLogo
  } else {
    logoContent = (
      <HeaderLogo
        src={systemLogo}
        loading={loading}
        logoLoaded={logoLoaded}
        className='size-full rounded-lg object-contain'
      />
    )
  }

  let authControl: React.ReactNode
  if (loading) {
    authControl = <Skeleton className='h-8 w-20 rounded-lg' />
  } else if (isEditorial) {
    authControl = (
      <Button
        size='sm'
        className='h-8 gap-1.5 rounded-xl bg-[#1f1b17] px-4 text-xs font-medium text-white hover:opacity-90 dark:bg-white dark:text-black'
        render={<Link to='/dashboard' />}
      >
        {t('Console')}
      </Button>
    )
  } else if (isAuthenticated) {
    authControl = <ProfileDropdown />
  } else {
    authControl = (
      <Button
        size='sm'
        className='h-8 rounded-lg px-3.5 text-xs font-medium'
        render={<Link to='/sign-in' />}
      >
        {t('Sign in')}
      </Button>
    )
  }

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 20)
    onScroll()
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => window.removeEventListener('scroll', onScroll)
  }, [])

  useEffect(() => {
    document.body.style.overflow = mobileOpen ? 'hidden' : ''
    return () => {
      document.body.style.overflow = ''
    }
  }, [mobileOpen])

  useEffect(() => {
    if (!authPromptTarget) return

    const intervalId = window.setInterval(() => {
      setAuthPromptSecondsLeft((seconds) => Math.max(seconds - 1, 0))
    }, 1000)

    const timeoutId = window.setTimeout(() => {
      const redirect = authPromptTarget.href
      setAuthPromptTarget(null)
      navigate({ to: '/sign-in', search: { redirect } })
    }, AUTH_PROMPT_SECONDS * 1000)

    return () => {
      window.clearInterval(intervalId)
      window.clearTimeout(timeoutId)
    }
  }, [authPromptTarget, navigate])

  const closeAuthPrompt = useCallback(() => {
    setAuthPromptTarget(null)
    setAuthPromptSecondsLeft(AUTH_PROMPT_SECONDS)
  }, [])

  const navigateToSignIn = useCallback(() => {
    const redirect = authPromptTarget?.href || '/'
    setAuthPromptTarget(null)
    navigate({ to: '/sign-in', search: { redirect } })
  }, [authPromptTarget?.href, navigate])

  const handleNavLinkClick = useCallback(
    (
      event: React.MouseEvent<HTMLAnchorElement>,
      link: TopNavLink,
      closeMobile = false
    ) => {
      if (link.disabled) {
        event.preventDefault()
        return
      }

      if (link.requiresAuth) {
        event.preventDefault()
        if (closeMobile) {
          setMobileOpen(false)
        }
        setAuthPromptSecondsLeft(AUTH_PROMPT_SECONDS)
        setAuthPromptTarget({
          title: t(link.title),
          href: link.href,
        })
        return
      }

      if (pathname === '/' && link.href.startsWith('/#')) {
        const target = document.querySelector<HTMLElement>(
          `#${link.href.slice(2)}`
        )
        if (target) {
          event.preventDefault()
          window.history.pushState({}, '', link.href)
          target.scrollIntoView({ behavior: 'smooth', block: 'start' })
        }
      }

      if (closeMobile) {
        setMobileOpen(false)
      }
    },
    [pathname, t]
  )

  return (
    <>
      <header className='pointer-events-none fixed inset-x-0 top-0 z-50'>
        <div className={headerContainerClass}>
          <nav className={navClassName}>
            {/* Logo */}
            <Link
              to={homeUrl}
              className='group flex shrink-0 items-center gap-2.5'
            >
              <div
                className={cn(
                  'flex shrink-0 items-center justify-center transition-all duration-300 group-hover:scale-105',
                  isEditorial ? 'size-8 xl:size-9' : 'size-7'
                )}
              >
                {logoContent}
              </div>
              <span
                className={cn(
                  isEditorial
                    ? 'font-serif text-[1.25rem] font-medium tracking-normal xl:text-[1.5rem]'
                    : 'font-sans text-sm font-semibold tracking-tight'
                )}
              >
                {loading ? <Skeleton className='h-4 w-16' /> : displaySiteName}
              </span>
            </Link>

            {/* Desktop nav */}
            <div
              className={cn(
                isEditorial
                  ? 'hidden items-center justify-center lg:contents'
                  : 'hidden items-center gap-0.5 sm:flex'
              )}
            >
              <div
                className={cn(
                  isEditorial
                    ? 'flex items-center justify-center gap-0.5 xl:gap-1'
                    : 'contents'
                )}
              >
                {links.map((link) => {
                  const isActive = pathname === link.href
                  const linkClassName = cn(
                    isEditorial
                      ? 'rounded-full px-3 py-1.5 text-xs font-medium text-muted-foreground transition hover:text-foreground xl:px-4 xl:py-2 xl:text-sm'
                      : 'rounded-lg px-3 py-1.5 text-sm font-medium transition-colors duration-200',
                    !isEditorial &&
                      (isActive
                        ? 'text-foreground'
                        : 'text-muted-foreground hover:text-foreground'),
                    link.disabled && 'pointer-events-none opacity-50'
                  )
                  if (link.external) {
                    return (
                      <a
                        key={link.href}
                        href={link.href}
                        target='_blank'
                        rel='noopener noreferrer'
                        aria-disabled={link.disabled}
                        tabIndex={link.disabled ? -1 : undefined}
                        onClick={(event) => handleNavLinkClick(event, link)}
                        className={linkClassName}
                      >
                        {getLinkLabel(link.title)}
                      </a>
                    )
                  }
                  return (
                    <Link
                      key={link.href}
                      to={link.href}
                      disabled={link.disabled}
                      onClick={(event) => handleNavLinkClick(event, link)}
                      className={cn(
                        linkClassName,
                        isEditorial && isActive && 'text-foreground'
                      )}
                    >
                      {getLinkLabel(link.title)}
                    </Link>
                  )
                })}
              </div>

              <div
                className={cn(
                  'flex items-center',
                  isEditorial
                    ? 'relative z-10 justify-self-end gap-2 xl:gap-3'
                    : 'gap-0.5'
                )}
              >
                {(showLanguageSwitcher ||
                  showThemeSwitch ||
                  showNotifications) && (
                  <div className='bg-border/40 mx-2 h-4 w-px' />
                )}

                {showLanguageSwitcher && (
                  <LanguageSwitcher
                    compact={isEditorial}
                    options={
                      isEditorial
                        ? PUBLIC_INTERFACE_LANGUAGE_OPTIONS
                        : undefined
                    }
                  />
                )}
                {showThemeSwitch && <ThemeSwitch compact={isEditorial} />}
                {showNotifications && (
                  <NotificationPopover
                    open={notifications.popoverOpen}
                    onOpenChange={notifications.setPopoverOpen}
                    unreadCount={notifications.unreadCount}
                    activeTab={notifications.activeTab}
                    onTabChange={notifications.setActiveTab}
                    notice={notifications.notice}
                    announcements={notifications.announcements}
                    loading={notifications.loading}
                  />
                )}

                {showAuthButtons && (
                  <>
                    {!isEditorial && (
                      <div className='bg-border/40 mx-1 h-4 w-px' />
                    )}
                    {authControl}
                  </>
                )}
              </div>
            </div>

            {/* Mobile: compact actions + hamburger */}
            <div
              className={cn(
                'ml-auto flex items-center gap-2',
                isEditorial ? 'lg:hidden' : 'sm:hidden'
              )}
            >
              {showThemeSwitch && <ThemeSwitch />}
              {showAuthButtons && !loading && isAuthenticated && (
                <ProfileDropdown />
              )}
              <Button
                type='button'
                variant='ghost'
                size='icon'
                className='size-9'
                onClick={() => setMobileOpen((v) => !v)}
                aria-label={t('Toggle navigation menu')}
              >
                <div className='relative size-4'>
                  <span
                    className={cn(
                      'absolute inset-x-0 block h-[1.5px] origin-center rounded-full bg-current transition-all duration-300',
                      mobileOpen ? 'top-[7px] rotate-45' : 'top-[3px]'
                    )}
                  />
                  <span
                    className={cn(
                      'absolute inset-x-0 top-[7px] block h-[1.5px] rounded-full bg-current transition-all duration-300',
                      mobileOpen ? 'scale-x-0 opacity-0' : 'opacity-100'
                    )}
                  />
                  <span
                    className={cn(
                      'absolute inset-x-0 block h-[1.5px] origin-center rounded-full bg-current transition-all duration-300',
                      mobileOpen ? 'top-[7px] -rotate-45' : 'top-[11px]'
                    )}
                  />
                </div>
              </Button>
            </div>
          </nav>
        </div>
      </header>

      {/* Mobile full-screen overlay */}
      <div
        className={cn(
          'bg-background/98 fixed inset-0 z-40 backdrop-blur-2xl transition-all duration-500 ease-[cubic-bezier(0.16,1,0.3,1)]',
          isEditorial
            ? 'lg:pointer-events-none lg:hidden'
            : 'sm:pointer-events-none sm:hidden',
          mobileOpen
            ? 'pointer-events-auto opacity-100'
            : 'pointer-events-none opacity-0'
        )}
      >
        <div className='flex h-full flex-col justify-between px-8 pt-20 pb-10'>
          <nav className='flex flex-col gap-1'>
            {links.map((link, i) => {
              const isActive = pathname === link.href
              const linkClassName = cn(
                'flex items-center gap-3 py-3 text-base font-medium tracking-tight transition-all duration-500 ease-[cubic-bezier(0.16,1,0.3,1)]',
                mobileOpen
                  ? 'translate-y-0 opacity-100'
                  : 'translate-y-4 opacity-0',
                isActive ? 'text-foreground' : 'text-muted-foreground',
                link.disabled && 'pointer-events-none opacity-50'
              )
              const transitionStyle = {
                transitionDelay: mobileOpen ? `${100 + i * 50}ms` : '0ms',
              }
              if (link.external) {
                return (
                  <a
                    key={link.href}
                    href={link.href}
                    target='_blank'
                    rel='noopener noreferrer'
                    aria-disabled={link.disabled}
                    tabIndex={link.disabled ? -1 : undefined}
                    onClick={(event) => handleNavLinkClick(event, link, true)}
                    className={linkClassName}
                    style={transitionStyle}
                  >
                    {getLinkLabel(link.title)}
                  </a>
                )
              }
              return (
                <Link
                  key={link.href}
                  to={link.href}
                  disabled={link.disabled}
                  onClick={(event) => handleNavLinkClick(event, link, true)}
                  className={linkClassName}
                  style={transitionStyle}
                >
                  {getLinkLabel(link.title)}
                </Link>
              )
            })}
          </nav>

          <div
            className={cn(
              'flex flex-col gap-3 transition-all duration-500',
              mobileOpen
                ? 'translate-y-0 opacity-100'
                : 'translate-y-4 opacity-0'
            )}
            style={{ transitionDelay: mobileOpen ? '250ms' : '0ms' }}
          >
            {showAuthButtons && (
              <Link
                to={isAuthenticated ? '/dashboard' : '/sign-in'}
                onClick={() => setMobileOpen(false)}
                className='bg-foreground text-background inline-flex h-10 items-center justify-center rounded-lg text-sm font-medium transition-opacity hover:opacity-90 active:opacity-80'
              >
                {isAuthenticated ? t('Go to Dashboard') : t('Sign in')}
              </Link>
            )}
          </div>
        </div>
      </div>

      <Dialog
        open={!!authPromptTarget}
        onOpenChange={(open) => {
          if (!open) {
            closeAuthPrompt()
          }
        }}
        title={t('Sign in required')}
        description={t('Please sign in to view {{module}}.', {
          module: authPromptTarget?.title || '',
        })}
        contentClassName='sm:max-w-md'
        contentHeight='auto'
        footer={
          <>
            <Button variant='outline' onClick={closeAuthPrompt}>
              {t('Cancel')}
            </Button>
            <Button onClick={navigateToSignIn}>{t('Sign in now')}</Button>
          </>
        }
      >
        <div className='bg-muted/40 text-muted-foreground rounded-lg px-3 py-2 text-sm'>
          {t('Redirecting to sign in in {{seconds}} seconds.', {
            seconds: authPromptSecondsLeft,
          })}
        </div>
      </Dialog>
    </>
  )
}
