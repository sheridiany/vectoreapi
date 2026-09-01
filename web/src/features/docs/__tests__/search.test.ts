/*
Copyright (C) 2023-2026 QuantumNous
*/
import { describe, expect, test } from 'vitest'

import { filterDocsSearchItems } from '../lib/search'

describe('docs search', () => {
  test('returns the initial navigation entries for an empty query', () => {
    const results = filterDocsSearchItems('')

    expect(results).toHaveLength(6)
    expect(results[0]?.label).toBe('快速开始（必读）')
  })

  test('matches a page by its title or description', () => {
    const results = filterDocsSearchItems('Chat')

    expect(results.map((result) => result.label)).toEqual(['Chat Completions'])
  })

  test('finds the vSearch API guide by product or protocol', () => {
    expect(filterDocsSearchItems('vSearch').map((result) => result.to)).toEqual(
      ['/docs/api/vsearch']
    )
    expect(filterDocsSearchItems('MCP').map((result) => result.to)).toEqual([
      '/docs/api/vsearch',
    ])
  })

  test('returns no result for an unknown document query', () => {
    expect(filterDocsSearchItems('不存在的页面')).toEqual([])
  })
})
