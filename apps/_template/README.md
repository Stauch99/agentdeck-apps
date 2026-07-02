# _template

新 App 起手样板。用法:

1. `cp -r apps/_template apps/<slug>`
2. 改 `cartridge.json`:`slug`(= 目录名)、`name`、`image`(ghcr.io/<org>/<slug>)、`port`、`glyph`、`color`;按需加 `env`/`volumes`/`llm`/`notify`/`artifacts`/`capabilities`/`secrets`(字段参照平台 `server/storage.go` Cartridge struct)
3. 填 `Dockerfile`:自包含型从本地源;魔改开源型加 `build.sh` + lockfile 钉 SHA 拉上游
4. 本地验证:`scripts/build.sh <slug>`(需 podman)
5. push → CI 构建推 ghcr
