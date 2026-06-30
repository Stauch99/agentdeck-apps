#!/usr/bin/env bash
# ============================================================================
#  build.sh —— 本地构建 OpenArt 合体卡带镜像
#  默认按宿主架构构建 (本机 Colima 直跑); deploy.sh 走 --amd64 (CentOS 目标)。
#  离线主机用 --repo 切 gh-proxy: ./build.sh --repo https://gh-proxy.com/https://github.com/Anil-matcha/Open-AI-Design-Agent
#  Usage: ./build.sh [--amd64] [--repo URL] [--sha SHA]
# ============================================================================
set -euo pipefail
cd "$(dirname "$0")"

TAG="agentdeck/openart:latest"   # 须等于 server/storage.go 的 Cartridge.Image
PLATFORM=()
ARGS=()
while [ $# -gt 0 ]; do
  case "$1" in
    --amd64)        PLATFORM=(--platform linux/amd64); shift ;;
    --repo)         ARGS+=(--build-arg "OPENART_REPO=$2"); shift 2 ;;
    --sha)          ARGS+=(--build-arg "OPENART_SHA=$2");  shift 2 ;;
    --npm-registry) ARGS+=(--build-arg "NPM_REGISTRY=$2"); shift 2 ;;  # 受限网络切镜像源 (如 https://registry.npmmirror.com)
    --pip-index)    ARGS+=(--build-arg "PIP_INDEX=$2");    shift 2 ;;  # 受限网络切 pip 源 (如 https://pypi.tuna.tsinghua.edu.cn/simple)
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

echo "building $TAG ${PLATFORM[*]:-} ${ARGS[*]:-}"
# ${arr[@]+"${arr[@]}"} 在空数组下安全展开为空 (兼容 bash 3.2 的 set -u, 否则报 unbound)
docker build ${PLATFORM[@]+"${PLATFORM[@]}"} ${ARGS[@]+"${ARGS[@]}"} -t "$TAG" .
echo "done: $TAG"
