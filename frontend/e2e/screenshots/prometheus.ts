// A fake Prometheus API with plausible data for the README screenshots.
// Each query of readme.spec.ts maps to series whose values follow a daily
// pattern with some noise; other queries return no data. The anomalies
// line up with the incidents readme.spec.ts creates: an API outage from
// 14h to 13h15m ago and a growing mail queue for the last 2 hours.
import { createServer, type ServerResponse } from "node:http"

// The spec creates its incidents right after the server started.
const started = Date.now() / 1000
const hour = 3600

/** daily rises from 0 at 04:00 UTC to 1 at 16:00 UTC. */
const daily = (t: number) => (1 - Math.cos(2 * Math.PI * (t / 86400 - 4 / 24))) / 2

/** noise returns a value in [0, 1) that is stable for t and seed. */
const noise = (t: number, seed: number) => {
  const x = Math.sin(t * 12.9898 + seed * 78.233) * 43758.5453
  return x - Math.floor(x)
}

/** since reports whether t is at most hours before the start. */
const since = (t: number, hours: number) => t >= started - hours * hour

const outage = (t: number) => since(t, 14) && !since(t, 13.25)

const latency = (base: number, seed: number) => (t: number) =>
  base + 60 * daily(t) + 25 * noise(t, seed) + (outage(t) ? 900 : 0)

const requests = (t: number) => 450 + 1100 * daily(t) + 90 * noise(t, 1)

type Series = {
  labels: Record<string, string>
  value:  (t: number) => number
}

const up = (job: string): Series[] => [{
  labels: { job },
  value:  () => 1,
}]

const series: Record<string, Series[]> = {
  'probe_success{job="website"}':           up("website"),
  'probe_success{job="api"}':               up("api"),
  'probe_success{job="dashboard"}':         up("dashboard"),
  'probe_success{job="payments"}':          up("payments"),
  'sum(mail_queue_size{queue="deferred"})': [{
    labels: {},
    value:  t => 8 + 20 * noise(t, 2) + (since(t, 2) ? (t - started + 2 * hour) / 8 : 0),
  }],
  'avg_over_time(probe_success{job="api"}[30d]) * 100': [{
    labels: {},
    value:  () => 99.968,
  }],
  "histogram_quantile(0.95, sum by (le) (rate(http_request_duration_seconds_bucket[5m]))) * 1000": [{
    labels: {},
    value:  latency(120, 3),
  }],
  "histogram_quantile(0.95, sum by (le, region) (rate(http_request_duration_seconds_bucket[5m]))) * 1000": [
    {
      labels: { region: "eu-central" },
      value:  latency(110, 4),
    },
    {
      labels: { region: "eu-west" },
      value:  latency(145, 5),
    },
  ],
  "sum(rate(http_requests_total[5m]))": [{
    labels: {},
    value:  requests,
  }],
  'sum(rate(http_requests_total{code=~"5.."}[5m])) / sum(rate(http_requests_total[5m])) * 100': [{
    labels: {},
    value:  t => 0.04 + 0.08 * noise(t, 6) + (outage(t) ? 4 + 3 * noise(t, 7) : 0),
  }],
}

function reply(res: ServerResponse, data: unknown) {
  res.writeHead(200, { "Content-Type": "application/json" })
  res.end(JSON.stringify({
    status: "success",
    data,
  }))
}

createServer((req, res) => {
  let body = ""
  req.on("data", (chunk: Buffer) => (body += chunk))
  req.on("end", () => {
    const url = new URL(req.url ?? "/", "http://fake")
    const params = new URLSearchParams(body || url.search)
    const found = series[params.get("query")?.trim() ?? ""] ?? []
    switch (url.pathname) {
      case "/api/v1/query": {
        const t = Date.now() / 1000
        return reply(res, {
          resultType: "vector",
          result:     found.map(s => ({
            metric: s.labels,
            value:  [t, String(s.value(t))],
          })),
        })
      }

      case "/api/v1/query_range": {
        const [start, end, step] = ["start", "end", "step"]
          .map(k => Number(params.get(k)))
        return reply(res, {
          resultType: "matrix",
          result:     found.map((s) => {
            const values = []
            for (let t = start!; t <= end!; t += step!) {
              values.push([t, String(s.value(t))])
            }
            return {
              metric: s.labels,
              values,
            }
          }),
        })
      }

      case "/api/v1/status/flags":
      case "/api/v1/metadata":
        return reply(res, {})
      case "/api/v1/labels":
      case "/api/v1/series":
      case "/api/v1/label/__name__/values":
        return reply(res, [])
    }

    res.writeHead(404).end()
  })
}).listen(26092, "127.0.0.1")
