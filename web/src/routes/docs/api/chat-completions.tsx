/*
Copyright (C) 2023-2026 QuantumNous
*/
import { createFileRoute } from '@tanstack/react-router'

import { DocsChatCompletionsPage } from '@/features/docs'

export const Route = createFileRoute('/docs/api/chat-completions')({
  component: DocsChatCompletionsPage,
})
