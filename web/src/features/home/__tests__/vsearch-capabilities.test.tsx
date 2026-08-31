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
import { render, screen } from '@testing-library/react'
import type { ReactNode } from 'react'
import { describe, expect, test, vi } from 'vitest'

import { VSearchCapabilities } from '../components/sections/vsearch-capabilities'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('@/components/animate-in-view', () => ({
  AnimateInView: ({
    children,
    className,
  }: {
    children: ReactNode
    className?: string
  }) => <div className={className}>{children}</div>,
}))

describe('home vSearch capability section', () => {
  test('shows the supported platforms as a non-interactive showcase', () => {
    render(<VSearchCapabilities />)

    expect(
      screen.getByRole('heading', {
        name: 'Real-time social data, built for intelligent agents.',
      })
    ).toBeVisible()
    expect(screen.getAllByLabelText('TikTok API')).toHaveLength(2)
    expect(screen.getAllByLabelText('Instagram API')).toHaveLength(2)
    expect(screen.getAllByLabelText('WeChat API')).toHaveLength(2)
    expect(screen.getAllByLabelText('Reddit API')).toHaveLength(2)
    for (const platform of [
      'Douyin API',
      'Rednote API',
      'Kuaishou API',
      'Lemon8 API',
      'LinkedIn API',
    ]) {
      expect(
        screen.getAllByLabelText(platform)[0].querySelector('svg')
      ).not.toBeNull()
    }
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })
})
