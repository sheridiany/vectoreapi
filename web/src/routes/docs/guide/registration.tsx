/*
Copyright (C) 2023-2026 QuantumNous
*/
import { createFileRoute } from '@tanstack/react-router'

import { DocsRegistrationPage } from '@/features/docs'

export const Route = createFileRoute('/docs/guide/registration')({
  component: DocsRegistrationPage,
})
