# k8s manifests for `captcha-solver-mcp`

These manifests follow the conventions used by the other MCP servers in the
home cluster's `agentgateway-system` namespace (compare
`home-cluster/.../base/mcp-servers/time/` and `.../mcp-servers/web-search/`).

## Resources

| File | Resource |
|------|----------|
| `deployment.yaml` | `Deployment/mcp-captcha-solver` |
| `service.yaml`    | `Service/mcp-captcha-solver` (ClusterIP, port 8085) |
| `agentgateway-backend.yaml` | `AgentgatewayBackend/mcp-captcha-solver` (per-server, hand-written) |
| `external-secret.yaml` | `ExternalSecret/1p-mcp-captcha-solver` — **optional**, only if you point at a public LLM endpoint |

## Install

The Deployment talks to the LLM through the in-cluster **agentgateway-proxy**
(ClusterIP `10.43.198.35`, port 80), which fronts the `minimax` AgentgatewayBackend
and holds the upstream `api.minimax.io` auth in the `agentgateway-minimax-auth`
secret. From the pod's perspective the endpoint is **keyless** — no
`CAPTCHA_LLM_API_KEY` is set, and no `ExternalSecret` is needed.

### 1. Add the manifests to `home-cluster`

There are two equally good layouts. Pick **layout B** (chart-driven) for
consistency with the existing per-server MCP backends unless you need a custom
`AgentgatewayPolicy`.

#### Layout A — hand-written, fully self-contained

```sh
mkdir -p ~/Documents/projects/home-cluster/kubernetes/namespaces/agentgateway-system/base/mcp-servers/captcha-solver
cp deploy/k8s/{deployment,service,agentgateway-backend}.yaml \
   ~/Documents/projects/home-cluster/kubernetes/namespaces/agentgateway-system/base/mcp-servers/captcha-solver/
```

Then edit `home-cluster/.../base/mcp-servers/kustomization.yaml`:

```yaml
   - ./captcha-solver/deployment.yaml
   - ./captcha-solver/service.yaml
   - ./captcha-solver/agentgateway-backend.yaml
```

#### Layout B — chart-driven (recommended, matches existing per-server MCPs)

Copy `deployment.yaml` and `service.yaml` exactly as in Layout A, **but skip
`agentgateway-backend.yaml`** — the `agentgw-cfg` chart will generate it from
`mcpServers` in values.

Edit `home-cluster/.../agentgw-cfg/values.yaml`:

```yaml
mcpServers:
  - name: captcha-solver
    host: mcp-captcha-solver.agentgateway-system.svc.cluster.local
    port: 8085
    # path: /mcp             (default)
    # protocol: StreamableHTTP (default)
```

This produces:

- `AgentgatewayBackend/mcp-captcha-solver` (per-server, StreamableHTTP,
  Stateless — matches the other MCP backends)
- `HTTPRoute/mcp-per-server` with a rule on
  `/mcp/captcha-solver` → that backend (per-server alias)

The same `values.yaml` entry also adds this server to the umbrella
`mcp-servers` backend that the catch-all `/mcp` HTTPRoute uses.

### 2. (Only if you switch the BASE_URL) Add the ExternalSecret

Skip this section if you keep the default
`http://agentgateway-proxy.agentgateway-system.svc.cluster.local/v1`. The
in-cluster gateway is keyless for cluster callers.

If you change `CAPTCHA_LLM_BASE_URL` to a public endpoint (e.g.
`https://llmgw.h.raczylo.com/v1` from outside the cluster, or
`https://api.openai.com/v1`), append `external-secret.yaml` to
`home-cluster/.../base/mcp-servers/eso.yaml` and re-add the
`valueFrom: secretKeyRef` block in the Deployment for `CAPTCHA_LLM_API_KEY`.

### 3. Build & push the image

The manifests reference `ghcr.io/lukaszraczylo/mcp-captchasolver:latest`. Build
and push from this repo:

```sh
docker buildx build --builder multi-arch \
  --platform linux/amd64,linux/arm64 \
  -t ghcr.io/lukaszraczylo/mcp-captchasolver:latest \
  -t ghcr.io/lukaszraczylo/mcp-captchasolver:$(git rev-parse --short HEAD) \
  --push .
```

(The `multi-registry-secret` patch in
`home-cluster/.../base/mcp-servers/kustomization.yaml` already grants the
service account permission to pull `ghcr.io/lukaszraczylo/mcp-*` images.)

### 4. Verify

```sh
kubectl --context admin@home -n agentgateway-system \
  rollout status deploy/mcp-captcha-solver
kubectl --context admin@home -n agentgateway-system \
  get agentgatewaybackend mcp-captcha-solver
curl -s http://10.0.2.101/mcp/captcha-solver \
  -H 'Accept: application/json, text/event-stream' \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}'
```

The last call should respond with a `serverInfo` block naming
`captcha-solver-mcp` v0.1.0 (or whatever `CAPTCHA_VERSION` was baked in at
build time). From inside a debug pod:

```sh
kubectl --context admin@home -n agentgateway-system exec -it deploy/mcp-captcha-solver -- \
  wget -qO- http://agentgateway-proxy.agentgateway-system.svc.cluster.local/v1/models | head
```

should list `minimax/MiniMax-M3` (proving the in-cluster path works).

## Port choice

`8085` is the next free port in the 80xx range used by the other MCP servers
(8081, 8083, 8084, 8087, 8088, 8089, 8092 are taken). Bump the `containerPort`,
`Service.port`, `Deployment` env `CAPTCHA_HTTP_ADDR`, and the `mcpServers`
`port:` if you want a different one.

## Resource sizing

Vision models can be heavier than text-only ones, so the limits (`1 CPU`,
`512Mi`) are a step above `mcp-time` (`200m`, `128Mi`) and on par with
`mcp-web-search`. Tune after observing real load.
