# openart/
> L2 | Parent: ../CLAUDE.md · 关联规格 ../docs/agentdeck/CARTRIDGE-CONTRACT.md · 设计稿 ../docs/superpowers/specs/2026-06-30-openart-cartridge-design.md

muapi.ai 设计 agent(海报/品牌/视频),AgentDeck 卡带。**合体单镜像 = Next.js 16 web(:3000)+ FastAPI 透传代理(uvicorn :8000)**。上游源码不 vendor,构建期按 commit SHA 钉死拉取。

## 本质
"Open-AI-Design-Agent" 是付费云 `api.muapi.ai` 的 BYO-key 薄壳:`server/` 纯透传代理(服务端持 `MU_API_KEY` 注入 `x-api-key`,**完全无状态**),`client/` 是 Next.js standalone(历史落浏览器 localStorage)。200+ 模型/编排/计费全在 muapi 云。无 `MU_API_KEY` → 空壳。

## 成员清单
Dockerfile: 多阶段合体镜像。src(钉 SHA git clone + sed 补丁 ApiContext.js 的 BASE_URL,grep fail-loud)→ deps(npm ci,复刻上游 client/Dockerfile)→ builder-lib(packages/design-agent)→ builder(next build → standalone)→ runtime(node:24-bookworm-slim + python venv + tini -g;uvicorn 后台 + next 前台,wait -n 任一退即整体退)。镜像 `agentdeck/openart:latest`,容器端口 3000
build.sh: 本地构建助手。默认按宿主架构;`--amd64`(CentOS 目标)、`--repo`(离线切 gh-proxy)、`--sha`(覆盖钉死版本)

## 拓扑要点(为什么合体而非 sidecar)
两进程被上游**硬绑 localhost**:`client/next.config.mjs` 把 `/api/*` rewrite 写死转发 `127.0.0.1:8000`(连 compose 的 `API_URL` build-arg 都是死代码)。server 无状态不需隔离。→ 合体单容器零改 `next.config` 天然满足耦合。

## 数据流(经 AgentDeck 反代)
- 浏览器 `/agent/openart/` → `hProxy`(剥前缀)→ next:3000 页面壳
- 浏览器 `/api/v1/*`、`/_next/*`(相对)→ `hRoot` Referer 兜底 → next:3000 → rewrite → `127.0.0.1:8000` uvicorn → `x-api-key` → api.muapi.ai
- Next.js **不需 basePath**;CORS 无关(浏览器→uvicorn 全程经 next 服务端转发)

## 接入 AgentDeck(机器相锚点)
- `server/storage.go` `cartridges[]` 一条:slug=openart, port=3000, `LLM{Gateway:false, Upstream:"muapi", Billing:"credits"}`,无 Volumes/Notify/Artifacts(无状态)
- `server/usage.go` `muapiCredit()` + `creditFor()` 把账户余额接进 `/api/usage`(`wantsCredits()` 认 muapi)
- `demo/index.html` `GLYPH.openart` + MARKET 一条
- `scripts/deploy.conf` 一行 `openart openart agentdeck/openart:latest`
- `MU_API_KEY` 经 `secrets.env`(EnvironmentFile)注入,不入源码/日志/git

## 安全注记
上游 `/api/v1/upload-binary` 的 `x-proxy-target-url` 客户端可控 (拿来直传 S3 预签名 URL),`proxy_s3_upload` 原无 host 校验 → 经反代成「服务端任意 URL POST」原语 (可达 host loopback / sidecar `host.docker.internal:<port>` / deck `:8080`)。Dockerfile src 阶段已注入 host allowlist 补丁 (仅 `api.muapi.ai` / `*.muapi.ai` / `*.amazonaws.com`,grep fail-loud),与 OpenMusic `cacheMedia` 私网防护对齐。注: 该 key 不经此路径外泄 (upload 不附 `x-api-key`)。纵深防御层面,容器 egress 理想上仅需达 muapi + S3。

Law: 不 vendor 上游 · 钉死 SHA · 密钥进 env · 无状态无卷 · 补丁 fail-loud · SSRF host allowlist

[PROTOCOL]: 变更时更新本头,再核对 ../docs/agentdeck 与设计稿,保持代码与文档同构。
