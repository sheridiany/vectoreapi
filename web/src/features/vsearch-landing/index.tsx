/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'

import { PublicLayout, type TopNavLink } from '@/components/layout'
import { useAuthStore } from '@/stores/auth-store'

import { fetchPublicSearchCatalog } from './api'
import { CanonicalContract } from './components/canonical-contract'
import { FinalCta } from './components/final-cta'
import { VSearchHero } from './components/vsearch-hero'
import { Workflow } from './components/workflow'
import { buildCatalogViewModel } from './lib/catalog-view-model'
import type { PublicSearchCatalogItem } from './types'

const EMPTY_CATALOG: PublicSearchCatalogItem[] = []

const VSEARCH_NAV_LINKS: TopNavLink[] = [
  { title: 'API Gateway', href: '/' },
  { title: 'vSearch', href: '/vsearch' },
  {
    title: 'vSearch capabilities',
    href: '/search/catalog',
    requiresAuth: true,
  },
  { title: 'Pricing', href: '/pricing' },
]

export function VSearchLanding() {
  const isAuthenticated = useAuthStore(
    (state) => !!state.auth.user && !!state.auth.accessToken
  )
  const catalogQuery = useQuery({
    queryKey: ['public-vsearch-catalog'],
    queryFn: fetchPublicSearchCatalog,
    staleTime: 5 * 60 * 1000,
    retry: false,
  })
  const catalog = catalogQuery.data || EMPTY_CATALOG
  const catalogViewModel = useMemo(
    () => buildCatalogViewModel(catalog),
    [catalog]
  )

  return (
    <PublicLayout
      showMainContainer={false}
      headerProps={{
        navLinks: VSEARCH_NAV_LINKS,
        showNotifications: false,
        useDynamicNavLinks: false,
        variant: 'editorial',
      }}
    >
      <main className='overflow-x-clip bg-white font-sans text-[#0b1324] dark:bg-[#050b16] dark:text-[#f1f6ff]'>
        <VSearchHero
          metrics={catalogViewModel.metrics}
          isAuthenticated={isAuthenticated}
          isLoading={catalogQuery.isPending}
          isError={catalogQuery.isError}
        />
        <Workflow />
        <CanonicalContract />
        <FinalCta isAuthenticated={isAuthenticated} />
      </main>
    </PublicLayout>
  )
}
