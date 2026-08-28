/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import type { TFunction } from 'i18next'
import { describe, expect, test } from 'vitest'

import {
  getNavGroupsForPath,
  resolveSidebarView,
} from '../sidebar-view-registry'

const t = ((key: string) => key) as TFunction

describe('vSearch root sidebar', () => {
  test.each([
    '/search/keys',
    '/search/catalog',
    '/search/logs',
    '/search/admin/agent-keys',
    '/search/admin/upstream-accounts',
    '/search/admin/catalog',
    '/search/admin/usage-logs',
  ])('keeps the root sidebar visible on %s', (pathname) => {
    expect(resolveSidebarView(pathname)).toBeNull()
    expect(getNavGroupsForPath(pathname, t)).toBeNull()
  })
})
