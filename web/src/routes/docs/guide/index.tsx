/*
Copyright (C) 2023-2026 QuantumNous
*/
import { createFileRoute } from '@tanstack/react-router'

import { DocsGuidePage } from '@/features/docs'

export const Route = createFileRoute('/docs/guide/')({
  component: DocsGuidePage,
})
