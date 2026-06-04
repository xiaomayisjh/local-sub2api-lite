# ANT-Sub2API-Local

跨平台自用 AI API 网关桌面版，采用 Anthropic 官网风格界面（暖米白底 + 黏土橙点缀、衬线标题）。基于 [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api) 二次开发，面向个人本地使用，不面向多租户 SaaS 部署。

> 仓库目录名与 Go 模块路径仍为 `local-sub2api-lite` / `github.com/Wei-Shaw/sub2api`；`ANT-Sub2API-Local` 是产品/显示名称（窗口标题、站点名、构建产物文件名）。

[English](README.md)

## 与原项目的区别

| 项目 | 原仓库 Sub2API | 本仓库 |
|------|----------------|--------|
| 定位 | 多用户 SaaS / 自托管 | 单机自用桌面应用 |
| 数据库 | PostgreSQL | SQLite（单文件） |
| 缓存 | 独立 Redis | 进程内嵌 Redis |
| 用户 | 注册 / 多用户 | 仅默认管理员 |
| 支付 / 订阅 | 支持 | 已禁用 |
| 运行方式 | Docker / 脚本部署 | Wails 桌面 exe |

核心业务（网关转发、账号/分组/通道、管理后台、运维监控）保留；SaaS 相关能力在 `run_mode: local` 下关闭。

## 功能

- AI 网关：兼容 Claude / OpenAI / Gemini / Antigravity 等上游
- 管理后台：账号、分组、通道、系统设置
- 运维监控：错误日志、流量与指标（SQLite 下部分聚合为精简实现）
- 启动时自动生成默认 API Key，管理后台 **本地设置**（`/admin/local`）可复制

## 环境要求

- Go 1.26+
- Node.js 18+、pnpm
- Windows / macOS / Linux（桌面壳依赖系统 WebView2 / WebKit 等）

## 构建

```bash
# 前端
cd frontend
pnpm install
pnpm run build

# 桌面 exe（推荐脚本，含正确 Wails 构建标签）
cd ..
./scripts/build-desktop.ps1   # Windows PowerShell
# 或: ./scripts/build-desktop.sh

# 手动构建（必须同时带 production + embed）
cd desktop
go mod tidy
go build -tags "production,embed" -ldflags "-s -w -H windowsgui" -o ../dist/ANT-Sub2API-Local.exe .
```

产物：`dist/ANT-Sub2API-Local.exe`（约 95MB，含内嵌前端与 WebView）。

开发调试（需安装 [Wails CLI](https://wails.io/)）：

```bash
cd desktop && wails dev
```

## 首次运行

1. 启动 exe，数据与 exe **同目录** 存放（`config.yaml`、`sub2api.db` 等）；也可通过环境变量 `DATA_DIR` 指定其它路径
2. 浏览器/WebView 打开管理后台，使用 `admin@localhost` 登录（密码若为空则首次随机生成，见 **本地设置** 或同目录下的配置）
3. 在 **本地设置** 复制默认 API Key，配置到 Claude Code / Codex 等客户端：

```bash
export ANTHROPIC_BASE_URL="http://127.0.0.1:8080"
export ANTHROPIC_AUTH_TOKEN="sk-..."
```

自定义端口：

- 管理后台 **本地设置**（`/admin/local`）：修改端口、检测占用、保存后重启应用
- 或直接编辑数据目录下 `config.yaml` 的 `server.port`
- 若启动时端口被占用，会自动在附近寻找可用端口并写入配置（弹窗提示）

配置模板见 [deploy/config.example.yaml](deploy/config.example.yaml)。

## 仅运行后端（无桌面壳）

```bash
cp deploy/config.example.yaml /path/to/DATA_DIR/config.yaml
export DATA_DIR=/path/to/data
cd backend
go run -tags embed ./cmd/server
```

## 项目结构

```
local-sub2api-lite/
├── backend/     # Go 服务（网关 + API）
├── frontend/    # Vue 管理界面
├── desktop/     # Wails 桌面入口
├── deploy/      # 配置示例
└── dist/        # 构建产物（git 忽略）
```

## 许可证

本项目继承上游 [GNU LGPL v3.0](LICENSE)。二次开发部分同样以 LGPL-3.0 发布。使用上游代码请遵守原仓库许可与免责声明。

## 致谢

- 上游项目：[Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api)
