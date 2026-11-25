# 跨平台部署、源码构建与运维手册

本文档涵盖 **Linux（Debian/Ubuntu）** 与 **Windows** 两种环境下的单二进制部署、自定义欢迎语、从源码交叉编译以及日常开关机运维。

---

## 一、Linux 生产环境部署（Debian 12 / Ubuntu 22.04+）

### 1. 环境准备
* **磁盘空间**：CS2 完整服务端（AppID 730）在安装与校验阶段需要 **≥ 100 GB** 可用空间（推荐 110 GB+ SSD）。
* **开放防火墙端口**：
  * `TCP/UDP 27015`：CS2 游戏联机与 RCON
  * `UDP 27020`：GOTV（可选）
  * `TCP 8080`：公开玩家皮肤工坊
  * `TCP 9090`：独立管理控制台（如仅本机/SSH 隧道管理可设为 `ADMIN_BIND=127.0.0.1` 无需公网放行）

### 2. 安装 MariaDB 并初始化数据库
```bash
sudo apt-get update && sudo apt-get install -y mariadb-server
sudo mysql -u root <<'SQL'
CREATE DATABASE IF NOT EXISTS cs2_weaponpaints CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'wpuser'@'127.0.0.1' IDENTIFIED BY '请替换为强密码';
GRANT ALL PRIVILEGES ON cs2_weaponpaints.* TO 'wpuser'@'127.0.0.1';
FLUSH PRIVILEGES;
SQL
```

### 3. 运行向导并一键部署 CS2 与插件栈
```bash
chmod +x cs2-panel-linux-amd64
sudo ./cs2-panel-linux-amd64 setup
```
向导会依次提示输入端口、管理密码、安装路径、MariaDB DSN，并询问是否自动下载 CS2 服务端与全皮肤插件栈（Metamod + CounterStrikeSharp + AnyBaseLib + PlayerSettings + MenuManager + WeaponPaints + 中文皮肤图库）。也可以跳过命令行下载，直接启动服务后在管理端 `http://<IP>:9090` → **「部署与更新」** 页面可视化点击下载并查看实时日志。

### 4. 配置为 systemd 常驻服务
创建 `/etc/cs2-skins-web.env`（权限 `chmod 600`）：
```env
PLAYER_PORT=8080
ADMIN_PORT=9090
ADMIN_BIND=0.0.0.0
ADMIN_PASSWORD=请替换为高强度管理密码
SESSION_SECRET=请替换为64位随机十六进制字符串
DB_DSN=wpuser:数据库密码@tcp(127.0.0.1:3306)/cs2_weaponpaints?charset=utf8mb4&parseTime=true
DATA_DIR=/var/www/weaponpaints/data
IMG_DIR=/var/www/weaponpaints/img
CS2_ADDR=127.0.0.1:27015
CS2_DIR=/home/cs2/cs2-ds
STEAMCMD_DIR=/home/cs2/steamcmd
CS2_USER=cs2
CS2_ENV_FILE=/home/cs2/env.conf
CS2_SERVICE_NAME=cs2.service
```

配置 systemd 单元 `/etc/systemd/system/cs2-skins-web.service` 并启停：
```bash
sudo systemctl daemon-reload
sudo systemctl enable --now cs2-skins-web.service
sudo systemctl status cs2-skins-web.service
```

---

## 二、Windows 原生部署（Windows 10 / 11 / Server）

无需 WSL、无需 Bash 或 Python，全部由单文件 `cs2-panel-windows-amd64.exe` 原生驱动：

1. **零配置一键开服（新手推荐）**：
   直接双击运行 `cs2-panel-windows-amd64.exe`，程序会自动生成并打印管理员密码（同时保存至 `panel-config.json`），内嵌中英双语皮肤数据无需额外下载。
   打开控制台打印的 `http://127.0.0.1:9090` 管理地址登录后，会自动弹出 **「新手引导」**：
   - 点击 **「一键全自动安装全部组件」**，程序会自动下载官方 **便携版 MariaDB 11.4 + SteamCMD + CS2 服务端 + 全皮肤插件栈 + 内置 SkinBind v1.2.0**；
   - 自动创建 `cs2_weaponpaints` 库与 7 张数据表、自动写入 `WeaponPaints.json` 与 `SkinBind.json`、自动配置 RCON 密钥；
   - 玩家在网页保存任意涂装/匕首/手套/探员/音乐盒/徽章时，面板通过 Source 2 RCON 自动触发 `wp_refresh` **即时刷新游戏内皮肤，无需手动输入 `!wp`**。
2. **安全演练测试（不下载 71 GB 服务端）**：
   ```powershell
   $env:DRY_RUN = "true"
   .\cs2-panel-windows-amd64.exe
   ```
   打开 `http://127.0.0.1:9090` 使用控制台打印的自动密码登录，即可无损演练全自动数据库/SteamCMD/CS2/插件部署流程。
3. **命令行交互向导（可选）**：
   ```powershell
   .\cs2-panel-windows-amd64.exe setup

   # 启动服务（默认管理端绑定 127.0.0.1:9090，玩家端绑定 0.0.0.0:8080）
   .\cs2-panel-windows-amd64.exe
   ```
   在管理端点击「启动 CS2」时，程序将自动拉起 `<CS2Dir>\game\bin\win64\cs2.exe` 并跟踪进程生命周期。

---

## 三、自定义进服欢迎语与自动配对码（`SkinBind.json`）

配置文件路径：`<CS2Dir>/game/csgo/addons/counterstrikesharp/configs/plugins/SkinBind/SkinBind.json`

```json
{
  "WelcomeTitle": "欢迎来到 Sherry 的专属服务器！",
  "JoinBroadcast": true,
  "AutoSendBindCode": true,
  "CodeExpireMinutes": 10,
  "JoinDelaySeconds": 3.0
}
```
* `WelcomeTitle`：进服全服播报与私聊欢迎标题。
* `JoinBroadcast`：玩家进入时是否全服播报欢迎提示。
* `AutoSendBindCode`：玩家加载进服后是否自动生成并私聊发送 6 位配对码。
* `CodeExpireMinutes`：配对码有效分钟数（默认 10 分钟，用完即失效）。
* `JoinDelaySeconds`：玩家进服后延迟几秒发送私聊（默认 3 秒，防止被进服系统消息刷掉）。

---

## 四、从源码重新构建与交叉编译

在已安装 Go 1.24+ 与 Node.js 22+ 的 Linux 机器上：
```bash
# 1. 构建 Vue 3 前端（输出至 web/dist，供 Go go:embed 打包）
cd /opt/cs2-skins-web/web
npm install
npm run build

# 2. 交叉编译 Linux & Windows 静态单文件二进制
cd /opt/cs2-skins-web
CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -ldflags="-s -w" -o release/cs2-panel-linux-amd64 .
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o release/cs2-panel-windows-amd64.exe .

# 3. 重新编译 SkinBind CS2 伴生插件（.NET 10 SDK）
dotnet build /opt/cs2-skins-web/cs2-plugin-skinbind/SkinBind.csproj -c Release \
  -o /home/cs2/cs2-ds/game/csgo/addons/counterstrikesharp/plugins/SkinBind
```

---

## 五、GCP 按需开关机与省钱运维（`cs2-taiwan`）

* **开机游玩**：`gcloud compute instances start cs2-taiwan --zone=asia-east1-b`（开机后约 25 秒自动拉起 CS2、`cs2-skins-web` 与 MariaDB，并自动刷新 `WeaponPaints.json` 与面板公网 IP）。
* **关机停表**：`gcloud compute instances stop cs2-taiwan --zone=asia-east1-b`（关机后 CPU/内存/临时 IP 费用归零，仅保留 110 GB SSD 磁盘费约 \$11/月）。
* **无人自动关机**：内置 `cs2-autoshutdown.timer`（每 5 分钟通过 `A2S_INFO` 扣除机器人统计真人数量；开机 20 分钟保护期后，若无 SSH 登录且连续 30 分钟真人数为 0 则自动 `poweroff`）。
