/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { describe, expect, test } from 'vitest'

import {
  formatCnyMoney,
  formatMicrosForInput,
  parseCnyInputToMicros,
} from '../money'

describe('vSearch money formatting', () => {
  test('prefers exact micros over a conflicting legacy amount', () => {
    expect(formatCnyMoney({ micros: 1, amount: 99 }, 'zh-CN')).toContain(
      '0.000001'
    )
  })

  test('keeps four to six decimal places for sub-cent charges', () => {
    expect(formatCnyMoney({ micros: 1_000 }, 'zh-CN')).toContain('0.0010')
    expect(formatCnyMoney({ micros: 100 }, 'zh-CN')).toContain('0.0001')
    expect(formatCnyMoney({ micros: 10 }, 'zh-CN')).toContain('0.00001')
    expect(formatCnyMoney({ micros: 1 }, 'zh-CN')).toContain('0.000001')
  })

  test('round-trips a six-decimal catalog price without float loss', () => {
    expect(formatMicrosForInput(1_234_567)).toBe('1.234567')
    expect(parseCnyInputToMicros('1.234567')).toBe(1_234_567)
    expect(parseCnyInputToMicros('0.000001')).toBe(1)
    expect(parseCnyInputToMicros('0.0000001')).toBeNull()
  })
})
