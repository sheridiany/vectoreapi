/*
Copyright (C) 2023-2026 QuantumNous
*/
import { createFileRoute } from '@tanstack/react-router'

import { DocsChatClientsPage } from '@/features/docs'

export const Route = createFileRoute('/docs/guide/chat-clients')({
  component: DocsChatClientsPage,
})
