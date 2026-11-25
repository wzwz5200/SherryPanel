<template>
  <section>
    <div class="page-header">
      <div>
        <h1 class="page-title">服务器控制台</h1>
        <p class="page-subtitle">实时状态监控、自由模块化练枪/战局参数开关、一键换图与原生 Source 2 RCON 终端</p>
      </div>
      <div style="display:flex;gap:8px;flex-wrap:wrap;">
        <button class="btn" @click="refreshAll">刷新状态</button>
        <button class="btn btn-tinted" :disabled="busy" @click="doAction('start')">启动 CS2</button>
        <button class="btn btn-accent" :disabled="busy" @click="doAction('restart')">重启 CS2</button>
        <button class="btn btn-destructive" :disabled="busy" @click="doAction('stop')">停止 CS2</button>
      </div>
    </div>

    <!-- Live Metrics Bento Row -->
    <div class="grid" style="grid-template-columns:repeat(auto-fit,minmax(215px,1fr));margin-bottom:20px;">
      <div class="card" style="cursor:default;">
        <div class="card-subtitle">运行状态</div>
        <div style="font-size:19px;font-weight:700;margin-top:6px;display:flex;align-items:center;gap:8px;">
          <span class="status-dot" :class="{ online: status.server?.online }"></span>
          <span>{{ status.server?.online ? '在线运行中' : (status.metrics?.service_active ? '启动中' : '已停止') }}</span>
        </div>
        <div class="card-subtitle" style="margin-top:6px;">{{ status.server?.name || 'CS2 Dedicated Server' }}</div>
      </div>

      <div class="card" style="cursor:default;">
        <div class="card-subtitle">当前地图与玩家</div>
        <div class="tabular" style="font-size:19px;font-weight:700;margin-top:6px;">{{ status.server?.map || '—' }}</div>
        <div class="card-subtitle tabular" style="margin-top:6px;">
          真人 {{ status.server?.players ?? 0 }} / {{ status.server?.max_players ?? 12 }} · Bot {{ status.server?.bots ?? 0 }}
        </div>
      </div>

      <div class="card" style="cursor:default;">
        <div class="card-subtitle">内存与系统负载</div>
        <div class="tabular" style="font-size:19px;font-weight:700;margin-top:6px;">{{ usedMemGB }} / {{ totalMemGB }} GB</div>
        <div class="card-subtitle tabular" style="margin-top:6px;">{{ status.metrics?.load_avg || '—' }}</div>
      </div>

      <div class="card" style="cursor:default;">
        <div class="card-subtitle">磁盘剩余空间</div>
        <div class="tabular" style="font-size:19px;font-weight:700;margin-top:6px;">{{ (status.metrics?.disk_free_gb || 0).toFixed(1) }} GB 可用</div>
        <div class="card-subtitle tabular" style="margin-top:6px;">总容量 {{ (status.metrics?.disk_total_gb || 0).toFixed(0) }} GB</div>
      </div>
    </div>

    <!-- Quick Map Switcher -->
    <div class="settings-group" style="margin-bottom:18px;">
      <div class="settings-row">
        <div class="row-label">
          <span>一键瞬切地图</span>
          <span style="font-size:12px;color:var(--label-secondary);">通过 RCON 即刻切换地图</span>
        </div>
        <div style="display:flex;flex-wrap:wrap;gap:7px;margin-top:4px;">
          <button
            v-for="m in maps"
            :key="m.id"
            class="btn btn-sm"
            :class="{ 'btn-accent': status.server?.map === m.id }"
            :disabled="busy"
            @click="runPreset('changelevel', m.id, `已切换地图至 ${m.label}`)"
          >
            {{ m.label }}
          </button>
        </div>
      </div>
    </div>

    <!-- Quick Scenario Presets (Still freely customizable below!) -->
    <div class="settings-group" style="margin-bottom:18px;">
      <div class="settings-row">
        <div class="row-label">
          <span>场景快捷模板</span>
          <span style="font-size:12px;color:var(--label-secondary);">套用模板后，仍可在下方自由开关任意单项（如关闭子弹落点）</span>
        </div>
        <div style="display:flex;flex-wrap:wrap;gap:8px;margin-top:4px;">
          <button class="btn btn-accent" :disabled="busy" @click="applyTemplate('practice_full')">跑图 / 练道具全开</button>
          <button class="btn btn-tinted" :disabled="busy" @click="applyTemplate('practice_clean')">纯净练枪（关子弹落点）</button>
          <button class="btn" :disabled="busy" @click="applyTemplate('deathmatch')">死斗热身（即时复活）</button>
          <button class="btn" :disabled="busy" @click="applyTemplate('competitive')">恢复标准 5v5 竞技</button>
          <span style="flex:1;"></span>
          <button class="btn btn-sm" :disabled="busy" @click="runPreset('restart_match', '', '已重开当前对局')">重开对局</button>
          <button class="btn btn-sm" :disabled="busy" @click="runPreset('end_warmup', '', '已跳过热身')">跳过热身</button>
          <button class="btn btn-sm" :disabled="busy" @click="runPreset('pause', '', '已暂停比赛')">暂停</button>
          <button class="btn btn-sm" :disabled="busy" @click="runPreset('unpause', '', '已恢复比赛')">恢复</button>
          <button class="btn btn-sm" :disabled="busy" @click="runCvarAction('swap_teams', '已交换双方阵营')">换边</button>
        </div>
      </div>
    </div>

    <!-- Modular Two-Column Apple System Settings Controls -->
    <div class="console-two-col">
      <!-- Column 1: Practice & Visual Aids -->
      <div class="settings-group">
        <div class="settings-row" style="background:var(--bg-surface-secondary);padding:9px 14px;">
          <span style="font-size:12px;font-weight:600;color:var(--label-secondary);">练枪与视觉辅助（每项点选即刻生效）</span>
        </div>

        <div class="settings-row-inline">
          <div>
            <div class="item-title">作弊 / 练枪权限 (sv_cheats)</div>
            <div class="item-desc">开启后方可使用子弹落点、无限弹药与道具预览</div>
          </div>
          <label class="ios-switch">
            <input type="checkbox" :checked="cv.sv_cheats === '1'" @change="toggleBool('sv_cheats', $event.target.checked, '作弊权限')" />
            <span class="ios-slider"></span>
          </label>
        </div>

        <div class="settings-row">
          <div class="row-label">
            <span>子弹落点显示 (sv_showimpacts)</span>
            <span class="row-value">{{ impactLabel(cv.sv_showimpacts) }}</span>
          </div>
          <div class="segmented" style="width:100%;">
            <button style="flex:1;" :class="{ active: cv.sv_showimpacts === '0' }" @click="setCVar('sv_showimpacts', '0', '已关闭子弹落点')">关闭</button>
            <button style="flex:1;" :class="{ active: cv.sv_showimpacts === '1' }" @click="setCVar('sv_showimpacts', '1', '子弹落点：红蓝双显')">双显</button>
            <button style="flex:1;" :class="{ active: cv.sv_showimpacts === '2' }" @click="setCVar('sv_showimpacts', '2', '子弹落点：仅客户端')">客户端</button>
            <button style="flex:1;" :class="{ active: cv.sv_showimpacts === '3' }" @click="setCVar('sv_showimpacts', '3', '子弹落点：仅服务端')">服务端</button>
          </div>
        </div>

        <div class="settings-row">
          <div class="row-label">
            <span>无限弹药模式 (sv_infinite_ammo)</span>
            <span class="row-value">{{ ammoLabel(cv.sv_infinite_ammo) }}</span>
          </div>
          <div class="segmented" style="width:100%;">
            <button style="flex:1;" :class="{ active: cv.sv_infinite_ammo === '0' }" @click="setCVar('sv_infinite_ammo', '0', '已关闭无限弹药')">关闭</button>
            <button style="flex:1;" :class="{ active: cv.sv_infinite_ammo === '1' }" @click="setCVar('sv_infinite_ammo', '1', '已设为无限子弹免换弹')">免换弹</button>
            <button style="flex:1;" :class="{ active: cv.sv_infinite_ammo === '2' }" @click="setCVar('sv_infinite_ammo', '2', '已设为无限备弹需换弹')">无限备弹</button>
          </div>
        </div>

        <div class="settings-row-inline">
          <div>
            <div class="item-title">投掷物落点画中画预览</div>
            <div class="item-desc">瞄准道具时实时预览落点视角</div>
          </div>
          <label class="ios-switch">
            <input type="checkbox" :checked="cv.sv_grenade_trajectory_prac_pipreview === '1'" @change="toggleBool('sv_grenade_trajectory_prac_pipreview', $event.target.checked, '道具画中画预览')" />
            <span class="ios-slider"></span>
          </label>
        </div>

        <div class="settings-row-inline">
          <div>
            <div class="item-title">自动回血 (sv_regeneration_force_on)</div>
            <div class="item-desc">受伤后自动恢复满血，适合对枪/测穿点</div>
          </div>
          <label class="ios-switch">
            <input type="checkbox" :checked="cv.sv_regeneration_force_on === '1'" @change="toggleBool('sv_regeneration_force_on', $event.target.checked, '自动回血')" />
            <span class="ios-slider"></span>
          </label>
        </div>

        <div class="settings-row-inline">
          <div class="item-title">投掷物携带上限</div>
          <div class="segmented">
            <button :class="{ active: cv.ammo_grenade_limit_total === '4' }" @click="setCVar('ammo_grenade_limit_total', '4', '投掷物上限 4 颗')">4 颗</button>
            <button :class="{ active: cv.ammo_grenade_limit_total === '5' }" @click="setCVar('ammo_grenade_limit_total', '5', '投掷物上限 5 颗')">5 颗</button>
            <button :class="{ active: cv.ammo_grenade_limit_total === '6' }" @click="setCVar('ammo_grenade_limit_total', '6', '投掷物上限 6 颗')">6 颗</button>
          </div>
        </div>

        <div class="settings-row">
          <div style="display:flex;flex-wrap:wrap;gap:7px;">
            <button class="btn btn-sm btn-tinted" :disabled="busy" @click="runCvarAction('rethrow_grenade', '已重掷上一颗投掷物')">重掷上一颗道具</button>
            <button class="btn btn-sm" :disabled="busy" @click="runCvarAction('clear_smokes', '已清除场上烟雾与燃烧弹')">清除场上烟火</button>
          </div>
        </div>
      </div>

      <!-- Column 2: Match Rules, Economy & Bots -->
      <div class="settings-group">
        <div class="settings-row" style="background:var(--bg-surface-secondary);padding:9px 14px;">
          <span style="font-size:12px;font-weight:600;color:var(--label-secondary);">战局规则、经济与 Bot 机器人</span>
        </div>

        <div class="settings-row-inline">
          <div>
            <div class="item-title">全图随时购买 (mp_buy_anywhere)</div>
            <div class="item-desc">不限出生点即可按 B 键买枪与道具</div>
          </div>
          <label class="ios-switch">
            <input type="checkbox" :checked="cv.mp_buy_anywhere === '1'" @change="toggleBool('mp_buy_anywhere', $event.target.checked, '全图购买')" />
            <span class="ios-slider"></span>
          </label>
        </div>

        <div class="settings-row-inline">
          <div>
            <div class="item-title">阵亡即时复活 (死斗模式)</div>
            <div class="item-desc">双方玩家阵亡后立即复活</div>
          </div>
          <label class="ios-switch">
            <input type="checkbox" :checked="cv.mp_respawn_on_death_t === '1'" @change="toggleRespawn($event.target.checked)" />
            <span class="ios-slider"></span>
          </label>
        </div>

        <div class="settings-row-inline">
          <div class="item-title">友军伤害 (mp_friendlyfire)</div>
          <label class="ios-switch">
            <input type="checkbox" :checked="cv.mp_friendlyfire === '1'" @change="toggleBool('mp_friendlyfire', $event.target.checked, '友军伤害')" />
            <span class="ios-slider"></span>
          </label>
        </div>

        <div class="settings-row-inline">
          <div class="item-title">自动队伍平衡</div>
          <label class="ios-switch">
            <input type="checkbox" :checked="cv.mp_autoteambalance === '1'" @change="toggleTeamBalance($event.target.checked)" />
            <span class="ios-slider"></span>
          </label>
        </div>

        <div class="settings-row-inline">
          <div class="item-title">回合时长 / 冻结秒数</div>
          <div style="display:flex;gap:6px;flex-wrap:wrap;">
            <div class="segmented">
              <button :class="{ active: Number(cv.mp_roundtime_defuse) <= 2 }" @click="setBatch({ mp_roundtime: '1.92', mp_roundtime_defuse: '1.92' }, '回合时长 1.92 分')">1.92分</button>
              <button :class="{ active: Number(cv.mp_roundtime_defuse) > 2 && Number(cv.mp_roundtime_defuse) < 30 }" @click="setBatch({ mp_roundtime: '10', mp_roundtime_defuse: '10' }, '回合时长 10 分')">10分</button>
              <button :class="{ active: Number(cv.mp_roundtime_defuse) >= 30 }" @click="setBatch({ mp_roundtime: '60', mp_roundtime_defuse: '60' }, '回合时长 60 分')">60分</button>
            </div>
            <div class="segmented">
              <button :class="{ active: cv.mp_freezetime === '0' }" @click="setCVar('mp_freezetime', '0', '开局 0 秒冻结')">0s</button>
              <button :class="{ active: cv.mp_freezetime === '5' }" @click="setCVar('mp_freezetime', '5', '开局 5 秒冻结')">5s</button>
              <button :class="{ active: cv.mp_freezetime === '15' }" @click="setCVar('mp_freezetime', '15', '开局 15 秒冻结')">15s</button>
            </div>
          </div>
        </div>

        <div class="settings-row-inline">
          <div class="item-title">金钱配置 (开局/上限)</div>
          <div class="segmented">
            <button :class="{ active: cv.mp_startmoney === '800' }" @click="setBatch({ mp_startmoney: '800', mp_maxmoney: '16000' }, '标准经济 $800/$16000')">$800</button>
            <button :class="{ active: cv.mp_startmoney === '16000' }" @click="setBatch({ mp_startmoney: '16000', mp_maxmoney: '16000' }, '满经济 $16000')">$16000</button>
            <button :class="{ active: cv.mp_startmoney === '60000' }" @click="setBatch({ mp_startmoney: '60000', mp_maxmoney: '60000' }, '无限买枪 $60000')">$60000</button>
          </div>
        </div>

        <div class="settings-row-inline">
          <div>
            <div class="item-title">冻结 Bot 原地站桩 (bot_stop)</div>
            <div class="item-desc">让 Bot 静止不动，便于测穿点与练爆头</div>
          </div>
          <label class="ios-switch">
            <input type="checkbox" :checked="cv.bot_stop === '1'" @change="toggleBool('bot_stop', $event.target.checked, '冻结 Bot')" />
            <span class="ios-slider"></span>
          </label>
        </div>

        <div class="settings-row">
          <div class="row-label">
            <span>Bot 难度与增减</span>
            <div class="segmented">
              <button :class="{ active: cv.bot_difficulty === '0' }" @click="setCVar('bot_difficulty', '0', 'Bot 难度：简单')">简单</button>
              <button :class="{ active: cv.bot_difficulty === '1' }" @click="setCVar('bot_difficulty', '1', 'Bot 难度：普通')">普通</button>
              <button :class="{ active: cv.bot_difficulty === '2' }" @click="setCVar('bot_difficulty', '2', 'Bot 难度：困难')">困难</button>
              <button :class="{ active: cv.bot_difficulty === '3' }" @click="setCVar('bot_difficulty', '3', 'Bot 难度：专家')">专家</button>
            </div>
          </div>
          <div style="display:flex;flex-wrap:wrap;gap:7px;margin-top:4px;">
            <button class="btn btn-sm btn-destructive" :disabled="busy" @click="runPreset('kick_bots', '', '已清空所有 Bot')">清空 Bot</button>
            <button class="btn btn-sm" :disabled="busy" @click="runPreset('add_ct', '', '已添加 CT Bot')">+ CT Bot</button>
            <button class="btn btn-sm" :disabled="busy" @click="runPreset('add_t', '', '已添加 T Bot')">+ T Bot</button>
            <button class="btn btn-sm btn-tinted" :disabled="busy" @click="runCvarAction('bot_place', '已将 Bot 放置到玩家准星处')">放置 Bot 到准星处</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Custom RCON Console -->
    <div class="settings-group" style="margin-top:18px;">
      <div class="settings-row">
        <div class="row-label">
          <span>自由 RCON 控制台</span>
          <button class="btn btn-sm" :disabled="busy" @click="runPreset('status', '', '已查询服务器状态')">查询在线玩家 (status)</button>
        </div>
        <div style="display:flex;gap:8px;margin-top:4px;">
          <input
            v-model="rconCmd"
            class="input-apple tabular"
            placeholder="输入任意 CS2 控制台命令按回车执行，例如：sv_gravity 400 / host_workshop_map ..."
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

const cv = ref({
  sv_cheats: '0',
  sv_showimpacts: '0',
  sv_infinite_ammo: '0',
  sv_grenade_trajectory_prac_pipreview: '0',
  sv_regeneration_force_on: '0',
  ammo_grenade_limit_total: '4',
  mp_buy_anywhere: '0',
  mp_roundtime_defuse: '1.92',
  mp_startmoney: '800',
  mp_maxmoney: '16000',
  mp_freezetime: '15',
  mp_friendlyfire: '1',
  mp_autoteambalance: '1',
  mp_limitteams: '1',
  mp_respawn_on_death_t: '0',
  mp_respawn_on_death_ct: '0',
  bot_difficulty: '1',
  bot_stop: '0'
})

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

function impactLabel(v) {
  return ({ '0': '已关闭', '1': '红蓝双显', '2': '仅客户端', '3': '仅服务端' }[v] || v)
}
function ammoLabel(v) {
  return ({ '0': '已关闭', '1': '无限免换弹', '2': '无限备弹' }[v] || v)
}

async function apiReq(url, opts = {}) {
  const res = await fetch(url, { headers: { 'Content-Type': 'application/json' }, ...opts })
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || '请求失败')
  return data
}

async function refreshAll() {
  try {
    status.value = await apiReq('/api/server/status')
    emit('refresh-meta')
    if (status.value.server?.online) {
      try {
        const res = await apiReq('/api/server/cvars')
        if (res.cvars) cv.value = { ...cv.value, ...res.cvars }
      } catch {}
    }
  } catch (e) {
    emit('toast', e.message, true)
  }
}

async function doAction(action) {
  busy.value = true
  try {
    await apiReq('/api/server/action', { method: 'POST', body: JSON.stringify({ action }) })
    emit('toast', `已执行操作: ${action}`)
    setTimeout(refreshAll, 3000)
  } catch (e) { emit('toast', e.message, true) } finally { busy.value = false }
}

async function setBatch(changes, okMsg, restartRound = false) {
  busy.value = true
  try {
    const res = await apiReq('/api/server/cvar', {
      method: 'POST',
      body: JSON.stringify({ changes, restart_round: restartRound })
    })
    cv.value = { ...cv.value, ...changes }
    consoleLog.value = `> ${res.executed}\n${res.output || '(参数已实时应用)'}`
    emit('toast', okMsg)
  } catch (e) {
    consoleLog.value = `错误: ${e.message}`
    emit('toast', e.message, true)
  } finally { busy.value = false }
}

function setCVar(name, val, msg) {
  const changes = { [name]: val }
  // Automatically ensure sv_cheats 1 when enabling cheat-protected practice aids
  if (val !== '0' && ['sv_showimpacts', 'sv_infinite_ammo', 'sv_grenade_trajectory_prac_pipreview', 'sv_regeneration_force_on', 'bot_stop'].includes(name) && cv.value.sv_cheats !== '1') {
    changes.sv_cheats = '1'
  }
  return setBatch(changes, msg)
}

function toggleBool(name, checked, label) {
  return setCVar(name, checked ? '1' : '0', `${label}已${checked ? '开启' : '关闭'}`)
}

function toggleRespawn(checked) {
  const v = checked ? '1' : '0'
  return setBatch({ mp_respawn_on_death_t: v, mp_respawn_on_death_ct: v }, `阵亡即时复活已${checked ? '开启' : '关闭'}`)
}

function toggleTeamBalance(checked) {
  return setBatch({ mp_autoteambalance: checked ? '1' : '0', mp_limitteams: checked ? '1' : '0' }, `自动队伍平衡已${checked ? '开启' : '关闭'}`)
}

async function runCvarAction(action, okMsg) {
  busy.value = true
  try {
    const res = await apiReq('/api/server/cvar', { method: 'POST', body: JSON.stringify({ action }) })
    consoleLog.value = `> ${res.executed}\n${res.output || '(动作已执行)'}`
    emit('toast', okMsg)
  } catch (e) { emit('toast', e.message, true) } finally { busy.value = false }
}

function applyTemplate(tpl) {
  if (tpl === 'practice_full') {
    return setBatch({
      sv_cheats: '1', sv_showimpacts: '1', sv_infinite_ammo: '1',
      sv_grenade_trajectory_prac_pipreview: '1', ammo_grenade_limit_total: '5',
      mp_buy_anywhere: '1', mp_buytime: '9999', mp_roundtime: '60', mp_roundtime_defuse: '60',
      mp_startmoney: '60000', mp_maxmoney: '60000', mp_freezetime: '0',
      mp_autoteambalance: '0', mp_limitteams: '0', mp_respawn_on_death_t: '0', mp_respawn_on_death_ct: '0'
    }, '已套用「跑图/练道具全开」模板（下方可自由关闭任意单项）', true)
  }
  if (tpl === 'practice_clean') {
    return setBatch({
      sv_cheats: '1', sv_showimpacts: '0', sv_infinite_ammo: '2',
      sv_grenade_trajectory_prac_pipreview: '0', ammo_grenade_limit_total: '5',
      mp_buy_anywhere: '1', mp_buytime: '9999', mp_roundtime: '60', mp_roundtime_defuse: '60',
      mp_startmoney: '60000', mp_maxmoney: '60000', mp_freezetime: '0',
      mp_autoteambalance: '0', mp_limitteams: '0', bot_stop: '1'
    }, '已套用「纯净练枪」模板（子弹落点已关闭、无限备弹、Bot 站桩）', true)
  }
  if (tpl === 'deathmatch') {
    return setBatch({
      sv_cheats: '1', sv_showimpacts: '0', sv_infinite_ammo: '2', sv_regeneration_force_on: '1',
      mp_buy_anywhere: '1', mp_buytime: '9999', mp_roundtime: '60', mp_roundtime_defuse: '60',
      mp_startmoney: '60000', mp_maxmoney: '60000', mp_freezetime: '0',
      mp_respawn_on_death_t: '1', mp_respawn_on_death_ct: '1', mp_friendlyfire: '0', bot_stop: '0'
    }, '已套用「死斗热身」模板（即时复活 + 自动回血 + 关子弹落点）', true)
  }
  return setBatch({
    sv_cheats: '0', sv_showimpacts: '0', sv_infinite_ammo: '0',
    sv_grenade_trajectory_prac_pipreview: '0', sv_regeneration_force_on: '0',
    ammo_grenade_limit_total: '4', mp_buy_anywhere: '0', mp_buytime: '20',
    mp_roundtime: '1.92', mp_roundtime_defuse: '1.92', mp_startmoney: '800', mp_maxmoney: '16000',
    mp_freezetime: '15', mp_friendlyfire: '1', mp_autoteambalance: '1', mp_limitteams: '1',
    mp_respawn_on_death_t: '0', mp_respawn_on_death_ct: '0', bot_stop: '0'
  }, '已恢复标准 5v5 竞技规则', true)
}

async function runPreset(preset, arg, okMsg) {
  busy.value = true
  try {
    const res = await apiReq('/api/server/preset', { method: 'POST', body: JSON.stringify({ preset, arg }) })
    consoleLog.value = `> [${preset}${arg ? ' ' + arg : ''}]\n${res.output || '(命令已执行)'}`
    emit('toast', okMsg)
    setTimeout(refreshAll, 1500)
  } catch (e) {
    consoleLog.value = `错误: ${e.message}`
    emit('toast', e.message, true)
  } finally { busy.value = false }
}

async function sendRCON() {
  const cmd = rconCmd.value.trim()
  if (!cmd) return
  busy.value = true
  try {
    const res = await apiReq('/api/server/rcon', { method: 'POST', body: JSON.stringify({ command: cmd }) })
    consoleLog.value = `> ${cmd}\n${res.output || '(命令已执行)'}`
    rconCmd.value = ''
    emit('toast', 'RCON 指令执行成功')
  } catch (e) {
    consoleLog.value = `> ${cmd}\n错误: ${e.message}`
    emit('toast', e.message, true)
  } finally { busy.value = false }
}

onMounted(refreshAll)
</script>

<style scoped>
.console-two-col {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
  align-items: start;
}
@media (max-width: 980px) {
  .console-two-col { grid-template-columns: 1fr; }
}
.item-title { font-size: 13px; font-weight: 500; }
.item-desc { font-size: 11.5px; color: var(--label-secondary); margin-top: 2px; }
.console-box {
  background: var(--bg-surface-secondary);
  border: 0.5px solid var(--separator);
  border-radius: 10px;
  padding: 12px 14px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.55;
  color: var(--label-primary);
  min-height: 110px;
  max-height: 280px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
