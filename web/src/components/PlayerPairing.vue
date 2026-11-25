<template>
  <section>
    <div class="page-header">
      <div>
        <h1 class="page-title">玩家免登授权发放</h1>
        <p class="page-subtitle">为自己或好友生成一次性 6 位配对码或专属免登链接（24 小时有效），打开后自动写入 30 天加密 Cookie，无需 Steam 登录且防他人冒充</p>
      </div>
      <button class="btn" @click="loadPlayers">刷新玩家列表</button>
    </div>

    <div class="settings-group" style="max-width:780px;margin-bottom:18px;">
      <div class="settings-row">
        <div class="row-label">
          <span>输入玩家 17 位 SteamID64</span>
          <span style="font-size:12px;color:var(--label-secondary);">也支持直接粘贴 Steam 个人主页链接</span>
        </div>
        <div style="display:flex;gap:10px;margin-top:4px;">
          <input
            v-model="steamInput"
            class="input-apple tabular"
            placeholder="例如 76561198000000000"
            @keyup.enter="createLink"
          />
          <button class="btn btn-accent" :disabled="busy" @click="createLink">
            {{ busy ? '生成中...' : '生成专属换肤链接' }}
          </button>
        </div>
      </div>

      <div class="settings-row" v-if="players.length">
        <div class="row-label"><span>数据库已有玩家（点击快速填入）</span></div>
        <div style="display:flex;flex-wrap:wrap;gap:6px;margin-top:4px;">
          <button
            v-for="id in players"
            :key="id"
            class="btn btn-sm btn-tinted tabular"
            @click="steamInput = id; createLink()"
          >
            {{ id }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="result" class="settings-group" style="max-width:780px;">
      <div class="settings-row-inline">
        <div>
          <div style="font-size:12px;color:var(--label-secondary);">目标 SteamID64</div>
          <div class="tabular" style="font-size:15px;font-weight:600;margin-top:2px;">{{ result.steamid }}</div>
        </div>
        <div style="text-align:right;">
          <div style="font-size:12px;color:var(--label-secondary);">6 位一次性配对码</div>
          <div class="tabular" style="font-size:22px;font-weight:700;color:var(--accent);letter-spacing:2px;">
            {{ result.code }}
          </div>
        </div>
      </div>

      <div class="settings-row">
        <div class="row-label"><span>一键免登专属链接（仅限单次激活，激活后自动绑定浏览器 30 天）</span></div>
        <div style="display:flex;gap:8px;margin-top:4px;">
          <input class="input-apple tabular" :value="result.magic_url" readonly />
          <button class="btn btn-accent" @click="copy(result.magic_url, '已复制专属换肤链接')">复制链接</button>
          <a class="btn btn-tinted" :href="result.magic_url" target="_blank" style="text-decoration:none;">直接打开</a>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup>
import { ref, onMounted } from 'vue'

const emit = defineEmits(['toast'])
const steamInput = ref('')
const players = ref([])
const busy = ref(false)
const result = ref(null)

async function loadPlayers() {
  try {
    const res = await fetch('/api/admin/players')
    const data = await res.json()
    if (res.ok) players.value = data.players || []
  } catch {}
}

async function createLink() {
  const m = steamInput.value.match(/7656119\d{10}/)
  if (!m) {
    emit('toast', '请输入有效的 17 位 SteamID64（以 7656119 开头）', true)
    return
  }
  busy.value = true
  try {
    const res = await fetch('/api/admin/pairing', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ steamid: m[0] })
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '生成失败')
    result.value = data
    emit('toast', '专属免登链接与 6 位配对码已生成')
  } catch (e) {
    emit('toast', e.message, true)
  } finally {
    busy.value = false
  }
}

async function copy(text, msg) {
  try { await navigator.clipboard.writeText(text); emit('toast', msg) } catch { emit('toast', text) }
}

onMounted(loadPlayers)
</script>
