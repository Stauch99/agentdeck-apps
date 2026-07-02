# agentdeck-apps

AgentDeck 官方精品 App 源码仓。详见 [CLAUDE.md](CLAUDE.md)。

## 加一个 App

1. `cp -r apps/_template apps/<slug>`
2. 填 `apps/<slug>/cartridge.json`(字段参照 BoxAssistant `server/storage.go` 的 `Cartridge` struct;含商店展示的 `meta` 块 —— tagline/description/category/version/developer,可选 `icon`(仓内 `icon.svg` 相对路径,CI 内联)与 `gallery`)
3. 写 `apps/<slug>/Dockerfile`(自包含型从本地源;魔改开源型 build.sh 钉 SHA 拉上游)
4. 本地验证:`scripts/build.sh <slug>`(需 podman;Mac 走 Colima VM)
5. push → CI 自动构建改动 App 推 ghcr,并聚合发布商店货架 index.json

## 骨架契约(强制三件套)

| 文件 | 职责 |
|---|---|
| cartridge.json | App 身份/镜像/端口/卷/llm/capabilities/secrets 唯一真源 |
| Dockerfile | 构建镜像,自包含不依赖仓外 |
| CLAUDE.md | L2 局部地图 |

manifest 深校验发生在平台安装期(`parseManifest` + `validateCartridges`);CI 只做 `jq` 结构快检。

## 货架发布(应用商店数据源)

- `scripts/index.sh [out]` 聚合全部 `apps/*/cartridge.json` → `index.json`(`{store, apps:[完整 manifest]}`;`meta.icon` 相对路径内联为 data URI;`_template` 排除)。
- CI `index` job 在每次 main 构建后聚合并发布(GitHub Pages;私有仓 Pages 不可用时的兜底:聚合产物提交孤儿分支 `store-index`,平台经 raw + PAT 拉取)。
- 平台侧:`AGENTDECK_STORE_URL` 指向发布点(缺省官方 Pages),私有源配 `AGENTDECK_STORE_TOKEN`;平台 `GET /api/store/catalog` 浏览、`POST /api/store/install` 安装,不 clone 本仓。
- 测试:`bash scripts/test/index_test.sh`。
