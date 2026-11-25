<template>
  <section>
    <div class="page-header">
      <div>
        <h1 class="page-title">新手向导 · 零配置一键开服</h1>
        <p class="page-subtitle">
          你不需要懂 SteamCMD、MariaDB 或插件配置。按下面 3 步点击即可自动下载服务端、部署皮肤系统，保存皮肤后游戏内自动刷新生效。
        </p>
      </div>
      <div style="display:flex;gap:8px;align-items:center;">
        <span v-if="st.dry_run" class="pill-tag orange">DRY-RUN 安全演练模式</span>
        <span v-else-if="st.ready" class="pill-tag" style="background:rgba(52,199,89,0.14);color:var(--green);">全部组件已就绪</span>
        <span v-else class="pill-tag orange">待完成初始化</span>
      </div>
    </div>

    <!-- 4 Readiness Cards -->
    <div class="grid" style="margin-bottom:18px;grid-template-columns:repeat(auto-fill,minmax(210px,1fr));">
      <div class="card" style="cursor:default;">
        <div class="card-subtitle">1. SteamCMD 下载器</div>
        <div style="display:flex;align-items:center;gap:8px;margin-top:6px;">
          <span class="status-dot" :class="{ online: st.steamcmd_ready }"></span>
          <span style="font-size:17px;font-weight:700;">{{ st.steamcmd_ready ? '已就绪' : '待自动下载' }}</span>
        </div>
        <div class="card-subtitle" style="margin-top:6px;word-break:break-all;">{{ st.steamcmd_dir || '-' }}</div>
      </div>

      <div class="card" style="cursor:default;">
        <div class="card-subtitle">2. CS2 服务端 (~71 GB)</div>
        <div style="display:flex;align-items:center;gap:8px;margin-top:6px;">
          <span class="status-dot" :class="{ online: st.cs2_installed }"></span>
          <span style="font-size:17px;font-weight:700;">{{ st.cs2_installed ? '已安装' : '待下载安装' }}</span>
        </div>
        <div class="card-subtitle tabular" style="margin-top:6px;">
          当前磁盘可用 {{ (st.disk_free_gb || 0).toFixed(1) }} GB
        </div>
      </div>

      <div class="card" style="cursor:default;">
        <div class="card-subtitle">3. 皮肤数据库 (自动托管)</div>
        <div style="display:flex;align-items:center;gap:8px;margin-top:6px;">
          <span class="status-dot" :class="{ online: st.db_connected }"></span>
          <span style="font-size:17px;font-weight:700;">{{ st.db_connected ? '已自动连通' : '待自动初始化' }}</span>
        </div>
        <div class="card-subtitle" style="margin-top:6px;">自动拉起 MariaDB 与 7 张数据表</div>
      </div>

      <div class="card" style="cursor:default;">
        <div class="card-subtitle">4. 全皮肤插件栈 (自动刷新)</div>
        <div style="display:flex;align-items:center;gap:8px;margin-top:6px;">
          <span class="status-dot" :class="{ online: st.plugins_ready && st.skinbind_ready }"></span>
          <span style="font-size:17px;font-weight:700;">{{ (st.plugins_ready && st.skinbind_ready) ? '全部就绪' : '待安装插件' }}</span>
        </div>
        <div class="card-subtitle" style="margin-top:6px;">WeaponPaints + SkinBind + 自动 RCON</div>
      </div>
    </div>

    <!-- Plain-language Beginner Guide -->
    <div class="settings-group" style="margin-bottom:18px;">
      <div class="settings-row">
        <div style="font-size:14px;font-weight:600;">换肤是如何工作的？（新手 30 秒看懂）</div>
        <div style="font-size:12.5px;color:var(--label-secondary);line-height:1.7;margin-top:4px;">
          <div>1. <b>不修改游戏客户端、不会封大号</b>：换肤由你自己的专用 CS2 服务器加载服务端插件 + 本地数据库完成。</div>
          <div>2. <b>全自动零配置</b>：点击下方一键安装后，本工具会自动下载 SteamCMD、71 GB 官方 CS2 服务端、便携版 MariaDB 数据库与全套皮肤插件，并自动写好全部密码和配置文件。</div>
          <div>3. <b>进服即用、网页保存自动刷新</b>：进服 3 秒聊天栏会自动给你发 6 位配对码 → 在玩家网页绑定后，<b>每次保存皮肤会自动同步到游戏内，无需手动输入 !wp</b>。</div>
        </div>
      </div>
    </div>

    <!-- Step 1: Directory & Basic Server Settings -->
    <div class="settings-group" style="margin-bottom:18px;">
      <div class="settings-row">
        <div style="font-size:14px;font-weight:600;">第 1 步：选择安装目录与房间信息</div>
        <div v-if="(st.disk_free_gb || 0) > 0 && st.disk_free_gb < 90 && !st.cs2_installed"
          style="margin-top:8px;padding:8px 12px;border-radius:8px;background:rgba(255,149,0,0.12);color:var(--orange);font-size:12.5px;">
          提示：当前路径所在磁盘剩余 {{ st.disk_free_gb.toFixed(1) }} GB，不足 90 GB。建议将 CS2 安装目录改到空间更大的硬盘（如 D:\CS2Server）。
        </div>
      </div>

      <div class="settings-row">
        <div class="row-label">
          <span>CS2 服务端安装目录</span>
          <span style="font-size:12px;color:var(--label-secondary);">需约 75 GB 可用空间</span>
        </div>
        <input v-model="form.cs2_dir" class="input-apple" placeholder="例如 D:\CS2Server 或 /home/cs2/cs2-ds" />
      </div>

      <div class="settings-row">
        <div class="row-label">
          <span>SteamCMD 下载器目录</span>
          <span style="font-size:12px;color:var(--label-secondary);">仅约 30 MB</span>
        </div>
        <input v-model="form.steamcmd_dir" class="input-apple" placeholder="例如 D:\SteamCMD" />
      </div>

      <div class="settings-row-inline" style="flex-wrap:wrap;gap:12px;">
        <div style="flex:1;min-width:220px;">
          <div style="font-size:12px;color:var(--label-secondary);margin-bottom:4px;">服务器名称 (房间标题)</div>
          <input v-model="form.settings.hostname" class="input-apple" placeholder="Sherry 的专属服务器" />
        </div>
        <div style="flex:1;min-width:220px;">
          <div style="font-size:12px;color:var(--label-secondary);margin-bottom:4px;">进服聊天框欢迎语</div>
          <input v-model="form.welcome_title" class="input-apple" placeholder="欢迎来到 Sherry 的专属服务器！" />
        </div>
      </div>

      <div class="settings-row-inline" style="flex-wrap:wrap;gap:12px;">
        <div style="flex:1;min-width:200px;">
          <div style="font-size:12px;color:var(--label-secondary);margin-bottom:4px;">进服房间密码（可留空）</div>
          <input v-model="form.settings.sv_password" class="input-apple" placeholder="防止陌生人误入可填密码" />
        </div>
        <div style="flex:1;min-width:260px;">
          <div style="font-size:12px;color:var(--label-secondary);margin-bottom:4px;">
            Steam 服务器令牌 GSLT（仅自己/局域网游玩可留空，公网联机需到
            <a href="https://steamcommunity.com/dev/managegameservers" target="_blank" style="color:var(--accent);">Steam GSLT 页面</a> 申请 AppID 730）
          </div>
          <input v-model="form.settings.gslt" class="input-apple tabular" placeholder="可先留空" />
        </div>
      </div>

      <div class="settings-row-inline">
        <span style="font-size:12px;color:var(--label-secondary);">保存后会自动为你生成 RCON 安全密钥，用于网页保存后自动刷新游戏皮肤。</span>
        <button class="btn btn-tinted" :disabled="savingCfg" @click="saveConfig">
          {{ savingCfg ? '保存中...' : '保存目录与房间配置' }}
        </button>
      </div>
    </div>

    <!-- Step 2: One-Click Install -->
    <div class="settings-group" style="margin-bottom:18px;">
      <div class="settings-row-inline">
        <div>
          <div style="font-size:14px;font-weight:600;">第 2 步：一键全自动下载与部署（全托管）</div>
          <div style="font-size:12px;color:var(--label-secondary);">
            按顺序自动完成：自动托管 MariaDB 建库 → 下载 SteamCMD → 下载官方 CS2 服务端 → 安装全皮肤插件并配置自动刷新。
          </div>
        </div>
        <button class="btn btn-accent" :disabled="inst.running" @click="runTask('all')">
          {{ inst.running && inst.task === 'all' ? `部署中 (${inst.step_index || 1}/${inst.step_total || 4})...` : '一键全自动安装全部组件' }}
        </button>
      </div>

      <div class="settings-row-inline" style="flex-wrap:wrap;gap:8px;">
        <span style="font-size:12px;color:var(--label-secondary);">分步 / 单项执行：</span>
        <div style="display:flex;flex-wrap:wrap;gap:8px;">
          <button class="btn btn-sm" :disabled="inst.running" @click="runTask('database')">
            {{ st.db_connected ? '校验/修复数据库' : '1. 初始化数据库' }}
          </button>
          <button class="btn btn-sm" :disabled="inst.running" @click="runTask('steamcmd')">2. 下载 SteamCMD</button>
          <button class="btn btn-sm" :disabled="inst.running" @click="runTask('cs2')">3. 下载/更新 CS2</button>
          <button class="btn btn-sm" :disabled="inst.running" @click="runTask('plugins')">4. 安装皮肤插件栈</button>
        </div>
      </div>

      <div class="settings-row">
        <div class="row-label">
          <span>
            部署进度 · {{ inst.stage || '等待操作' }}
            <span v-if="inst.step_total" class="tabular">（步骤 {{ inst.step_index }}/{{ inst.step_total }}）</span>
          </span>
          <span v-if="inst.running" class="row-value">自动执行中（每 2 秒刷新日志）</span>
          <span v-else-if="inst.last_error" style="color:var(--red);font-size:12px;">错误: {{ inst.last_error }}</span>
        </div>
        <pre class="log-box tabular">{{ (inst.logs && inst.logs.length) ? inst.logs.join('\n') : '建议先确认上方安装目录无误，然后点击「一键全自动安装全部组件」。' }}</pre>
      </div>
    </div>

    <!-- Step 3: Start Server & Play -->
    <div class="settings-group">
      <div class="settings-row-inline">
        <div>
          <div style="font-size:14px;font-weight:600;">第 3 步：启动服务器并开始换肤</div>
          <div style="font-size:12px;color:var(--label-secondary);">
            启动后进入服务器，聊天栏会自动收到 6 位配对码，打开玩家换肤网页绑定即可享受保存即自动刷新。
          </div>
        </div>
        <div style="display:flex;gap:8px;flex-wrap:wrap;">
          <button class="btn btn-accent" :disabled="acting" @click="startServer">
            {{ st.server_running ? '重启 CS2 服务器' : '启动 CS2 服务器' }}
          </button>
          <a v-if="st.player_url" class="btn btn-tinted" :href="st.player_url" target="_blank" style="text-decoration:none;">
            打开玩家皮肤工坊 ↗
          </a>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'

const emit = defineEmits(['toast', 'refresh-meta', 'setup-changed'])

const st = ref({})
const inst = ref({ running: false, task: '', stage: '', step_index: 0, step_total: 0, last_error: '', logs: [] })
const savingCfg = ref(false)
const acting = ref(false)
const initializedForm = ref(false)
const form = ref({
  cs2_dir: '',
  steamcmd_dir: '',
  welcome_title: '',
  settings: {
    hostname: '',
    sv_password: '',
    gslt: ''
  }
})
let timer = null

async function loadState(syncForm = false) {
  try {
    const [sRes, iRes] = await Promise.all([
      fetch('/api/setup/state').then(r => r.json()),
      fetch('/api/installer/status').then(r => r.json())
    ])
    if (sRes && !sRes.error) {
      const prevReady = st.value.ready
      st.value = sRes
      if (!initializedForm.value || syncForm) {
        form.value = {
          cs2_dir: sRes.cs2_dir || '',
          steamcmd_dir: sRes.steamcmd_dir || '',
          welcome_title: sRes.welcome_title || '欢迎来到 Sherry 的专属服务器！',
          settings: {
            hostname: sRes.settings?.hostname || 'Sherry CS2 Server',
            sv_password: sRes.settings?.sv_password || '',
            gslt: sRes.settings?.gslt || ''
          }
        }
        initializedForm.value = true
      }
      if (prevReady !== sRes.ready) {
        emit('setup-changed', sRes)
      }
    }
    if (iRes && !iRes.error) {
      inst.value = iRes
    }
  } catch {}
}

async function saveConfig() {
  savingCfg.value = true
  try {
    const res = await fetch('/api/setup/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(form.value)
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '保存失败')
    if (data.state) st.value = data.state
    emit('toast', '目录与房间配置已保存（RCON 自动刷新密钥已就绪）')
    emit('refresh-meta')
  } catch (e) {
    emit('toast', e.message, true)
  } finally {
    savingCfg.value = false
  }
}

async function runTask(task) {
  if (task === 'all' || task === 'cs2') {
    if (!confirm('将先自动保存当前目录与房间配置并开始下载/部署（完整 CS2 服务端约 71 GB）。确定继续吗？')) return
    await saveConfig()
  }
  try {
    const res = await fetch('/api/installer/run', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ task })
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '启动安装失败')
    emit('toast', `已启动全自动任务: ${task}`)
    await loadState(false)
  } catch (e) {
    emit('toast', e.message, true)
  }
}

async function startServer() {
  acting.value = true
  try {
    const action = st.value.server_running ? 'restart' : 'start'
    const res = await fetch('/api/server/action', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action })
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '启动失败')
    emit('toast', action === 'restart' ? '已重启 CS2 服务器' : '已启动 CS2 服务器')
    await loadState(false)
    emit('refresh-meta')
  } catch (e) {
    emit('toast', e.message, true)
  } finally {
    acting.value = false
  }
}

onMounted(() => {
  loadState(true)
  timer = setInterval(() => loadState(false), 2000)
})

onUnmounted(() => clearInterval(timer))
</script>

<style scoped>
.log-box {
  background: var(--bg-surface-secondary);
  border: 0.5px solid var(--separator);
  border-radius: 10px;
  padding: 12px 14px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.5;
  min-height: 160px;
  max-height: 320px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
