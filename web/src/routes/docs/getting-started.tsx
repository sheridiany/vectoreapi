/*
Copyright (C) 2023-2026 QuantumNous
*/
import { createFileRoute } from '@tanstack/react-router'

import { DocsGettingStartedPage } from '@/features/docs'

export const Route = createFileRoute('/docs/getting-started')({
  component: DocsGettingStartedPage,
})
