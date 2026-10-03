// A stub of the Prometheus HTTP API for the e2e tests. A query that is a
// number literal returns that number; any other query returns 1 for the
// series {job="stub"}.
import { createServer } from "node:http"

const port = Number(process.env.STUB_PORT ?? 26090)

function value(query: string): string {
  return /^\s*-?\d+(\.\d+)?\s*$/.test(query) ? query.trim() : "1"
}

function reply(res: import("node:http").ServerResponse, data: unknown) {
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
    const url = new URL(req.url ?? "/", "http://stub")
    const params = new URLSearchParams(body || url.search)
    const query = params.get("query") ?? ""
    const literal = value(query) === query.trim()
    switch (url.pathname) {
      case "/api/v1/query": {
        const now = Date.now() / 1000
        const data = literal
          ? {
            resultType: "scalar",
            result:     [now, value(query)],
          }
          : {
            resultType: "vector",
            result:     [{
              metric: { job: "stub" },
              value:  [now, "1"],
            }],
          }
        return reply(res, data)
      }

      case "/api/v1/query_range": {
        const [start, end, step] = ["start", "end", "step"]
          .map(k => Number(params.get(k)))
        const values = []
        for (let t = start!; t <= end!; t += step!) {
          values.push([t, value(query)])
        }
        return reply(res, {
          resultType: "matrix",
          result:     [{
            metric: { job: "stub" },
            values,
          }],
        })
      }

      case "/api/v1/labels":
        return reply(res, ["__name__", "job"])
      case "/api/v1/label/__name__/values":
        return reply(res, ["up", "stub_requests_total"])
      case "/api/v1/series":
      case "/api/v1/status/flags":
        return reply(res, url.pathname.endsWith("flags") ? {} : [])
      case "/api/v1/metadata":
        return reply(res, {})
    }

    res.writeHead(404).end()
  })
}).listen(port, "127.0.0.1")
