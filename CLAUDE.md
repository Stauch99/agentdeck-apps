# openmusic/
> L2 | Parent: ../docs/agentdeck/SPEC.md

AI 音乐生成应用,AgentDeck 卡带。纯 stdlib-Go 后端代理 kie.ai SunoAPI,embed 前端,曲库落卷。

成员清单
main.go: 装配入口,读 env(KIE_API_KEY/KIE_BASE_URL/OPENMUSIC_DATA_DIR/OPENMUSIC_ADDR/OPENMUSIC_ALLOW_PRIVATE_MEDIA),embed web/,signal.NotifyContext 优雅关停,ListenAndServe
suno.go: kie.ai 客户端 + Service。Generate/RecordInfo(Bearer);Submit→占位+后台 poll(ctx 可取消,10s/10min,slot 按 trackID 去重抗乱序)→Materialize→cacheMedia(fetch:http/https + LimitReader + SSRF 拦私网)
library.go: Song/Library。library.json 索引 + media/<id>.{mp3,jpg} 落卷;线程安全;占位→物化→done/error(MarkError 仅翻 generating,不动 done)
server.go: HTTP 面。/api/generate(MaxBytesReader)/api/songs /api/songs/{id}(DELETE)/media/{file} + embed 静态 + 安全头中间件(nosniff/CSP,不设 frame-ancestors)
web/: 前端。index.html(三区) styles.css(oklch 暗色) app.js(表单/轮询/播放器);全相对路径(过反代)
Dockerfile: 多阶段 → FROM scratch + ca-certs;镜像 agentdeck/openmusic:latest,容器端口 8080
devtools/fake-kie/: 仅本地 UI 验证用的假 kie server,不进镜像(.dockerignore 排除)

接入 AgentDeck:server/main.go 的 cartridges[] 一条 + demo/index.html 的 GLYPH 一条;通知走 watchAgents() 轮询 /api/songs

Law: 零依赖 · 文件 <800 行 · 密钥进 env · 真 API 直连(测试用 httptest 替 baseURL)

[PROTOCOL]: 变更时更新本头与各文件 L3 头,再核对 docs/agentdeck 与 spec,保持代码与文档同构。
