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
*/
import { describe, expect, test } from 'vitest'

import { toIntlLocale } from './languages'

describe('interface language locale conversion', () => {
  test('maps project language codes to valid Intl locales', () => {
    expect(toIntlLocale('zhCN')).toBe('zh-CN')
    expect(toIntlLocale('zhTW')).toBe('zh-TW')
  })

  test('does not pass malformed language codes to Intl', () => {
    expect(toIntlLocale('zh CN')).toBeUndefined()
    expect(() => new Intl.DateTimeFormat(toIntlLocale('zhCN'))).not.toThrow()
  })
})
