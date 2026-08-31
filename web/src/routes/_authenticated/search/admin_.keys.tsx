/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
at your option) any later version.
*/
import { createFileRoute } from '@tanstack/react-router'

import { SearchAdminUpstreamAccountsPage } from '@/features/search/admin/upstream-accounts-page'
import { requireSearchAdmin } from '@/features/search/route-guard'

export const Route = createFileRoute('/_authenticated/search/admin_/keys')({
  beforeLoad: requireSearchAdmin,
  component: SearchAdminUpstreamAccountsPage,
})
