# CS2 Panel & Skin Workshop（安全分离式 CS2 服务端管理与免登录皮肤工坊）

一个基于 **Golang + Vue 3（Apple 人机界面指南 HIG 风格）** 的单文件跨平台 CS2 专用服务器管理与安全免登录皮肤工坊系统。编译后仅一个 **~7.5 MB** 的静态二进制可执行文件（前端静态资源通过 `go:embed` 内嵌），同时支持 **Linux (amd64)** 与 **Windows (amd64)** 原生运行。

---

## 核心特性

### 1. 纵深安全架构（玩家端与管理端物理分离）
* **双端口隔离监听**：
  * **公开玩家换肤端（默认 `:8080`）**：物理上不注册任何管理/RCON/配置/安装路由（访问 `/api/server/*` 返回 `404`），绝不泄露 GSLT、RCON 密码、进服密码或他人 SteamID。
  * **独立管理控制台（默认 `:9090`）**：独立监听，支持绑定 `127.0.0.1`（仅本机/SSH 隧道）或 `0.0.0.0`，强制管理员密码登录 + `HttpOnly + SameSite=Strict` 签名 Cookie + 5 分钟 10 次失败防爆破限流。
* **免 Steam 网页登录且防 SteamID64 冒充**：
  * 摒弃不安全的「输入任意 SteamID64 即可改皮肤」方式，也无需申请 Steam Web API Key 或登录 Steam 网页；
  * 通过 **CS2 游戏内 `SkinBind` 插件自动私发 6 位一次性配对码**（由 Valve 服务器验真真实 SteamID，无法伪造）或 **管理端发放一次性专属链接** 换取 30 天 `HttpOnly` 加密会话 Cookie；
  * 所有换肤接口强制从后端验签 Cookie 中提取玩家 `SteamID64`，忽略客户端传入的任意身份字段。

### 2. Apple 人机界面指南（HIG）视觉设计
* 原生 **SF Pro / 苹方** 字体层级与等宽数字（`tabular-nums`）；
* macOS 分栏布局 + 半透明毛玻璃侧边栏与工具栏（`backdrop-filter: saturate(180%) blur(24px)`）；
* 支持**浅色 / 深色双外观**自适应与一键切换；
* 原生分段控制器（Segmented Control）、iOS 绿色开关（StatTrak™）、macOS 表单式 Sheet 定制弹窗与顶部居中 HUD 提示胶囊。

### 3. 全套 CS2 皮肤与服务器运维能力
* **玩家皮肤工坊（`:8080`）**：
  * 覆盖全部简体中文武器皮肤（含磨损度 6 档快捷预设、图案模板 Seed 0–1000、StatTrak™ 暗金计数、自定义名称标签）、20 款匕首类型、95 款手套（含磨损）、65 款 T/CT 探员、110 款音乐盒、663 款收藏徽章；
  * 支持 **双阵营同步（T+CT）/ 仅 T / 仅 CT** 独立配置；
  * 内置本地图片托管（`/img/`），零 GitHub 图床延迟。
* **独立管理控制台（`:9090`）**：
  * **控制台与指令**：纯 Go 原生 **Source 2 RCON** 客户端，支持一键启停/重启 CS2、实时 A2S 在线地图与真人/机器人统计、一键瞬切 9 张竞技地图、一键开启**跑图练枪模式** / 恢复 **5v5 竞技模式**、Bot 管理及自由 RCON 命令行终端；
  * **服务端配置**：可视化编辑服名、默认地图、游戏模式、最大人数、端口、私服进服密码、RCON 密码、Steam GSLT 并一键保存生效；
  * **自动部署与更新**：支持新机器一键下载安装 SteamCMD、CS2 服务端（AppID 730）、Metamod、CounterStrikeSharp、WeaponPaints 及全部前置依赖与中文皮肤图库，内置 **`DRY-RUN` 安全演练模式**；
  * **玩家免登授权**：房主一键为指定 SteamID64 签发 6 位配对码与一次性免登专属链接。
* **游戏内 `SkinBind` 伴生插件（.NET 10 / CounterStrikeSharp）**：
  * 玩家进服 3 秒后自动显示可定制欢迎语（如 `欢迎来到 Sherry 的专属服务器！`）与常用指令提示；
  * 进服自动私聊发送 6 位一次性配对码与换肤网页地址（仅本人可见），并支持随时输入 `!bind` / `!skin` 重新获取。

---

## 目录结构

```text
/opt/cs2-skins-web/
├── main.go                       # 程序入口：CLI 子命令 + 双端口隔离 HTTP 服务 + go:embed 前端
├── go.mod / go.sum               # 纯 Go 依赖（仅 go-sql-driver/mysql，零 CGO）
├── internal/
│   ├── config/config.go          # 配置加载、环境变量覆盖、跨平台默认值与交互式 setup 向导
│   ├── auth/auth.go              # HMAC-SHA256 HttpOnly Cookie、防爆破限流、一次性配对码/Token 管理
│   ├── db/db.go                  # MariaDB 6 张皮肤表 CRUD 与事务写入
│   ├── catalog/catalog.go        # 简体中文皮肤库解析、武器分组与本地 /img/ 路径改写
│   ├── a2s/a2s.go                # UDP A2S_INFO 实时查服（自动扣除机器人统计真人数量）
│   ├── rcon/rcon.go              # 纯 Go Source 2 TCP RCON 协议实现
│   ├── serverctl/                # 跨平台系统监控与进程控制
│   │   ├── common.go             # env.conf / server.cfg 读写与参数消毒
│   │   ├── platform_linux.go     # Linux systemctl + statfs + /proc/meminfo
│   │   └── platform_windows.go   # Windows kernel32.dll 磁盘/内存监控 + cs2.exe 进程管理
│   ├── installer/installer.go    # Linux & Windows 自动安装器（支持 --dry-run 安全演练）
│   └── api/api.go                # 物理分离的 RegisterPlayer(:8080) 与 RegisterAdmin(:9090) 路由
├── web/                          # Vue 3 + Vite Apple HIG 前端源码
│   └── src/
│       ├── style.css             # Apple HIG 浅色/深色设计系统
│       ├── App.vue               # 自动识别 player/admin 模式、玩家配对门禁与 6 大换肤视图
│       └── components/           # SkinSheet / AdminApp / ServerConsole / ServerSettings / PlayerPairing / InstallerView
├── cs2-plugin-skinbind/          # CounterStrikeSharp .NET 10 伴生插件源码（欢迎语 + !bind 自动配对码）
├── release/                      # 交叉编译产物
│   ├── cs2-panel-linux-amd64
│   └── cs2-panel-windows-amd64.exe
└── docs/
    ├── DEPLOYMENT.md             # Linux & Windows 完整部署、构建与运维手册
    └── API_AND_SECURITY.md       # 安全模型、数据库表结构与完整 REST API 文档
```

---

## 快速开始

### 命令行用法（Linux / Windows 通用）
```bash
# 1. 运行交互式配置向导（选择端口、管理密码、安装目录、数据库，并可选择自动下载 CS2 服务端与插件栈）
./cs2-panel-linux-amd64 setup        # Windows: .\cs2-panel-windows-amd64.exe setup

# 2. 或直接一键下载安装（支持 cs2 / plugins / all，加 --dry-run 为安全演练不下载 71 GB）
./cs2-panel-linux-amd64 install all
./cs2-panel-windows-amd64.exe install all --dry-run

# 3. 启动双端口服务（公开换肤端 :8080 + 独立管理端 :9090）
./cs2-panel-linux-amd64
```

### 游戏内常用指令
| 指令 | 说明 |
| :--- | :--- |
| `!wp` | 立即从数据库刷新你在网页选好的全套武器皮肤、刀、手套、音乐盒与徽章 |
| `!bind` / `!skin` | 私聊重新获取 6 位一次性网页换肤配对码（10 分钟有效，进服也会自动发一次） |
| `!knife` / `!gloves` / `!agents` / `!music` / `!pins` | 游戏内快捷换装菜单 |
| `!st` | 切换 StatTrak™ 暗金计数器 |

---

## 延伸文档
* 📘 [跨平台部署、源码编译与日常运维手册](docs/DEPLOYMENT.md)
* 🔐 [安全架构、数据库结构与 REST API 接口规范](docs/API_AND_SECURITY.md)

> **Valve GSLT 合规提示**：`WeaponPaints` 全皮肤插件违反 Valve 社区服公开准则，建议始终开启私服进服密码（`sv_password`）或使用小号 GSLT 令牌，避免影响主 Steam 账号的 GSLT 资格。
