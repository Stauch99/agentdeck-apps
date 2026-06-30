# syntax=docker/dockerfile:1
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

# ---- deps: 根 workspace 装 node 依赖 (复刻上游 client/Dockerfile; server 非 node 包, 不参与) ----
FROM node:24-bookworm-slim AS deps
WORKDIR /app
COPY --from=src /src/package.json /src/package-lock.json ./
COPY --from=src /src/client/package.json ./client/
COPY --from=src /src/packages/design-agent/package.json ./packages/design-agent/
RUN npm ci --include=dev

# ---- build-lib: 共享设计包 ----
FROM deps AS builder-lib
COPY --from=src /src/packages/design-agent ./packages/design-agent
RUN npm run build -w packages/design-agent

# ---- build-client: next build -> standalone (源已含补丁) ----
FROM deps AS builder
COPY --from=builder-lib /app/packages/design-agent/dist ./packages/design-agent/dist
COPY --from=src /src/client ./client
WORKDIR /app/client
RUN npm run build

# ---- runtime: node + python venv + tini (单镜像双进程) ----
FROM node:24-bookworm-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends python3 python3-venv tini ca-certificates \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /app

# python 依赖: venv 在 runtime 自身的 python3 上创建 -> 解释器 symlink 有效 (避免跨阶段 venv 悬空坑)
COPY --from=src /src/server/requirements.txt /tmp/requirements.txt
RUN python3 -m venv /opt/venv && /opt/venv/bin/pip install --no-cache-dir -r /tmp/requirements.txt

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
