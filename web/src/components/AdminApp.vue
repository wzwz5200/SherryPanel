<template>
  <div v-if="!authed" class="gate-screen">
    <div class="gate-card">
      <div class="gate-icon">
        <svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
          <rect x="4" y="11" width="16" height="10" rx="2" /><path d="M8 11V7a4 4 0 0 1 8 0v4" />
        </svg>
      </div>
      <h1 style="font-size:22px;font-weight:700;letter-spacing:-0.3px;">CS2 独立管理控制台</h1>
      <p style="font-size:13px;color:var(--label-secondary);margin-top:6px;">
        此端口与公开玩家换肤端物理隔离，请输入管理员密码解锁服务器一键安装、RCON 控制台与配对管理功能。（首次启动的管理员密码已打印在运行黑框中）
      </p>
      <div style="margin-top:20px;display:flex;flex-direction:column;gap:10px;">
        <input
          v-model="password"
          type="password"
          class="input-apple"
          style="padding:10px 12px;font-size:14px;"
          placeholder="请输入管理员密码"
          @keyup.enter="login"
        />
        <button class="btn btn-accent" style="justify-content:center;padding:10px;font-size:14px;" :disabled="busy" @click="login">
          {{ busy ? '验证中...' : '解锁管理控制台' }}
        </button>
      </div>
    </div>
  </div>

  <div v-else class="app-shell">
    <aside class="sidebar">
      <div class="sidebar-brand">
        <div class="app-icon">ADM</div>
        <div>
          <div class="brand-name">CS2 管理端</div>
          <div class="brand-meta">
            <span class="status-dot" :class="{ online: meta.server?.online }"></span>
            <span>{{ meta.os }}/{{ meta.arch }}<span v-if="meta.dry_run"> · 演练</span></span>
          </div>
        </div>
      </div>

      <div>
        <div class="nav-section-label">服务器运维</div>
        <nav class="nav-list">
          <button class="nav-item" :class="{ active: tab === 'wizard' }" @click="tab = 'wizard'">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75"><path d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4M4.93 19.07l2.83-2.83M16.24 7.76l2.83-2.83" /></svg>
            <span>新手引导</span>
            <span v-if="!setupReady" class="nav-count" style="background:rgba(255,149,0,0.18);color:var(--orange);">待配</span>
          </button>
          <button class="nav-item" :class="{ active: tab === 'console' }" @click="tab = 'console'">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75"><rect x="3" y="4" width="18" height="16" rx="3" /><path d="m8 10 3 2-3 2" /><path d="M13 14h3" /></svg>
            <span>控制台与指令</span>
          </button>
          <button class="nav-item" :class="{ active: tab === 'settings' }" @click="tab = 'settings'">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75"><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33h.01a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82v.01a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" /></svg>
            <span>服务端配置</span>
          </button>
          <button class="nav-item" :class="{ active: tab === 'pairing' }" @click="tab = 'pairing'">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" /><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" /></svg>
            <span>玩家免登授权</span>
          </button>
          <button class="nav-item" :class="{ active: tab === 'installer' }" @click="tab = 'installer'">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75"><path d="M12 3v12" /><path d="m7 10 5 5 5-5" /><path d="M5 20h14" /></svg>
            <span>部署与更新</span>
          </button>
        </nav>
      </div>

      <div class="sidebar-footer">
        <div class="segmented" style="width:100%;">
          <button style="flex:1;" :class="{ active: theme === 'light' }" @click="$emit('set-theme', 'light')">浅色</button>
          <button style="flex:1;" :class="{ active: theme === 'dark' }" @click="$emit('set-theme', 'dark')">深色</button>
        </div>
        <button class="btn btn-destructive btn-sm" style="justify-content:center;" @click="logout">锁定并退出管理端</button>
      </div>
    </aside>

    <div class="main-area">
      <header class="toolbar">
        <div class="toolbar-left" style="font-size:13px;color:var(--label-secondary);">
          独立管理通道 · 已通过管理员安全认证
        </div>
        <div class="toolbar-right">
          <button v-if="meta.connect_cmd" class="btn btn-tinted" @click="copy(meta.connect_cmd, '已复制进服命令')">
            复制进服命令
          </button>
          <a class="btn" :href="`http://${playerHost}:${meta.player_port}`" target="_blank" style="text-decoration:none;">
            打开玩家皮肤端 ↗
          </a>
        </div>
      </header>
      <main class="content">
        <OnboardingWizard
          v-if="tab === 'wizard'"
          @toast="(m,e) => $emit('toast', m, e)"
          @refresh-meta="$emit('refresh-meta')"
          @setup-changed="s => setupReady = !!s.ready"
        />
        <ServerConsole v-else-if="tab === 'console'" @toast="(m,e) => $emit('toast', m, e)" @refresh-meta="$emit('refresh-meta')" />
        <ServerSettings v-else-if="tab === 'settings'" @toast="(m,e) => $emit('toast', m, e)" @refresh-meta="$emit('refresh-meta')" />
        <PlayerPairing v-else-if="tab === 'pairing'" @toast="(m,e) => $emit('toast', m, e)" />
        <InstallerView v-else-if="tab === 'installer'" @toast="(m,e) => $emit('toast', m, e)" />
      </main>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import OnboardingWizard from './OnboardingWizard.vue'
import ServerConsole from './ServerConsole.vue'
import ServerSettings from './ServerSettings.vue'
import PlayerPairing from './PlayerPairing.vue'
import InstallerView from './InstallerView.vue'

const props = defineProps({
  meta: { type: Object, required: true },
  theme: { type: String, required: true }
})
const emit = defineEmits(['toast', 'refresh-meta', 'set-theme'])

const tab = ref('console')
const password = ref('')
const busy = ref(false)
const setupReady = ref(true)
const checkedInitialSetup = ref(false)
const authed = computed(() => !!props.meta.admin_authenticated)
const playerHost = computed(() => {
  if (props.meta.public_ip && props.meta.public_ip !== '127.0.0.1') return props.meta.public_ip
  return window.location.hostname || '127.0.0.1'
})

async function checkSetupState() {
  try {
    const res = await fetch('/api/setup/state')
    if (!res.ok) return
    const st = await res.json()
    setupReady.value = !!st.ready
    if (!checkedInitialSetup.value) {
      checkedInitialSetup.value = true
      if (!st.ready) tab.value = 'wizard'
    }
  } catch {}
}

watch(authed, (val) => {
  if (val) checkSetupState()
}, { immediate: true })

async function login() {
  if (!password.value) return
  busy.value = true
  try {
    const res = await fetch('/api/admin/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ password: password.value })
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '密码错误')
    password.value = ''
    emit('toast', '管理员验证成功')
    emit('refresh-meta')
  } catch (e) {
    emit('toast', e.message, true)
  } finally {
    busy.value = false
  }
}

async function logout() {
  await fetch('/api/admin/logout', { method: 'POST' })
  checkedInitialSetup.value = false
  emit('toast', '已锁定管理控制台')
  emit('refresh-meta')
}

async function copy(text, msg) {
  try { await navigator.clipboard.writeText(text); emit('toast', msg) } catch { emit('toast', text) }
}
</script>
