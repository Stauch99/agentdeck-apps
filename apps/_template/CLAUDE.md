# apps/_template/
> L2 | Parent: ../../CLAUDE.md · 起手样板,`cp -r apps/_template apps/<slug>` 后按实改

新 App 起手骨架。强制三件套:`cartridge.json`(身份/镜像/端口/卷/llm/secrets)+ `Dockerfile`(构建)+ 本 `CLAUDE.md`(L2)。

两 archetype 选一:
- 自包含型:源码 + web/ + 测试落本目录,Dockerfile 从本地源构建(样板 apps/openmusic)
- 魔改开源型:`build.sh` + lockfile,Dockerfile 构建期钉 SHA 拉上游(样板 apps/openart)

manifest 字段以平台 `BoxAssistant/server/storage.go` 的 `Cartridge` struct 为唯一真源;深校验在平台安装期。

[PROTOCOL]: cp 后改本头 slug/职责,补成员清单;字段变更对账平台 storage.go。
