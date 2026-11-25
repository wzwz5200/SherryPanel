<template>
  <section>
    <div class="page-header">
      <div>
        <h1 class="page-title">服务端配置</h1>
        <p class="page-subtitle">以系统设置方式管理 CS2 持久化启动选项（自动同步写入 env.conf 与 server.cfg）</p>
      </div>
      <div style="display:flex;gap:8px;">
        <button class="btn" :disabled="saving" @click="save(false)">仅保存配置</button>
        <button class="btn btn-accent" :disabled="saving" @click="save(true)">
          {{ saving ? '保存中...' : '保存并重启 CS2' }}
        </button>
      </div>
    </div>

    <div class="settings-group" style="max-width:820px;margin-bottom:18px;">
      <div class="settings-row-inline">
        <div>
          <div style="font-size:13.5px;font-weight:500;">服务器名称 (hostname)</div>
          <div style="font-size:12px;color:var(--label-secondary);">显示在玩家列表与服务器浏览器中</div>
        </div>
        <input v-model="form.hostname" class="input-apple" style="width:300px;" />
      </div>

      <div class="settings-row-inline">
        <div>
          <div style="font-size:13.5px;font-weight:500;">默认启动地图 (+map)</div>
          <div style="font-size:12px;color:var(--label-secondary);">每次开机自动加载的初始地图</div>
        </div>
        <input v-model="form.map" class="input-apple tabular" style="width:300px;" placeholder="de_mirage" />
      </div>

      <div class="settings-row-inline">
        <div>
          <div style="font-size:13.5px;font-weight:500;">游戏模式预设</div>
          <div style="font-size:12px;color:var(--label-secondary);">自动设置 game_type 与 game_mode</div>
        </div>
        <div class="segmented">
          <button :class="{ active: modeKey === '0-1' }" @click="setMode('0','1')">竞技 5v5</button>
          <button :class="{ active: modeKey === '0-0' }" @click="setMode('0','0')">休闲</button>
          <button :class="{ active: modeKey === '0-2' }" @click="setMode('0','2')">搭档 2v2</button>
          <button :class="{ active: modeKey === '1-2' }" @click="setMode('1','2')">死斗</button>
        </div>
      </div>

      <div class="settings-row-inline">
        <div>
          <div style="font-size:13.5px;font-weight:500;">最大玩家槽位 (-maxplayers)</div>
          <div style="font-size:12px;color:var(--label-secondary);">建议 10~16 人</div>
        </div>
        <input v-model="form.maxplayers" class="input-apple tabular" style="width:140px;" />
      </div>

      <div class="settings-row-inline">
        <div>
          <div style="font-size:13.5px;font-weight:500;">监听端口 (-port)</div>
          <div style="font-size:12px;color:var(--label-secondary);">默认 27015（TCP/UDP）</div>
        </div>
        <input v-model="form.port" class="input-apple tabular" style="width:140px;" />
      </div>
    </div>

    <div class="settings-group" style="max-width:820px;">
      <div class="settings-row-inline">
        <div>
          <div style="font-size:13.5px;font-weight:500;">私服进服密码 (sv_password)</div>
          <div style="font-size:12px;color:var(--label-secondary);">留空则公开直连；建议设置密码保护私服与 GSLT</div>
        </div>
        <input v-model="form.sv_password" class="input-apple tabular" style="width:300px;" placeholder="留空表示无密码" />
      </div>

      <div class="settings-row-inline">
        <div>
          <div style="font-size:13.5px;font-weight:500;">RCON 管理密码 (rcon_password)</div>
          <div style="font-size:12px;color:var(--label-secondary);">用于本面板控制台与游戏内远程管理</div>
        </div>
        <input v-model="form.rcon_password" class="input-apple tabular" style="width:300px;" />
      </div>

      <div class="settings-row-inline">
        <div>
          <div style="font-size:13.5px;font-weight:500;">Steam GSLT 登录令牌</div>
          <div style="font-size:12px;color:var(--label-secondary);">AppID 730 专用令牌，公网联机必需</div>
        </div>
        <input v-model="form.gslt" class="input-apple tabular" style="width:300px;" placeholder="32 位 Steam GSLT Token" />
      </div>
    </div>
  </section>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'

const emit = defineEmits(['toast', 'refresh-meta'])

const saving = ref(false)
const form = ref({
  hostname: '',
  map: 'de_mirage',
  maxplayers: '12',
  port: '27015',
  gslt: '',
  sv_password: '',
  rcon_password: '',
  game_type: '0',
  game_mode: '1'
})

const modeKey = computed(() => `${form.value.game_type}-${form.value.game_mode}`)
function setMode(gt, gm) {
  form.value.game_type = gt
  form.value.game_mode = gm
}

async function load() {
  try {
    const res = await fetch('/api/server/status')
    const data = await res.json()
    if (data.settings) form.value = { ...form.value, ...data.settings }
  } catch (e) {
    emit('toast', e.message, true)
  }
}

async function save(restart) {
  saving.value = true
  try {
    const res = await fetch('/api/server/settings', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ settings: form.value, restart })
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '保存失败')
    emit('toast', restart ? '配置已保存并正在重启 CS2 服务端' : '配置已保存（下次重启 CS2 生效）')
    emit('refresh-meta')
  } catch (e) {
    emit('toast', e.message, true)
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>
