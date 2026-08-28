/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { SearchAdminAgentKeysPage } from '../admin/agent-keys-page'
import {
  createAdminSearchAgentKey,
  createAdminSearchInstallToken,
  fetchAdminSearchAgentKeys,
  fetchSearchAgentKeyOwnerCandidates,
  revokeAdminSearchAgentKey,
} from '../api'

vi.mock('../api', () => ({
  createAdminSearchAgentKey: vi.fn(),
  createAdminSearchInstallToken: vi.fn(),
  fetchAdminSearchAgentKeys: vi.fn(),
  fetchSearchAgentKeyOwnerCandidates: vi.fn(),
  revokeAdminSearchAgentKey: vi.fn(),
}))

vi.mock('@tanstack/react-router', () => ({
  Link: ({
    children,
    to,
    ...props
  }: {
    children: React.ReactNode
    to: string
  }) => (
    <a href={to} {...props}>
      {children}
    </a>
  ),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <SearchAdminAgentKeysPage />
    </QueryClientProvider>
  )
}

describe('vSearch managed AgentKey page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useAuthStore.getState().auth.setUser({
      id: 1,
      username: 'root',
      role: ROLE.SUPER_ADMIN,
    })
    vi.mocked(fetchSearchAgentKeyOwnerCandidates).mockResolvedValue([
      { id: 7, username: 'alice', display_name: 'Alice' },
    ])
    vi.mocked(fetchAdminSearchAgentKeys).mockResolvedValue([
      {
        id: 21,
        user_id: 7,
        enterprise_id: 11,
        label: 'research-bot',
        prefix: 'vr_live_admin',
        owner: 'alice',
        status: 'active',
        scopes: ['web-search'],
        created_at: 1,
      },
    ])
    vi.mocked(createAdminSearchInstallToken).mockResolvedValue({
      token: 'vr_search_install_test',
      expires_at: 2,
    })
    vi.mocked(revokeAdminSearchAgentKey).mockResolvedValue()
  })

  test('creates a key for the selected managed user and shows its secret once', async () => {
    vi.mocked(createAdminSearchAgentKey).mockResolvedValue({
      id: 22,
      user_id: 7,
      enterprise_id: 11,
      label: 'ops-bot',
      prefix: 'vr_live_created',
      owner: 'alice',
      status: 'active',
      scopes: ['web-search', 'extract'],
      created_at: 1,
      secret: 'vr_live_created_secret',
    })
    const actor = userEvent.setup()
    renderPage()

    await screen.findByText('research-bot')
    expect(
      screen.getByRole('button', { name: 'Upstream accounts' })
    ).toHaveAttribute('href', '/search/admin/upstream-accounts')
    await actor.selectOptions(screen.getByLabelText('Owner'), '7')
    await actor.type(screen.getByLabelText('Key name'), 'ops-bot')
    await actor.click(
      screen.getByRole('button', { name: 'Create vSearch key' })
    )

    await waitFor(() =>
      expect(createAdminSearchAgentKey).toHaveBeenCalledWith(
        expect.objectContaining({
          user_id: 7,
          name: 'ops-bot',
          scopes: expect.arrayContaining(['web-search', 'extract']),
        })
      )
    )
    expect(await screen.findByText('vr_live_created_secret')).toBeVisible()
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument()
  })

  test('uses the managed install-token and revoke endpoints for another user key', async () => {
    const actor = userEvent.setup()
    renderPage()

    await screen.findByText('research-bot')
    await actor.click(screen.getByRole('button', { name: 'macOS / Linux' }))
    await waitFor(() =>
      expect(createAdminSearchInstallToken).toHaveBeenCalledWith(21)
    )
    expect(
      await screen.findByText((content) =>
        content.includes('vr_search_install_test')
      )
    ).toBeVisible()

    await actor.click(screen.getByRole('button', { name: 'Revoke key' }))
    const confirmRevoke = screen
      .getAllByRole('button', { name: 'Revoke key' })
      .at(-1)
    expect(confirmRevoke).toBeDefined()
    if (!confirmRevoke) return
    await actor.click(confirmRevoke)
    await waitFor(() =>
      expect(revokeAdminSearchAgentKey).toHaveBeenCalledWith(21)
    )
  })

  test('loads only the current enterprise owner candidates for enterprise managers', async () => {
    useAuthStore.getState().auth.setUser({
      id: 3,
      username: 'enterprise-owner',
      role: ROLE.USER,
      enterprise: {
        id: 11,
        name: 'Northstar',
        code: 'northstar',
        membership_id: 5,
        role: 'owner',
      },
    })

    renderPage()

    await screen.findByText('research-bot')
    expect(fetchSearchAgentKeyOwnerCandidates).toHaveBeenCalledWith(11)
    expect(
      screen.queryByRole('button', { name: 'Upstream accounts' })
    ).not.toBeInTheDocument()
  })

  test('keeps root users platform-scoped even when they also have an enterprise membership', async () => {
    useAuthStore.getState().auth.setUser({
      id: 1,
      username: 'root',
      role: ROLE.SUPER_ADMIN,
      enterprise: {
        id: 11,
        name: 'Northstar',
        code: 'northstar',
        membership_id: 5,
        role: 'owner',
      },
    })

    renderPage()

    await screen.findByText('research-bot')
    expect(fetchSearchAgentKeyOwnerCandidates).toHaveBeenCalledWith(undefined)
  })
})
