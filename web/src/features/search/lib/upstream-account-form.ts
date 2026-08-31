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
import { z } from 'zod'

const ACCOUNT_STATUSES = ['healthy', 'warning', 'standby', 'paused'] as const

export const UPSTREAM_PROVIDER_OPTIONS = [
  {
    value: 'tikhub_rest',
    label: 'TikHub REST',
    baseURL: 'https://api.tikhub.io',
  },
  {
    value: 'justoneapi_rest',
    label: 'JustOneAPI REST',
    baseURL: 'https://api.justoneapi.com',
  },
] as const

export type SearchUpstreamProvider =
  (typeof UPSTREAM_PROVIDER_OPTIONS)[number]['value']

const upstreamAccountFields = {
  provider: z.enum(['justoneapi_rest', 'tikhub_rest'], {
    error: 'Choose an upstream provider',
  }),
  name: z
    .string()
    .trim()
    .min(1, 'Account name is required')
    .max(64, 'Account name must be 64 characters or fewer'),
  base_url: z
    .string()
    .trim()
    .min(1, 'Enter a valid provider API base URL')
    .max(512, 'Enter a valid provider API base URL')
    .refine(
      isAllowedUpstreamURL,
      'Use HTTPS for remote provider endpoints. HTTP is allowed only for loopback addresses.'
    ),
  pool_id: z
    .number()
    .int('Pool ID must be zero or a positive integer')
    .min(0, 'Pool ID must be zero or a positive integer'),
  weight: z
    .number()
    .int('Weight must be between 1 and 100')
    .min(1, 'Weight must be between 1 and 100')
    .max(100, 'Weight must be between 1 and 100'),
  priority: z
    .number()
    .int('Priority must be zero or a positive integer')
    .min(0, 'Priority must be zero or a positive integer'),
  status: z.enum(ACCOUNT_STATUSES),
}

export const createUpstreamAccountSchema = z.object({
  ...upstreamAccountFields,
  api_key: z
    .string()
    .trim()
    .min(1, 'Provider API key is required')
    .max(4096, 'Provider API key is too long'),
})

export const updateUpstreamAccountSchema = z.object({
  ...upstreamAccountFields,
  api_key: z.string().trim().max(4096, 'Provider API key is too long'),
})

export type UpstreamAccountFormValues = z.infer<
  typeof updateUpstreamAccountSchema
>

export const UPSTREAM_ACCOUNT_FORM_DEFAULTS: UpstreamAccountFormValues = {
  provider: 'tikhub_rest',
  name: '',
  api_key: '',
  base_url: 'https://api.tikhub.io',
  pool_id: 0,
  weight: 1,
  priority: 0,
  status: 'healthy',
}

export type UpstreamAccountServerFormError = {
  field: keyof UpstreamAccountFormValues | 'root.server'
  messageKey: string
}

export function getUpstreamAccountServerFormError(
  error: unknown
): UpstreamAccountServerFormError {
  const message = getServerMessage(error)

  if (/base[ _-]?url|https|服务地址|上游地址/i.test(message)) {
    return {
      field: 'base_url',
      messageKey: 'Check the provider API base URL and try again.',
    }
  }
  if (/provider|供应商|提供商/i.test(message)) {
    return {
      field: 'provider',
      messageKey: 'Choose a supported upstream provider and try again.',
    }
  }
  if (/secret|encrypted secret|密钥/i.test(message)) {
    return {
      field: 'api_key',
      messageKey: 'Check the provider API key and try again.',
    }
  }
  if (/account name|账号名称|账户名称/i.test(message)) {
    return {
      field: 'name',
      messageKey: 'Check the account name and try again.',
    }
  }
  if (/pool|账号池|账户池/i.test(message)) {
    return {
      field: 'pool_id',
      messageKey: 'Check the pool ID and try again.',
    }
  }
  if (/priority|优先级/i.test(message)) {
    return {
      field: 'priority',
      messageKey: 'Check the routing priority and try again.',
    }
  }
  if (/weight|routing configuration|权重|路由配置/i.test(message)) {
    return {
      field: 'weight',
      messageKey: 'Check the routing weight and try again.',
    }
  }

  return {
    field: 'root.server',
    messageKey:
      'Unable to save upstream account. Check the form and try again.',
  }
}

export function getUpstreamProviderDefaultURL(
  provider: SearchUpstreamProvider
): string {
  return (
    UPSTREAM_PROVIDER_OPTIONS.find((option) => option.value === provider)
      ?.baseURL || UPSTREAM_PROVIDER_OPTIONS[0].baseURL
  )
}

function isAllowedUpstreamURL(value: string): boolean {
  let parsed: URL
  try {
    parsed = new URL(value)
  } catch {
    return false
  }

  if (
    parsed.username ||
    parsed.password ||
    parsed.search ||
    parsed.hash ||
    !parsed.hostname
  ) {
    return false
  }
  if (parsed.protocol === 'https:') return true
  if (parsed.protocol !== 'http:') return false

  const hostname = parsed.hostname.replaceAll(/^\[|\]$/g, '').toLowerCase()
  return (
    hostname === 'localhost' ||
    hostname === '::1' ||
    /^127(?:\.\d{1,3}){3}$/.test(hostname)
  )
}

function getServerMessage(error: unknown): string {
  if (error && typeof error === 'object' && 'response' in error) {
    const response = error.response
    if (response && typeof response === 'object' && 'data' in response) {
      const data = response.data
      if (data && typeof data === 'object' && 'message' in data) {
        if (typeof data.message === 'string') return data.message
      }
    }
  }
  return error instanceof Error ? error.message : ''
}
