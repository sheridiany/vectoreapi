/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import {
  ArrowRight,
  BookOpen,
  CircleAlert,
  Database,
  Gauge,
  RefreshCw,
  Search,
  Sparkles,
  SlidersHorizontal,
} from "lucide-react";
import { useMemo, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { StatusBadge } from "@/components/status-badge";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";

import { fetchSearchCatalog, type SearchCatalogItem } from "./api";
import { SearchShell } from "./components/search-shell";
import { formatVSearchPlatformLabel } from "./lib/platform-label";
import { formatCnyMoney } from "./money";

const ALL_PLATFORMS = "all";
const EMPTY_CATALOG: SearchCatalogItem[] = [];
const PLATFORM_ORDER = [
  "douyin",
  "tiktok",
  "xiaohongshu",
  "tiktok_shop",
  "weibo",
  "wechat_mp",
  "wechat_channels",
  "youtube",
  "reddit",
  "linkedin",
];

export function SearchCatalogPage() {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");
  const [platform, setPlatform] = useState(ALL_PLATFORMS);
  const catalogQuery = useQuery({
    queryKey: ["search-catalog"],
    queryFn: fetchSearchCatalog,
  });

  const catalog = catalogQuery.data || EMPTY_CATALOG;
  const platforms = useMemo(() => {
    const values = new Set(
      catalog.flatMap((item) =>
        (item.supported_platforms || []).map(normalizePlatform),
      ),
    );
    return [...values].sort((left, right) => {
      const leftIndex = PLATFORM_ORDER.indexOf(left);
      const rightIndex = PLATFORM_ORDER.indexOf(right);
      if (leftIndex === -1 && rightIndex === -1)
        return left.localeCompare(right);
      if (leftIndex === -1) return 1;
      if (rightIndex === -1) return -1;
      return leftIndex - rightIndex;
    });
  }, [catalog]);
  const filteredCatalog = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase();
    return catalog.filter((item) => {
      if (
        platform !== ALL_PLATFORMS &&
        !(item.supported_platforms || [])
          .map(normalizePlatform)
          .includes(platform)
      ) {
        return false;
      }
      if (!normalizedQuery) return true;
      return `${item.name} ${item.description} ${item.category} ${(item.supported_platforms || []).join(" ")} ${(item.request_parameters || []).join(" ")} ${(item.information_fields || []).join(" ")}`
        .toLocaleLowerCase()
        .includes(normalizedQuery);
    });
  }, [catalog, platform, query]);
  const catalogedInterfaceCount = catalog.reduce(
    (total, item) => total + item.interface_count,
    0,
  );
  const callableInterfaceCount = catalog.reduce(
    (total, item) => total + getCallableInterfaceCount(item),
    0,
  );

  let catalogContent: ReactNode;
  if (catalogQuery.isLoading) {
    catalogContent = <CatalogSkeleton />;
  } else if (catalogQuery.isError) {
    catalogContent = (
      <Empty className="bg-card min-h-64 border">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <CircleAlert aria-hidden="true" />
          </EmptyMedia>
          <EmptyTitle>{t("Failed to load capability catalog")}</EmptyTitle>
          <EmptyDescription>
            {t("Check your connection and try again.")}
          </EmptyDescription>
        </EmptyHeader>
        <Button variant="outline" onClick={() => void catalogQuery.refetch()}>
          {t("Retry")}
        </Button>
      </Empty>
    );
  } else if (filteredCatalog.length === 0) {
    catalogContent = (
      <Empty className="bg-card min-h-64 border">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <Search aria-hidden="true" />
          </EmptyMedia>
          <EmptyTitle>{t("No matching capabilities")}</EmptyTitle>
          <EmptyDescription>
            {t("Try another search term or platform.")}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    );
  } else {
    catalogContent = (
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        {filteredCatalog.map((item) => (
          <CapabilityCard key={item.id} item={item} t={t} />
        ))}
      </div>
    );
  }

  return (
    <SearchShell
      title={t("vSearch capabilities")}
      description={t(
        "Browse the live vSearch catalog, availability, interface counts, and recent latency.",
      )}
      action={
        <Button render={<Link to="/search/keys" />}>
          {t("Create vSearch key")}
          <ArrowRight data-icon="inline-end" aria-hidden="true" />
        </Button>
      }
    >
      <section className="bg-card rounded-xl border p-5 shadow-sm sm:p-7">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div className="max-w-3xl">
            <p className="text-muted-foreground text-xs font-semibold tracking-[0.18em] uppercase">
              <span translate="no">{t("VECTOR EPOCH SEARCH")}</span>
            </p>
            <h2 className="mt-2 text-2xl font-semibold tracking-tight sm:text-3xl">
              {t("Find the right capability for an Agent task")}
            </h2>
            <p className="text-muted-foreground mt-3 text-sm leading-6 sm:text-base">
              {t(
                "Choose a platform to see exactly what public information vSearch can retrieve and which interfaces are ready to call.",
              )}
            </p>
          </div>
          <StatusBadge
            label={t("Live capability catalog")}
            icon={BookOpen}
            variant="info"
            copyable={false}
            size="lg"
          />
        </div>

        <div className="mt-6 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <CatalogMetric
            label={t("Supported platforms")}
            value={
              catalogQuery.isLoading ? "—" : platforms.length.toLocaleString()
            }
          />
          <CatalogMetric
            label={t("Catalog capabilities")}
            value={
              catalogQuery.isLoading ? "—" : catalog.length.toLocaleString()
            }
          />
          <CatalogMetric
            label={t("Cataloged interfaces")}
            value={
              catalogQuery.isLoading
                ? "—"
                : catalogedInterfaceCount.toLocaleString()
            }
          />
          <CatalogMetric
            label={t("Callable interfaces")}
            value={
              catalogQuery.isLoading
                ? "—"
                : callableInterfaceCount.toLocaleString()
            }
          />
        </div>
      </section>

      <Card>
        <CardContent className="space-y-4 p-4 sm:p-5">
          <label className="relative block" htmlFor="search-catalog-query">
            <Search
              className="text-muted-foreground absolute top-1/2 left-3 size-4 -translate-y-1/2"
              aria-hidden="true"
            />
            <Input
              id="search-catalog-query"
              aria-label={t("Search capability catalog")}
              className="h-10 pl-9"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder={t("Search capability catalog")}
            />
          </label>
          <div
            className="flex gap-2 overflow-x-auto pb-1"
            role="group"
            aria-label={t("Catalog platforms")}
          >
            <PlatformButton
              active={platform === ALL_PLATFORMS}
              label={t("All")}
              onClick={() => setPlatform(ALL_PLATFORMS)}
            />
            {platforms.map((item) => (
              <PlatformButton
                key={item}
                active={platform === item}
                label={formatVSearchPlatformLabel(item)}
                onClick={() => setPlatform(item)}
              />
            ))}
          </div>
        </CardContent>
      </Card>

      <section aria-labelledby="search-catalog-list-title">
        <div className="mb-3 flex flex-wrap items-end justify-between gap-2">
          <div>
            <h2
              id="search-catalog-list-title"
              className="text-lg font-semibold"
            >
              {t("Capability catalog")}
            </h2>
            {!catalogQuery.isLoading && (
              <p className="text-muted-foreground mt-1 text-sm">
                {filteredCatalog.length.toLocaleString()} /{" "}
                {catalog.length.toLocaleString()}
              </p>
            )}
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => void catalogQuery.refetch()}
            disabled={catalogQuery.isFetching}
          >
            <RefreshCw
              data-icon="inline-start"
              className={catalogQuery.isFetching ? "animate-spin" : undefined}
              aria-hidden="true"
            />
            {t("Refresh")}
          </Button>
        </div>

        {catalogContent}
      </section>
    </SearchShell>
  );
}

function CapabilityCard(props: {
  item: SearchCatalogItem;
  t: (key: string, options?: Record<string, unknown>) => string;
}) {
  const { item, t } = props;
  const callableInterfaceCount = getCallableInterfaceCount(item);
  const status = getCatalogStatus(item, callableInterfaceCount, t);
  return (
    <Card size="sm">
      <CardContent className="flex h-full flex-col p-4">
        <div className="flex items-start justify-between gap-3">
          <div className="bg-primary/10 text-primary flex size-10 shrink-0 items-center justify-center rounded-lg">
            <Sparkles className="size-5" aria-hidden="true" />
          </div>
          <StatusBadge
            label={status.label}
            variant={status.variant}
            copyable={false}
          />
        </div>
        <h3 className="mt-4 font-semibold">{item.name}</h3>
        <p className="text-muted-foreground mt-3 flex-1 text-sm leading-6">
          {item.description}
        </p>
        {(item.supported_platforms || []).length > 0 && (
          <div className="mt-4">
            <p className="text-muted-foreground text-xs font-medium">
              {t("Supported platforms")}
            </p>
            <div className="mt-2 flex flex-wrap gap-1.5">
              {item.supported_platforms?.map((platform) => (
                <Badge key={platform} variant="outline">
                  {formatVSearchPlatformLabel(platform)}
                </Badge>
              ))}
            </div>
          </div>
        )}
        <div className="mt-4 grid gap-2 border-t pt-4">
          <CapabilityDetail
            icon={Database}
            label={t(
              item.status === "available"
                ? "Accessible information"
                : "Planned information",
            )}
            values={item.information_fields || []}
            emptyLabel={t("Information contract pending verification")}
            t={t}
          />
          <CapabilityDetail
            icon={SlidersHorizontal}
            label={t("Request parameters")}
            values={item.request_parameters || []}
            emptyLabel={t("No parameters required")}
            t={t}
          />
        </div>
        <div className="text-muted-foreground mt-4 flex flex-wrap gap-x-4 gap-y-2 border-t pt-3 text-xs">
          <span>
            {t("Cataloged interfaces")}: {item.interface_count.toLocaleString()}
          </span>
          <span>
            {t("Callable interfaces")}:{" "}
            {callableInterfaceCount.toLocaleString()}
          </span>
          {item.recent_latency_ms != null && (
            <span className="inline-flex items-center gap-1">
              <Gauge className="size-3.5" aria-hidden="true" />
              {item.recent_latency_ms.toLocaleString()} ms
            </span>
          )}
          {catalogPriceLabel(item) && (
            <span className="text-foreground font-medium">
              {t("Price")}: {catalogPriceLabel(item)} / {t("call")}
            </span>
          )}
        </div>
      </CardContent>
    </Card>
  );
}

function CapabilityDetail(props: {
  icon: typeof Database;
  label: string;
  values: string[];
  emptyLabel: string;
  t: (key: string) => string;
}) {
  const Icon = props.icon;
  return (
    <div className="bg-muted/40 rounded-lg p-3">
      <div className="text-muted-foreground flex items-center gap-1.5 text-xs font-medium">
        <Icon className="size-3.5" aria-hidden="true" />
        {props.label}
      </div>
      {props.values.length > 0 ? (
        <div className="mt-2 flex flex-wrap gap-1.5">
          {props.values.map((value) => (
            <Badge key={value} variant="secondary">
              {capabilityFieldLabel(value, props.t)}
            </Badge>
          ))}
        </div>
      ) : (
        <p className="text-muted-foreground mt-2 text-xs">{props.emptyLabel}</p>
      )}
    </div>
  );
}

function capabilityFieldLabel(field: string, t: (key: string) => string) {
  const labels: Record<string, string> = {
    account_ref: "Account ID or username",
    author: "Author",
    avatar_url: "Avatar",
    bio: "Bio",
    comment_ref: "Comment ID",
    content_count: "Published content",
    content_ref: "Content ID",
    display_name: "Display name",
    follower_count: "Followers",
    following_count: "Following",
    id: "ID",
    like_count: "Likes",
    media: "Media",
    metrics: "Engagement metrics",
    platform: "Platform",
    product_ref: "Product ID",
    published_at: "Published at",
    query: "Keyword",
    rank: "Rank",
    reply_count: "Replies",
    score: "Popularity",
    text: "Text",
    title: "Title",
    type: "Data type",
    url: "URL",
    username: "Username",
    verified: "Verified",
  };
  return t(labels[field] || field);
}

function getCallableInterfaceCount(item: SearchCatalogItem) {
  if (item.available_interface_count !== undefined) {
    return item.available_interface_count;
  }
  if (item.healthy_route_count !== undefined) return item.healthy_route_count;
  if (item.status === "available" && item.enabled) return item.interface_count;
  return 0;
}

function getCatalogStatus(
  item: SearchCatalogItem,
  callableInterfaceCount: number,
  t: (key: string) => string,
) {
  if (
    item.status === "available" &&
    item.enabled &&
    callableInterfaceCount > 0
  ) {
    return { label: t("Available"), variant: "success" as const };
  }
  if (item.status === "catalog") {
    return { label: t("Preparing"), variant: "warning" as const };
  }
  return {
    label: t("Temporarily unavailable"),
    variant: "neutral" as const,
  };
}

function catalogPriceLabel(item: SearchCatalogItem) {
  if (
    item.price_min_micros === undefined ||
    item.price_max_micros === undefined ||
    (item.price_min_micros === 0 && item.price_max_micros === 0)
  ) {
    return item.cost_label || "";
  }
  const minimum = formatCnyMoney({ micros: item.price_min_micros });
  if (item.price_min_micros === item.price_max_micros) return minimum;
  return `${minimum} – ${formatCnyMoney({ micros: item.price_max_micros })}`;
}

function CatalogMetric(props: { label: string; value: string }) {
  return (
    <div className="bg-muted/50 rounded-lg p-4">
      <div className="text-muted-foreground text-xs">{props.label}</div>
      <div className="mt-1 text-xl font-semibold tabular-nums">
        {props.value}
      </div>
    </div>
  );
}

function normalizePlatform(platform: string) {
  return platform.trim().toLocaleLowerCase();
}

function PlatformButton(props: {
  active: boolean;
  label: string;
  onClick: () => void;
}) {
  return (
    <Button
      type="button"
      variant={props.active ? "default" : "outline"}
      size="sm"
      aria-pressed={props.active}
      className="shrink-0"
      onClick={props.onClick}
    >
      {props.label}
    </Button>
  );
}

function CatalogSkeleton() {
  return (
    <div
      className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3"
      aria-hidden="true"
    >
      {Array.from({ length: 6 }, (_, index) => (
        <Card key={index} size="sm">
          <CardContent className="space-y-4 p-4">
            <Skeleton className="size-10 rounded-lg" />
            <Skeleton className="h-5 w-1/2" />
            <Skeleton className="h-16 w-full" />
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
