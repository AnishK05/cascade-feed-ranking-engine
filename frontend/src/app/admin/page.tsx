"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { formatNumber, formatPercent } from "@/lib/format";

export default function AdminRoute() {
  const metrics = useQuery({
    queryKey: ["admin-metrics"],
    queryFn: () => api.adminMetrics(),
    refetchInterval: 5000,
  });

  const data = metrics.data;

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h2 className="text-lg font-semibold">Live serving metrics</h2>
        <p className="text-sm text-zinc-500">
          Polled from Gateway <code>/api/admin/metrics</code>, which queries Prometheus. Grafana
          lives at <code>http://localhost:3001</code>.
        </p>
      </div>
      {metrics.isLoading ? <p className="text-sm text-zinc-500">Loading metrics…</p> : null}
      {metrics.error instanceof Error ? (
        <p className="text-sm text-red-600">{metrics.error.message}</p>
      ) : null}
      {data ? (
        <>
          {!data.available ? (
            <p className="rounded-md border border-amber-500/40 bg-amber-500/10 p-3 text-sm">
              Prometheus is unreachable. Start Compose Prometheus on port 9095, then generate
              traffic against the feed.
            </p>
          ) : null}
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <Metric label="Feed req/sec" value={formatNumber(data.requestsPerSecond)} />
            <Metric label="Cache hit ratio" value={formatPercent(data.cacheHitRatio)} />
            <Metric label="Feed p50" value={`${formatNumber(data.feedLatencyMs.p50)} ms`} />
            <Metric label="Feed p95" value={`${formatNumber(data.feedLatencyMs.p95)} ms`} />
            <Metric label="Feed p99" value={`${formatNumber(data.feedLatencyMs.p99)} ms`} />
            <Metric
              label="Fanout events/sec"
              value={formatNumber(data.fanoutEventsPerSecond)}
            />
            <Metric label="Fanout lag" value={`${formatNumber(data.fanoutLagMs)} ms`} />
            <Metric label="Kafka consumer lag" value={formatNumber(data.kafkaConsumerLag, 0)} />
          </div>
        </>
      ) : null}
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border border-black/10 p-4 dark:border-white/10">
      <p className="text-xs uppercase tracking-wide text-zinc-500">{label}</p>
      <p className="mt-2 font-mono text-2xl">{value}</p>
    </div>
  );
}
