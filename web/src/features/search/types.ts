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
export const SEARCH_CAPABILITY_GROUPS = [
  {
    id: 'web-search',
    label: 'Web search',
    description: 'Search the web through the gateway-managed providers.',
    services: [
      'Brave Search',
      'Tavily',
      'Serper',
      'Perplexity',
      'Parallel',
      'Exa',
    ],
  },
  {
    id: 'extract',
    label: 'Extract',
    description: 'Fetch and extract readable content from a URL.',
    services: ['Jina Reader', 'Firecrawl', 'Bright Data'],
  },
  {
    id: 'social',
    label: 'Social search',
    description: 'Search public social and community sources.',
    services: [
      'Instagram',
      'YouTube',
      'Reddit',
      'Bilibili',
      'Douyin',
      'TikTok',
      'X',
      'WeChat',
    ],
  },
  {
    id: 'finance',
    label: 'Finance and commerce',
    description: 'Use finance, product, and commerce search connectors.',
    services: [
      'Market Data',
      'Yahoo Finance',
      'Finnhub',
      'Tushare',
      'Taobao',
      'JD.com',
      'Amazon',
    ],
  },
  {
    id: 'news',
    label: 'News and research',
    description: 'Find current news, papers, and research references.',
    services: ['Web News', 'Research Papers', 'Industry Events'],
  },
  {
    id: 'company',
    label: 'Company and industry',
    description: 'Research companies, products, and industry information.',
    services: ['Crunchbase', 'RootData', 'Company Profiles'],
  },
  {
    id: 'travel',
    label: 'Travel and local',
    description: 'Search places, travel information, and local services.',
    services: ['QWeather', 'Booking.com', 'Local Places'],
  },
  {
    id: 'jobs',
    label: 'Jobs and opportunities',
    description: 'Find public job and opportunity listings.',
    services: ['Liepin', 'JSearch', 'Public Job Boards'],
  },
] as const

export type SearchCapabilityGroup =
  (typeof SEARCH_CAPABILITY_GROUPS)[number]['id']
