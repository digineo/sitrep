// A mock OpenID provider for the e2e tests. Its authorization page offers
// two members of the sign-in group, a user outside it and a cancel link;
// the token endpoint checks PKCE and signs ID tokens with a key made at
// start.
import {
  createHash,
  createSign,
  generateKeyPairSync,
  randomUUID,
} from "node:crypto"
import { createServer, type ServerResponse } from "node:http"

const port = Number(process.env.IDP_PORT ?? 26091)
const issuer = `http://127.0.0.1:${port}`
const { privateKey, publicKey } = generateKeyPairSync(
  "rsa",
  { modulusLength: 2048 },
)

const users = {
  ann: {
    name:           "Ann Admin",
    groups:         ["staff", "admins"],
    email:          "ann@example.com",
    email_verified: true,
  },
  bob: {
    name:   "Bob Staff",
    groups: ["staff"],
  },
  carol: {
    name:           "Carol Responder",
    groups:         ["admins"],
    email:          "carol@example.com",
    email_verified: true,
  },
}

// Authorization requests by code.
const codes = new Map<string, {
  user:   keyof typeof users
  params: URLSearchParams
}>()

function json(res: ServerResponse, status: number, data: unknown) {
  res.writeHead(status, { "Content-Type": "application/json" })
  res.end(JSON.stringify(data))
}

function jwt(claims: Record<string, unknown>): string {
  const part = (v: unknown) => Buffer.from(JSON.stringify(v)).toString("base64url")
  const header = {
    alg: "RS256",
    kid: "e2e",
    typ: "JWT",
  }
  const payload = `${part(header)}.${part(claims)}`
  const signature = createSign("RSA-SHA256")
    .update(payload)
    .sign(privateKey, "base64url")
  return `${payload}.${signature}`
}

function escape(s: string): string {
  return s
    .replaceAll("&", "&amp;")
    .replaceAll("\"", "&quot;")
    .replaceAll("<", "&lt;")
}

createServer((req, res) => {
  let body = ""
  req.on("data", (chunk: Buffer) => (body += chunk))
  req.on("end", () => {
    const url = new URL(req.url ?? "/", issuer)
    switch (url.pathname) {
      case "/.well-known/openid-configuration":
        return json(res, 200, {
          issuer,
          authorization_endpoint:                `${issuer}/authorize`,
          token_endpoint:                        `${issuer}/token`,
          jwks_uri:                              `${issuer}/jwks`,
          id_token_signing_alg_values_supported: ["RS256"],
        })
      case "/jwks":
        return json(res, 200, {
          keys: [{
            ...publicKey.export({ format: "jwk" }),
            alg: "RS256",
            use: "sig",
            kid: "e2e",
          }],
        })
      case "/authorize": {
        const back = (params: Record<string, string>) =>
          escape(`${url.searchParams.get("redirect_uri")}?${new URLSearchParams({
            ...params,
            state: url.searchParams.get("state") ?? "",
          })}`)
        const links = Object.keys(users).map((user) => {
          const code = randomUUID()
          codes.set(code, {
            user:   user as keyof typeof users,
            params: url.searchParams,
          })
          return `<li><a href="${back({ code })}">${user}</a></li>`
        })
        res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" })
        return res.end(
          `<!doctype html><title>Mock IdP</title><ul>${links.join("")}`
          + `<li><a href="${back({ error: "access_denied" })}">cancel</a></li>`
          + "</ul>",
        )
      }

      case "/token": {
        const form = new URLSearchParams(body)
        const grant = codes.get(form.get("code") ?? "")
        codes.delete(form.get("code") ?? "")
        const challenge = createHash("sha256")
          .update(form.get("code_verifier") ?? "")
          .digest("base64url")
        if (!grant || grant.params.get("code_challenge") !== challenge) {
          return json(res, 400, { error: "invalid_grant" })
        }

        const now = Math.floor(Date.now() / 1000)
        return json(res, 200, {
          access_token: randomUUID(),
          token_type:   "Bearer",
          expires_in:   300,
          id_token:     jwt({
            iss:   issuer,
            aud:   grant.params.get("client_id"),
            sub:   grant.user,
            iat:   now,
            exp:   now + 300,
            nonce: grant.params.get("nonce"),
            ...users[grant.user],
          }),
        })
      }
    }

    res.writeHead(404).end()
  })
}).listen(port, "127.0.0.1")
