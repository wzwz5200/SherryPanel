# 安全架构、数据库表结构与 REST API 规范

---

## 一、安全模型设计

### 1. 物理端口与路由隔离
系统启动两个完全独立的 `http.Server` 实例：
* **Player Server（默认 `:8080`）**：仅注册 `RegisterPlayer` 路由（皮肤库、玩家配对兑换、已认证玩家个人换肤 CRUD、静态图片 `/img/`）。**所有管理、RCON、配置、安装器接口在该端口上根本未注册，直接返回 `404`。**
* **Admin Server（默认 `:9090`）**：仅注册 `RegisterAdmin` 路由。除 `/api/meta`（未登录时仅返回 `admin_authenticated: false`）与 `/api/admin/login` 外，所有接口均由 `requireAdmin` 中间件强制拦截，未登录返回 `401`。

### 2. 防 SteamID64 冒充的免登录认证流程
```text
[玩家进入 CS2] --(Valve 官方 Steam 票据验真)--> [SkinBind 插件]
                                                      │
                    1. 进服自动私发 / 输入 !bind       ▼
                                          写入 wp_web_tokens
                                      (6位随机码 + 36位随机Token)
                                                      │
[玩家浏览器 :8080] --(提交 6位码 或 ?token=...)--> [Go 后端 /api/auth/redeem]
                                                      │
                         2. 校验有效 & 立即标记 used=1 ▼
                                  签发 30 天 HttpOnly HMAC Cookie
                                 (cs2_player_session = player|steamid|exp.sig)
                                                      │
[后续所有换肤请求] -----------------------------> 后端仅从 Cookie 验签取 SteamID64
                                                (彻底忽略客户端任意传入的 steamid)
```

### 3. 其他安全加固措施
* **恒定时间密码比对**：管理员密码校验使用 `crypto/subtle.ConstantTimeCompare`，防止时序侧信道攻击。
* **防爆破速率限制**：管理员登录与玩家配对码兑换均按来源 IP 限流（5 分钟内失败超过 10 次触发 `429 Too Many Requests`）。
* **最小权限与注入防护**：所有 MariaDB 查询均采用参数化预处理语句（Prepared Statements）；写入 `env.conf` / `server.cfg` 时自动剥离 shell 特殊字符（`"`、`$`、`` ` ``、换行）；`changelevel` 地图名严格匹配 `^[a-zA-Z0-9_]+$`。
* **Cookie 安全属性**：管理员 Cookie 使用 `HttpOnly + SameSite=Strict`（24 小时过期）；玩家 Cookie 使用 `HttpOnly + SameSite=Lax`（30 天过期）。

---

## 二、MariaDB 数据库表结构（`cs2_weaponpaints`）

| 表名 | 创建方 | 用途 | 唯一键 |
| :--- | :--- | :--- | :--- |
| `wp_player_skins` | WeaponPaints | 武器、匕首与手套涂装、磨损、模板、名称标签、StatTrak、印花/挂件 | `(steamid, weapon_team, weapon_defindex)` |
| `wp_player_knife` | WeaponPaints | 玩家 T(2)/CT(3) 装备的匕首类型（如 `weapon_knife_butterfly`） | `(steamid, weapon_team)` |
| `wp_player_gloves` | WeaponPaints | 玩家 T(2)/CT(3) 装备的手套 `weapon_defindex` | `(steamid, weapon_team)` |
| `wp_player_agents` | WeaponPaints | 玩家 T/CT 探员模型路径（`agent_t` / `agent_ct`） | `(steamid)` |
| `wp_player_music` | WeaponPaints | 玩家音乐盒 ID | `(steamid, weapon_team)` |
| `wp_player_pins` | WeaponPaints | 玩家收藏徽章 ID | `(steamid, weapon_team)` |
| `wp_web_tokens` | Go 后端 / SkinBind | 一次性 6 位配对码与专属链接 Token（含 `expires_at` 与 `used` 标记） | `PRIMARY KEY (token)` |

---

## 三、REST API 完整规范

### A. 公开玩家端接口（`:8080`）

| 方法与路径 | 鉴权 | 说明 |
| :--- | :--- | :--- |
| `GET /api/meta` | 公开 | 返回 `mode: "player"`、当前浏览器是否已认证、认证后的 `steamid` 与只读 A2S 服务器在线/地图/人数 |
| `GET /api/catalog` | 公开 | 返回完整简体中文皮肤库 JSON（武器分组、匕首、手套、探员、音乐盒、徽章；强缓存 1 小时） |
| `POST /api/auth/redeem` | 公开（限流） | 请求体 `{"code":"123456"}` 或 `{"token":"..."}`，验证通过后写入 30 天 `HttpOnly` 玩家 Cookie 并作废一次性码 |
| `POST /api/auth/logout` | 公开 | 清除当前浏览器玩家绑定 Cookie |
| `GET /api/loadout` | **需玩家 Cookie** | 获取当前已认证玩家在 T 与 CT 阵营的全部 6 类装备配置 |
| `PUT /api/loadout/skin` | **需玩家 Cookie** | 保存武器/匕首涂装（`team: 0双阵营/2T/3CT`、`weapon_defindex`、`weapon_paint_id`、`weapon_wear`、`weapon_seed`、`weapon_nametag`、`weapon_stattrak`） |
| `PUT /api/loadout/knife` | **需玩家 Cookie** | 切换匕首类型（`team`、`knife`） |
| `PUT /api/loadout/glove` | **需玩家 Cookie** | 切换手套并同步写入 `wp_player_gloves` 与 `wp_player_skins` |
| `PUT /api/loadout/agent` | **需玩家 Cookie** | 切换 T(`2`) 或 CT(`3`) 探员模型 |
| `PUT /api/loadout/music` | **需玩家 Cookie** | 切换音乐盒 |
| `PUT /api/loadout/pin` | **需玩家 Cookie** | 切换收藏徽章 |
| `DELETE /api/loadout` | **需玩家 Cookie** | 清空当前已认证玩家的全部皮肤配置 |

### B. 独立管理端接口（`:9090`）

| 方法与路径 | 鉴权 | 说明 |
| :--- | :--- | :--- |
| `GET /api/meta` | 公开 | 返回 `mode: "admin"`、`admin_authenticated`、运行平台 `os/arch`；已登录时额外返回 `connect_cmd` 与 A2S 状态 |
| `POST /api/admin/login` | 公开（限流） | 请求体 `{"password":"..."}`，校验通过后签发 24 小时 `HttpOnly` 管理员 Cookie |
| `POST /api/admin/logout` | 公开 | 清除管理员会话 Cookie |
| `GET /api/server/status` | **需管理 Cookie** | 返回 A2S 实时状态、跨平台磁盘/内存/负载指标、当前 `env.conf` 设置与路径 |
| `PUT /api/server/settings` | **需管理 Cookie** | 更新 `env.conf` 与 `server.cfg`，支持 `restart: true` 立即重启 CS2 |
| `POST /api/server/action` | **需管理 Cookie** | 启停重启 CS2 服务端（`action`: `start` / `stop` / `restart`） |
| `POST /api/server/rcon` | **需管理 Cookie** | 执行任意 CS2 Source 2 RCON 指令并返回控制台原始回显 |
| `POST /api/server/preset` | **需管理 Cookie** | 执行内置安全快捷指令宏（`changelevel`、`practice`、`competitive`、`restart_match`、`end_warmup`、`kick_bots`、`add_ct`、`add_t`、`pause`、`unpause`、`status`） |
| `GET /api/installer/status` | **需管理 Cookie** | 查询后台自动安装任务状态与实时日志行 |
| `POST /api/installer/run` | **需管理 Cookie** | 启动自动安装任务（`task`: `cs2` / `plugins` / `all`，可选 `dry_run: true`） |
| `GET /api/admin/players` | **需管理 Cookie** | 列出数据库中已有换肤记录的玩家 SteamID64 |
| `POST /api/admin/pairing` | **需管理 Cookie** | 为指定 `steamid` 生成 24 小时有效的一次性 6 位配对码与专属免登链接 |
