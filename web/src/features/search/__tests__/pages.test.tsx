/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { createSearchAgentKey, fetchSearchAgentKeys } from '../api'
import { SearchKeysPage } from '../search-keys-page'

vi.mock('../api', () => ({
  fetchSearchAgentKeys: vi.fn().mockResolvedValue([]),
  createSearchAgentKey: vi.fn(),
  revokeSearchAgentKey: vi.fn(),
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

function renderWithQuery(ui: React.ReactNode) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>
  )
}

describe('vSearch key page', () => {
  beforeEach(() => vi.clearAllMocks())

  test('loads the dedicated key list and keeps model credentials separate', async () => {
    renderWithQuery(<SearchKeysPage />)

    expect(await screen.findByText('No vSearch keys yet')).toBeInTheDocument()
    expect(fetchSearchAgentKeys).toHaveBeenCalledOnce()
    expect(
      screen.getByRole('button', { name: 'Create vSearch key' })
    ).toBeEnabled()
    expect(
      screen.getByRole('link', { name: 'Open model API Keys' })
    ).toHaveAttribute('href', '/keys')
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument()
  })

  test('shows a newly created secret once', async () => {
    vi.mocked(createSearchAgentKey).mockResolvedValueOnce({
      id: 11,
      user_id: 7,
      enterprise_id: 0,
      label: 'research-bot',
      prefix: 'vr_live_user',
      status: 'active',
      scopes: ['web-search'],
      created_at: 1,
      secret: 'vr_live_user_secret',
    })
    const user = userEvent.setup()
    renderWithQuery(<SearchKeysPage />)

    await screen.findByText('No vSearch keys yet')
    await user.type(screen.getByLabelText('Key name'), 'research-bot')
    await user.click(screen.getByRole('button', { name: 'Create vSearch key' }))

    expect(await screen.findByText('vr_live_user_secret')).toBeInTheDocument()
    expect(createSearchAgentKey).toHaveBeenCalledWith(
      'research-bot',
      expect.arrayContaining(['web-search'])
    )
  })

  test('requires confirmation before revoking a key', async () => {
    vi.mocked(fetchSearchAgentKeys).mockResolvedValueOnce([
      {
        id: 12,
        user_id: 7,
        enterprise_id: 0,
        label: 'research-bot',
        prefix: 'vr_live_user',
        status: 'active',
        scopes: ['web-search'],
        created_at: 1,
      },
    ])
    const user = userEvent.setup()
    renderWithQuery(<SearchKeysPage />)

    await screen.findByText('research-bot')
    await user.click(screen.getByRole('button', { name: 'Revoke key' }))

    expect(screen.getByRole('alertdialog')).toHaveTextContent(
      'Revoke vSearch key?'
    )
  })

  test('generates an install command with the newly created key', async () => {
    vi.mocked(createSearchAgentKey).mockResolvedValueOnce({
      id: 12,
      user_id: 7,
      enterprise_id: 0,
      label: 'research-bot',
      prefix: 'vr_live_user',
      status: 'active',
      scopes: ['web-search'],
      created_at: 1,
      secret: 'vr_live_user_secret',
    })
    const user = userEvent.setup()
    renderWithQuery(<SearchKeysPage />)

    await screen.findByText('No vSearch keys yet')
    await user.type(screen.getByLabelText('Key name'), 'research-bot')
    await user.click(screen.getByRole('button', { name: 'Create vSearch key' }))
    await user.click(screen.getByRole('button', { name: 'macOS / Linux' }))

    expect(await screen.findByText(/curl -fsSL/)).toHaveTextContent(
      "--key 'vr_live_user_secret'"
    )
    expect(screen.getByText(/Uses the same vSearch key/)).toBeInTheDocument()
    expect(
      screen.getByText(
        'The installer keeps this key unchanged and can be run again on another device.'
      )
    ).toBeInTheDocument()
  })

  test('does not offer install commands for previously created keys', async () => {
    vi.mocked(fetchSearchAgentKeys).mockResolvedValueOnce([
      {
        id: 12,
        user_id: 7,
        enterprise_id: 0,
        label: 'disabled-key',
        prefix: 'vr_disabled',
        status: 'disabled',
        scopes: ['web-search'],
        created_at: 1,
      },
      {
        id: 13,
        user_id: 7,
        enterprise_id: 0,
        label: 'revoked-key',
        prefix: 'vr_revoked',
        status: 'revoked',
        scopes: ['web-search'],
        created_at: 1,
      },
    ])
    renderWithQuery(<SearchKeysPage />)

    const disabledRow = (await screen.findByText('disabled-key')).closest(
      '[data-search-key]'
    ) as HTMLElement | null
    const revokedRow = screen
      .getByText('revoked-key')
      .closest('[data-search-key]') as HTMLElement | null
    expect(disabledRow).not.toBeNull()
    expect(revokedRow).not.toBeNull()
    if (!disabledRow || !revokedRow) return

    expect(
      within(disabledRow).queryByRole('button', { name: 'macOS / Linux' })
    ).not.toBeInTheDocument()
    expect(
      within(disabledRow).queryByRole('button', {
        name: 'Windows PowerShell',
      })
    ).not.toBeInTheDocument()
    expect(
      within(revokedRow).queryByRole('button', { name: 'macOS / Linux' })
    ).not.toBeInTheDocument()
  })
})
