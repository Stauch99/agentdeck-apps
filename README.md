# agentdeck-apps

AgentDeck 官方精品 App 源码仓。详见 [CLAUDE.md](CLAUDE.md)。

## 加一个 App

1. `cp -r apps/_template apps/<slug>`
2. 填 `apps/<slug>/cartridge.json`(字段参照 BoxAssistant `server/storage.go` 的 `Cartridge` struct)
3. 写 `apps/<slug>/Dockerfile`(自包含型从本地源;魔改开源型 build.sh 钉 SHA 拉上游)
4. 本地验证:`scripts/build.sh <slug>`(需 podman;Mac 走 Colima VM)
5. push → CI 自动构建改动 App 推 ghcr

## 骨架契约(强制三件套)

| 文件 | 职责 |
|---|---|
| cartridge.json | App 身份/镜像/端口/卷/llm/capabilities/secrets 唯一真源 |
| Dockerfile | 构建镜像,自包含不依赖仓外 |
| CLAUDE.md | L2 局部地图 |

manifest 深校验发生在平台安装期(`parseManifest` + `validateCartridges`);CI 只做 `jq` 结构快检。
