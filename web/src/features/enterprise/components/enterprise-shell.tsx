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
import { Link } from '@tanstack/react-router'
import {
  BarChart3,
  Building2,
  ClipboardList,
  KeyRound,
  Settings2,
  Users,
  type LucideIcon,
} from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { StatusBadge } from '@/components/status-badge'
import { Separator } from '@/components/ui/separator'
import { ROLE } from '@/lib/roles'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

export type EnterpriseSection =
  | 'overview'
  | 'members'
  | 'rankings'
  | 'audit'
  | 'api-access'
  | 'onboarding'

type SectionConfig = {
  id: EnterpriseSection
  href: string
  labelKey: string
  icon: LucideIcon
  managerOnly?: boolean
}

const sections: SectionConfig[] = [
  {
    id: 'overview',
    href: '/enterprise/overview',
    labelKey: 'Overview',
    icon: Building2,
  },
  {
    id: 'members',
    href: '/enterprise/members',
    labelKey: 'Enterprise members',
    icon: Users,
    managerOnly: true,
  },
  {
    id: 'rankings',
    href: '/enterprise/rankings',
    labelKey: 'Member consumption rankings',
    icon: BarChart3,
  },
  {
    id: 'audit',
    href: '/enterprise/audit',
    labelKey: 'Audit logs',
    icon: ClipboardList,
  },
  {
    id: 'api-access',
    href: '/enterprise/api-access',
    labelKey: 'API access',
    icon: KeyRound,
  },
  {
    id: 'onboarding',
    href: '/enterprise/onboarding',
    labelKey: 'Enterprise settings',
    icon: Settings2,
    managerOnly: true,
  },
]

export function EnterpriseShell(props: {
  section: EnterpriseSection
  title: string
  description?: string
  action?: ReactNode
  children: ReactNode
}) {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const enterprise = user?.enterprise
  const isPlatformAdmin = user?.role === ROLE.SUPER_ADMIN
  const isEnterpriseManager =
    enterprise?.role === 'owner' || enterprise?.role === 'admin'
  const roleLabel = enterprise?.role
    ? {
        owner: 'Owner',
        admin: 'Admin',
        auditor: 'Auditor',
        member: 'Member',
      }[enterprise.role]
    : undefined

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{props.title}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>{props.action}</SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='mx-auto w-full max-w-[1440px] space-y-5'>
          <section className='border-border/60 bg-card rounded-xl border shadow-xs'>
            <div className='flex flex-col gap-4 p-4 sm:p-5 lg:flex-row lg:items-center lg:justify-between'>
              <div className='flex min-w-0 items-center gap-3'>
                <div className='bg-primary/10 text-primary flex size-11 shrink-0 items-center justify-center rounded-xl'>
                  <Building2 className='size-5' aria-hidden='true' />
                </div>
                <div className='min-w-0'>
                  <div className='flex flex-wrap items-center gap-2'>
                    <h1 className='truncate text-base font-semibold tracking-tight sm:text-lg'>
                      {isPlatformAdmin
                        ? t('Enterprise management')
                        : enterprise?.name || t('Enterprise')}
                    </h1>
                    {!isPlatformAdmin && enterprise?.role && (
                      <StatusBadge
                        label={t(roleLabel ?? 'Member')}
                        variant='info'
                        copyable={false}
                      />
                    )}
                  </div>
                  <p className='text-muted-foreground mt-1 truncate text-xs'>
                    {isPlatformAdmin
                      ? t('Platform')
                      : enterprise?.code || t('Enterprise')}
                  </p>
                </div>
              </div>
              <div className='text-muted-foreground flex items-center gap-2 text-xs'>
                <span>{t('Current page')}</span>
                <span className='text-foreground font-medium'>
                  {props.title}
                </span>
              </div>
            </div>
            {!isPlatformAdmin && (
              <>
                <Separator />
                <nav
                  aria-label={t('Enterprise navigation')}
                  className='flex gap-1 overflow-x-auto p-2'
                >
                  {sections
                    .filter((item) => !item.managerOnly || isEnterpriseManager)
                    .map((item) => {
                      const Icon = item.icon
                      const active = item.id === props.section
                      return (
                        <Link
                          key={item.id}
                          to={item.href}
                          className={cn(
                            'text-muted-foreground hover:text-foreground inline-flex shrink-0 items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium transition-colors',
                            'focus-visible:ring-ring focus-visible:ring-2 focus-visible:outline-none',
                            active &&
                              'bg-primary/10 text-primary hover:text-primary'
                          )}
                          aria-current={active ? 'page' : undefined}
                        >
                          <Icon className='size-4' aria-hidden='true' />
                          {t(item.labelKey)}
                        </Link>
                      )
                    })}
                </nav>
              </>
            )}
          </section>
          {props.description && (
            <p className='text-muted-foreground max-w-3xl text-sm leading-6'>
              {props.description}
            </p>
          )}
          {props.children}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
