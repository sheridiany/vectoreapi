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
*/
import { renderHook } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'

import type { NavGroup } from '@/components/layout/types'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { useSidebarData } from '../use-sidebar-data'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

afterEach(() => {
  useAuthStore.getState().auth.setUser(null)
})

describe('root sidebar vSearch menus', () => {
  test('shows three user menus and three administration menus directly', () => {
    useAuthStore.getState().auth.setUser({
      id: 1,
      username: 'root',
      role: ROLE.SUPER_ADMIN,
    })

    const { result } = renderHook(() => useSidebarData())

    expect(searchUrls(result.current.navGroups, 'general')).toEqual([
      '/search/keys',
      '/search/catalog',
      '/search/logs',
    ])
    expect(searchUrls(result.current.navGroups, 'admin')).toEqual([
      '/search/admin/keys',
      '/search/admin/catalog',
      '/search/admin/usage-logs',
    ])
    expect(searchTitles(result.current.navGroups, 'general')).toEqual([
      'vSearch keys',
      'vSearch capabilities',
      'vSearch logs',
    ])
    expect(searchTitles(result.current.navGroups, 'admin')).toEqual([
      'vSearch keys',
      'vSearch capabilities',
      'vSearch logs',
    ])
  })

  test('does not show platform vSearch administration to enterprise managers', () => {
    useAuthStore.getState().auth.setUser({
      id: 2,
      username: 'owner',
      role: ROLE.USER,
      enterprise: {
        id: 7,
        name: 'Northstar Research',
        code: 'northstar',
        membership_id: 1,
        role: 'owner',
      },
    })

    const { result } = renderHook(() => useSidebarData())

    expect(searchUrls(result.current.navGroups, 'admin')).toEqual([])
    expect(searchTitles(result.current.navGroups, 'admin')).toEqual([])
  })
})

function searchUrls(groups: NavGroup[], id: string) {
  const group = groups.find((item) => item.id === id)
  return (group?.items || [])
    .flatMap((item) => ('url' in item && item.url ? [String(item.url)] : []))
    .filter((url) => url.startsWith('/search'))
}

function searchTitles(groups: NavGroup[], id: string) {
  const group = groups.find((item) => item.id === id)
  return (group?.items || [])
    .filter((item) => 'url' in item && item.url?.startsWith('/search'))
    .map((item) => item.title)
}
