<template>
  <section>
    <div class="page-header">
      <div>
        <h1 class="page-title">服务器控制台</h1>
        <p class="page-subtitle">实时监控 CS2 状态、一键启停重启、快速切图、跑图练枪与实时 RCON 控制台</p>
      </div>
      <div style="display:flex;gap:8px;flex-wrap:wrap;">
        <button class="btn" @click="refresh">刷新状态</button>
        <button class="btn btn-tinted" :disabled="busy" @click="doAction('start')">启动 CS2</button>
        <button class="btn btn-accent" :disabled="busy" @click="doAction('restart')">重启 CS2</button>
        <button class="btn btn-destructive" :disabled="busy" @click="doAction('stop')">停止 CS2</button>
      </div>
    </div>

    <!-- Live Metrics Cards (Apple HIG Bento Row) -->
    <div class="grid" style="grid-template-columns:repeat(auto-fit,minmax(220px,1fr));margin-bottom:20px;">
      <div class="card" style="cursor:default;">
        <div class="card-subtitle">运行状态</div>
        <div style="font-size:20px;font-weight:700;margin-top:6px;display:flex;align-items:center;gap:8px;">
          <span class="status-dot" :class="{ online: status.server?.online }"></span>
          <span>{{ status.server?.online ? '在线运行中' : (status.metrics?.service_active ? '启动中' : '已停止') }}</span>
        </div>
        <div class="card-subtitle" style="margin-top:6px;">{{ status.server?.name || 'CS2 Dedicated Server' }}</div>
      </div>

      <div class="card" style="cursor:default;">
        <div class="card-subtitle">当前地图与玩家</div>
        <div class="tabular" style="font-size:20px;font-weight:700;margin-top:6px;">
          {{ status.server?.map || '—' }}
        </div>
        <div class="card-subtitle tabular" style="margin-top:6px;">
          真人 {{ status.server?.players ?? 0 }} / {{ status.server?.max_players ?? 12 }} · 机器人 {{ status.server?.bots ?? 0 }}
        </div>
      </div>

      <div class="card" style="cursor:default;">
        <div class="card-subtitle">系统内存与负载</div>
        <div class="tabular" style="font-size:20px;font-weight:700;margin-top:6px;">
          {{ usedMemGB }} / {{ totalMemGB }} GB
        </div>
        <div class="card-subtitle tabular" style="margin-top:6px;">Load: {{ status.metrics?.load_avg || '—' }}</div>
      </div>

      <div class="card" style="cursor:default;">
        <div class="card-subtitle">磁盘剩余空间</div>
        <div class="tabular" style="font-size:20px;font-weight:700;margin-top:6px;">
          {{ (status.metrics?.disk_free_gb || 0).toFixed(1) }} GB 可用
        </div>
        <div class="card-subtitle tabular" style="margin-top:6px;">
          总容量 {{ (status.metrics?.disk_total_gb || 0).toFixed(0) }} GB
        </div>
      </div>
    </div>

    <!-- Quick Map Switcher -->
    <div class="settings-group" style="margin-bottom:18px;">
      <div class="settings-row">
        <div class="row-label">
          <span>一键瞬切地图（实时 RCON）</span>
          <span style="font-size:12px;color:var(--label-secondary);">点击后立即执行 changelevel</span>
        </div>
        <div style="display:flex;flex-wrap:wrap;gap:8px;margin-top:4px;">
          <button
            v-for="m in maps"
            :key="m.id"
            class="btn"
            :class="{ 'btn-accent': status.server?.map === m.id }"
            :disabled="busy"
            @click="runPreset('changelevel', m.id, `已切换地图至 ${m.label}`)"
          >
            {{ m.label }}
          </button>
        </div>
      </div>

      <!-- Quick Match & Practice Commands -->
      <div class="settings-row">
        <div class="row-label">
          <span>常用战局与练枪指令</span>
          <span style="font-size:12px;color:var(--label-secondary);">免进控制台，网页一键生效</span>
        </div>
        <div style="display:flex;flex-wrap:wrap;gap:8px;margin-top:4px;">
          <button class="btn btn-tinted" :disabled="busy" @click="runPreset('practice', '', '已开启跑图练枪模式（无限弹药/60分钟/全图购买/道具轨迹）')">
            开启跑图练枪模式
          </button>
          <button class="btn" :disabled="busy" @click="runPreset('competitive', '', '已恢复标准 5v5 竞技模式')">
            恢复标准 5v5 竞技
          </button>
          <button class="btn" :disabled="busy" @click="runPreset('restart_match', '', '已重新开始对局')">
            重新开始对局
          </button>
          <button class="btn" :disabled="busy" @click="runPreset('end_warmup', '', '已跳过热身')">
            跳过热身
          </button>
          <button class="btn" :disabled="busy" @click="runPreset('kick_bots', '', '已清空所有机器人')">
            清空机器人
          </button>
          <button class="btn" :disabled="busy" @click="runPreset('add_ct', '', '已添加 CT 机器人')">
            + CT 机器人
          </button>
          <button class="btn" :disabled="busy" @click="runPreset('add_t', '', '已添加 T 机器人')">
            + T 机器人
          </button>
          <button class="btn" :disabled="busy" @click="runPreset('pause', '', '已暂停比赛')">
            暂停比赛
          </button>
          <button class="btn" :disabled="busy" @click="runPreset('unpause', '', '已恢复比赛')">
            恢复比赛
          </button>
          <button class="btn" :disabled="busy" @click="runPreset('status', '', '已查询服务器状态')">
            查看在线玩家 (status)
          </button>
        </div>
      </div>
    </div>

    <!-- Custom RCON Console -->
    <div class="settings-group">
      <div class="settings-row">
        <div class="row-label">
          <span>自由 RCON 控制台</span>
          <span style="font-size:12px;color:var(--label-secondary);">支持任意 CS2 服务端指令（如 sv_gravity 400、mp_friendlyfire 0）</span>
        </div>
        <div style="display:flex;gap:8px;margin-top:4px;">
          <input
            v-model="rconCmd"
            class="input-apple tabular"
            placeholder="输入 CS2 控制台命令后按回车执行，例如：status / mp_restartgame 1 / sv_cheats 1"
            @keyup.enter="sendRCON"
          />
          <button class="btn btn-accent" :disabled="busy" @click="sendRCON">执行指令</button>
        </div>
        <pre class="console-box tabular">{{ consoleLog || '等待指令执行结果...' }}</pre>
      </div>
    </div>
  </section>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'

const emit = defineEmits(['toast', 'refresh-meta'])

const busy = ref(false)
const status = ref({ server: {}, metrics: {}, settings: {} })
const rconCmd = ref('')
const consoleLog = ref('')

const maps = [
  { id: 'de_mirage', label: '荒漠迷城 Mirage' },
  { id: 'de_dust2', label: '炙热沙城 II Dust2' },
  { id: 'de_inferno', label: '炼狱小镇 Inferno' },
  { id: 'de_ancient', label: '远古遗迹 Ancient' },
  { id: 'de_anubis', label: '阿努比斯 Anubis' },
  { id: 'de_nuke', label: '核子危机 Nuke' },
  { id: 'de_overpass', label: '死亡游乐园 Overpass' },
  { id: 'de_vertigo', label: '殒命大厦 Vertigo' },
  { id: 'de_train', label: '列车停放站 Train' }
]

const totalMemGB = computed(() => ((status.value.metrics?.mem_total_mb || 0) / 1024).toFixed(1))
const usedMemGB = computed(() => {
  const t = status.value.metrics?.mem_total_mb || 0
  const a = status.value.metrics?.mem_avail_mb || 0
  return (Math.max(0, t - a) / 1024).toFixed(1)
})

async function apiReq(url, opts = {}) {
  const res = await fetch(url, { headers: { 'Content-Type': 'application/json' }, ...opts })
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || '请求失败')
  return data
}

async function refresh() {
  try {
    status.value = await apiReq('/api/server/status')
    emit('refresh-meta')
  } catch (e) {
    emit('toast', e.message, true)
  }
}

async function doAction(action) {
  busy.value = true
  try {
    await apiReq('/api/server/action', { method: 'POST', body: JSON.stringify({ action }) })
    emit('toast', `已执行操作: ${action}（约 15 秒后完全就绪）`)
    setTimeout(refresh, 3000)
  } catch (e) {
    emit('toast', e.message, true)
  } finally {
    busy.value = false
  }
}

async function runPreset(preset, arg, okMsg) {
  busy.value = true
  try {
    const res = await apiReq('/api/server/preset', { method: 'POST', body: JSON.stringify({ preset, arg }) })
    consoleLog.value = `> [${preset}${arg ? ' ' + arg : ''}]\n${res.output || '(命令已执行，无额外回显)'}`
    emit('toast', okMsg)
    setTimeout(refresh, 2000)
  } catch (e) {
    consoleLog.value = `错误: ${e.message}`
    emit('toast', e.message, true)
  } finally {
    busy.value = false
  }
}

async function sendRCON() {
  const cmd = rconCmd.value.trim()
  if (!cmd) return
  busy.value = true
  try {
    const res = await apiReq('/api/server/rcon', { method: 'POST', body: JSON.stringify({ command: cmd }) })
    consoleLog.value = `> ${cmd}\n${res.output || '(命令已执行，无额外回显)'}`
    rconCmd.value = ''
    emit('toast', 'RCON 指令执行成功')
  } catch (e) {
    consoleLog.value = `> ${cmd}\n错误: ${e.message}`
    emit('toast', e.message, true)
  } finally {
    busy.value = false
  }
}

onMounted(refresh)
</script>

<style scoped>
.console-box {
  background: var(--bg-surface-secondary);
  border: 0.5px solid var(--separator);
  border-radius: 10px;
  padding: 12px 14px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.55;
  color: var(--label-primary);
  min-height: 120px;
  max-height: 300px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
