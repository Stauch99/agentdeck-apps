# ============================================================================
#  OpenArt —— AgentDeck 合体卡带镜像
#  Next.js 16 standalone (web :3000) + FastAPI 透传代理 (uvicorn :8000)
#  两进程被上游硬绑 localhost (next.config rewrite /api/* -> 127.0.0.1:8000);
#  合体单容器天然满足之, server 无状态故无卷。tini 当 PID1, 任一进程退即整体退 (fail-loud)。
#  上游源码不 vendor —— 构建期按 SHA 钉死拉取 (离线主机用 build-arg 切 gh-proxy)。
# ============================================================================

ARG OPENART_REPO=https://github.com/Anil-matcha/Open-AI-Design-Agent
ARG OPENART_SHA=ee3b86a4636db02665dac622a7c4a40c350d8f05

# ---- src: 钉死 SHA 拉上游源码 + 打补丁 (单一改源点) ----
FROM node:24-bookworm-slim AS src
ARG OPENART_REPO
ARG OPENART_SHA
RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /src
RUN git clone "$OPENART_REPO" . && git checkout "$OPENART_SHA"
# 补丁: ApiContext 的 BASE_URL 写死 "http://127.0.0.1:8000" 是浏览器侧绝对 URL —— 反代下浏览器会打
# 用户自己机器, 必断。改空串走相对路径, 经 hRoot Referer 兜底 -> next rewrite -> uvicorn。
# grep 校验补丁命中, 未命中 (上游改了这行) 即构建失败 fail-loud, 不静默出坏镜像。
RUN sed -i 's#const BASE_URL = "http://127.0.0.1:8000";#const BASE_URL = "";#' client/context/ApiContext.js \
 && grep -q 'const BASE_URL = "";' client/context/ApiContext.js
# 补丁 (SSRF 收口): /api/v1/upload-binary 的 x-proxy-target-url 客户端可控, 上游 proxy_s3_upload 对它无 host 校验
# -> 经 AgentDeck 反代成为「服务端任意 URL POST」原语 (可达 host loopback / sidecar host.docker.internal:<port> /
# deck 自身 :8080)。与 OpenMusic cacheMedia 的私网防护对齐: 注入 host allowlist (仅 muapi + S3), grep fail-loud。
RUN sed -i 's#            response = await client.post(target_url, files=fields, timeout=120.0)#            from urllib.parse import urlparse as _up\n            _h = (_up(target_url).hostname or "").lower()\n            if not (_h == "api.muapi.ai" or _h.endswith(".muapi.ai") or _h.endswith(".amazonaws.com")):\n                raise HTTPException(status_code=400, detail="upload target host not allowed")\n            response = await client.post(target_url, files=fields, timeout=120.0)#' server/app/utils/muapi_helper.py \
 && grep -q "upload target host not allowed" server/app/utils/muapi_helper.py
# 补丁 (离线构建): layout.js 用 next/font/google 的 Inter, next build 会构建期拉 Google Fonts ->
# GMT+8/无公网下 "SocketError: other side closed" 必断。去掉该依赖, 退回系统字体栈 (视觉影响极小)。grep fail-loud。
RUN sed -i \
      -e '/import { Inter } from "next\/font\/google";/d' \
      -e 's#const inter = Inter({ subsets: \["latin"\] });#const inter = { className: "" };#' \
      client/app/layout.js \
 && grep -q 'const inter = { className: "" };' client/app/layout.js \
 && ! grep -q 'next/font/google' client/app/layout.js

# ---- deps: 根 workspace 装 node 依赖 (复刻上游 client/Dockerfile; server 非 node 包, 不参与) ----
FROM node:24-bookworm-slim AS deps
# 默认 vanilla npmjs.org (全局可移植); 受限网络 (如 GMT+8 Colima) 经 build-arg 切镜像源, 见 build.sh --npm-registry
ARG NPM_REGISTRY=https://registry.npmjs.org
WORKDIR /app
# 持久化 registry + 重试到 /root/.npmrc —— 关键: next build 会自下载 @next/swc 原生包, 它读 .npmrc 而非 CLI flag,
# 不写 .npmrc 则 swc 仍走 npmjs.org (经代理 IP 大文件易断)。builder FROM deps 继承此 .npmrc。
RUN npm config set registry "$NPM_REGISTRY" \
 && npm config set fetch-retries 5 \
 && npm config set fetch-retry-mintimeout 20000 \
 && npm config set fetch-retry-maxtimeout 120000 \
 && npm config set fetch-timeout 600000
COPY --from=src /src/package.json /src/package-lock.json ./
COPY --from=src /src/client/package.json ./client/
COPY --from=src /src/packages/design-agent/package.json ./packages/design-agent/
# 删 lock + npm install: 基于 lockfileVersion 3 的 lock 解析 optional 原生包 (lightningcss/oxide/@next/swc
# 平台二进制) 有时静默漏装 (npm 老 bug, 时灵时不灵) -> 构建期暴 "Cannot find module .node"。删 lock 让
# npm install 按当前构建平台 (arm64/amd64) 重新解析, 可靠装齐平台 optional。node 端可复现性以镜像 (docker save)
# 固化, 而非 node lock —— 与 server/requirements.lock 的 python 钉版互补。
# 装毕用 require() 验关键原生包真在位 (fail-loud, 早于 next build 暴问题)。
RUN rm -f package-lock.json \
 && npm install --include=dev --include=optional --no-audit --no-fund \
 && ( cd client && node -e "require('lightningcss'); require('@tailwindcss/oxide')" )

# ---- build-lib: 共享设计包 ----
FROM deps AS builder-lib
COPY --from=src /src/packages/design-agent ./packages/design-agent
RUN npm run build -w packages/design-agent

# ---- build-client: next build -> standalone (源已含补丁) ----
FROM deps AS builder
ENV NEXT_TELEMETRY_DISABLED=1
COPY --from=builder-lib /app/packages/design-agent/dist ./packages/design-agent/dist
COPY --from=src /src/client ./client
WORKDIR /app/client
RUN npm run build

# ---- runtime: node + python venv + tini (单镜像双进程) ----
FROM node:24-bookworm-slim AS runtime
# 默认 vanilla PyPI (全局可移植); 受限网络 (GMT+8 Colima) 经 build-arg 切 pip 镜像源, 见 build.sh --pip-index
ARG PIP_INDEX=https://pypi.org/simple
RUN apt-get update && apt-get install -y --no-install-recommends python3 python3-venv tini ca-certificates \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /app

# python 依赖: venv 在 runtime 自身的 python3 上创建 -> 解释器 symlink 有效 (避免跨阶段 venv 悬空坑)。
# 用本仓库 vendored requirements.lock (== 全量钉版, 含传递依赖; 由首个成功镜像 pip freeze 固化) 而非上游松散
# requirements.txt —— 持 MU_API_KEY 的进程不应随 PyPI 当下解析漂移; 与上游 SHA-pin 姿态端到端一致。
COPY requirements.lock /tmp/requirements.txt
RUN python3 -m venv /opt/venv \
 && /opt/venv/bin/pip install --no-cache-dir --index-url "$PIP_INDEX" --retries 5 --timeout 120 -r /tmp/requirements.txt

# next standalone (monorepo 输出嵌套于 client/)
COPY --from=builder /app/client/.next/standalone ./
COPY --from=builder /app/client/public ./client/public
COPY --from=builder /app/client/.next/static ./client/.next/static
# server app (FastAPI 透传代理)
COPY --from=src /src/server ./server

ENV PATH="/opt/venv/bin:$PATH" \
    NODE_ENV=production \
    PORT=3000 \
    HOSTNAME=0.0.0.0 \
    API_SUFFIX=https://api.muapi.ai
EXPOSE 3000

# tini -g: 信号广播到整个进程组, docker stop 干净收两个子进程。
# bash 监管: uvicorn(后台, next rewrite 目标 127.0.0.1:8000) + next(后台); wait -n 任一退出即收另一个并整体退。
ENTRYPOINT ["/usr/bin/tini", "-g", "--"]
CMD ["bash", "-c", "cd /app/server && uvicorn app.main:app --host 127.0.0.1 --port 8000 & SV=$!; cd /app && node client/server.js & CL=$!; wait -n; kill $SV $CL 2>/dev/null; exit 1"]
