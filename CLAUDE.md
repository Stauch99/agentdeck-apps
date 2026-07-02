# agentdeck-apps — AgentDeck 官方精品 App 源码仓

> L1 仓宪法。平台仓 BoxAssistant 不拥有 App;本仓是官方 App 的唯一源码地。
> 每 App = `apps/<slug>/`,强制三件套 `cartridge.json` + `Dockerfile` + `CLAUDE.md`。
> 平台经 install 层(`cartridges/<slug>/cartridge.json`)+ 懒拉 ghcr 镜像消费,两仓只在「manifest + 镜像 tag」一处咬合。

技术栈:git subtree 迁移 · GitHub Actions + podman 推 ghcr · JSON manifest(schema-v2)。

<directory>
apps/ — 每 App 一子目录(openmusic 自包含型 · openart 魔改开源型 · _template 起手样板)
scripts/ — 共享工具(build.sh/ship.sh/index.sh 货架聚合 + lib 纯函数 + test/ 断言)
.github/workflows/ — CI:路径过滤只构建改动 App 推 ghcr;index job 聚合货架发布
</directory>

<store>
本仓同时是 AgentDeck 应用商店的数据源(store = git repo of manifests):
scripts/index.sh 把全部 apps/*/cartridge.json 聚合成一份 index.json(内嵌完整 manifest;meta.icon 相对路径内联为 data URI;_template 排除),CI index job 发布;平台一次 GET 拿整货架,不 clone 本仓(安装单元 = manifest + ghcr 镜像引用,非目录)。
manifest 的可选 meta 块(tagline/description/category/version/developer/website/icon/gallery/releaseNotes)是商店展示规格 —— 平台编排器无视,仅商店 UI 消费;category 限平台枚举(创作/设计/办公/开发/工具/娱乐/系统),icon 终态限 data:image/* 或 https。
</store>

<archetype>
自包含型:源码 + web/ + 测试在目录内,Dockerfile 从本地源构建(样板 openmusic)
魔改开源型:build.sh + lockfile,Dockerfile 构建期钉 SHA 拉上游(样板 openart)
</archetype>

Law: 一 App 一目录 · 骨架三件套恒定 · manifest 单一真源对齐平台 Cartridge struct

[PROTOCOL]: 加/删 App 时更新 apps/ 清单,再核对各 App L2(apps/<slug>/CLAUDE.md);manifest 字段变更须对账平台 server/storage.go。
