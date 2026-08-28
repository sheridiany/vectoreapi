/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { createFileRoute } from '@tanstack/react-router'

import { SearchAdminAgentKeysPage } from '@/features/search/admin/agent-keys-page'
import { requireSearchAgentKeyAdmin } from '@/features/search/route-guard'

export const Route = createFileRoute(
  '/_authenticated/search/admin_/agent-keys'
)({
  beforeLoad: requireSearchAgentKeyAdmin,
  component: SearchAdminAgentKeysPage,
})
