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
import { render, screen } from '@testing-library/react'
import { describe, expect, test } from 'vitest'

const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { MemberRankingTable } = await import('../member-ranking-table')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        Rank: 'Rank',
        Member: 'Member',
        'Net consumption': 'Net consumption',
        Tokens: 'Tokens',
        Requests: 'Requests',
        Growth: 'Growth',
        'No Data': 'No Data',
        'No enterprise activity yet.': 'No enterprise activity yet.',
      },
    },
  },
})

function renderTable(data: Parameters<typeof MemberRankingTable>[0]['data']) {
  return render(
    <I18nextProvider i18n={i18n}>
      <MemberRankingTable data={data} />
    </I18nextProvider>
  )
}

describe('enterprise member ranking table', () => {
  test('shows member identity and formatted usage metrics', () => {
    renderTable([
      {
        rank: 1,
        user_id: 4,
        username: 'roarkist',
        net_quota: 123456,
        total_tokens: 9876,
        request_count: 42,
        growth_pct: 12.5,
      },
    ])

    expect(screen.getByText('roarkist')).toBeInTheDocument()
    expect(screen.getByText('ID 4')).toBeInTheDocument()
    expect(screen.getByText('123,456')).toBeInTheDocument()
    expect(screen.getByText('+12.5%')).toBeInTheDocument()
  })

  test('shows a composed empty state when no members have usage', () => {
    renderTable([])

    expect(screen.getByText('No Data')).toBeInTheDocument()
    expect(screen.getByText('No enterprise activity yet.')).toBeInTheDocument()
  })
})
