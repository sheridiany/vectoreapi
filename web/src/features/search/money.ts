/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/

const MICROS_PER_CNY = 1_000_000
const MAX_CNY_MICROS = 9_000_000_000_000_000

type MoneyValue = {
  micros?: number | null
  amount?: number | null
}

export function formatCnyMoney(
  value: MoneyValue,
  locales?: Intl.LocalesArgument
) {
  return formatMoneyValue(value, 'CNY', 'symbol', locales)
}

export function formatMoney(
  value: MoneyValue,
  currency: string,
  locales?: Intl.LocalesArgument
) {
  return formatMoneyValue(value, currency, 'code', locales)
}

function formatMoneyValue(
  value: MoneyValue,
  currency: string,
  currencyDisplay: 'code' | 'symbol',
  locales?: Intl.LocalesArgument
) {
  const micros = resolveCnyMicros(value)
  if (micros === null) return '—'

  const normalizedCurrency = currency.trim().toUpperCase() || 'CNY'
  const absoluteMicros = Math.abs(micros)
  const whole = Math.floor(absoluteMicros / MICROS_PER_CNY)
  const remainder = absoluteMicros % MICROS_PER_CNY
  const fractionDigits = moneyFractionDigits(remainder)
  const fraction = String(remainder).padStart(6, '0').slice(0, fractionDigits)
  const templateAmount =
    (micros < 0 ? -1 : 1) * (whole + (remainder === 0 ? 0 : 0.1))

  if (!/^[A-Z]{3}$/.test(normalizedCurrency)) {
    const decimal = `${micros < 0 ? '-' : ''}${whole}${fraction ? `.${fraction}` : ''}`
    return `${normalizedCurrency} ${decimal}`
  }

  return new Intl.NumberFormat(locales, {
    style: 'currency',
    currency: normalizedCurrency,
    currencyDisplay,
    minimumFractionDigits: fractionDigits,
    maximumFractionDigits: fractionDigits,
  })
    .formatToParts(templateAmount)
    .map((part) => (part.type === 'fraction' ? fraction : part.value))
    .join('')
}

export function formatMicrosForInput(micros: number) {
  if (!isValidMicros(micros)) return ''
  const sign = micros < 0 ? '-' : ''
  const absoluteMicros = Math.abs(micros)
  const whole = Math.floor(absoluteMicros / MICROS_PER_CNY)
  const fraction = String(absoluteMicros % MICROS_PER_CNY)
    .padStart(6, '0')
    .replace(/0+$/, '')
  return fraction ? `${sign}${whole}.${fraction}` : `${sign}${whole}`
}

export function parseCnyInputToMicros(value: string) {
  const match = value.trim().match(/^(\d+)(?:\.(\d{0,6}))?$/)
  if (!match) return null

  const whole = BigInt(match[1])
  const fraction = BigInt((match[2] || '').padEnd(6, '0'))
  const micros = whole * BigInt(MICROS_PER_CNY) + fraction
  if (micros > BigInt(MAX_CNY_MICROS)) return null
  return Number(micros)
}

export function resolveCnyMicros(value: MoneyValue) {
  if (isValidMicros(value.micros)) return value.micros
  if (
    value.amount === null ||
    value.amount === undefined ||
    !Number.isFinite(value.amount)
  ) {
    return null
  }
  const micros = Math.round(value.amount * MICROS_PER_CNY)
  return isValidMicros(micros) ? micros : null
}

function isValidMicros(value: number | null | undefined): value is number {
  return (
    typeof value === 'number' &&
    Number.isSafeInteger(value) &&
    Math.abs(value) <= MAX_CNY_MICROS
  )
}

function moneyFractionDigits(remainder: number) {
  if (remainder % 10_000 === 0) return 2

  let exactDigits = 6
  let remaining = remainder
  while (exactDigits > 0 && remaining % 10 === 0) {
    remaining /= 10
    exactDigits -= 1
  }
  return Math.max(4, exactDigits)
}
