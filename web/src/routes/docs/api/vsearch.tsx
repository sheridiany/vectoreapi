/*
Copyright (C) 2023-2026 QuantumNous
*/
import { createFileRoute } from '@tanstack/react-router'

import { DocsVSearchPage } from '@/features/docs'

export const Route = createFileRoute('/docs/api/vsearch')({
  component: DocsVSearchPage,
})
