<template>
  <div>
    <!-- Loading -->
    <div v-if="booting" class="gate-screen">
      <div style="color:var(--label-secondary);font-size:14px;">正在连接服务...</div>
    </div>

    <!-- MODE 1: ISOLATED ADMIN CONSOLE (Port 9090) -->
    <AdminApp
      v-else-if="meta.mode === 'admin'"
      :meta="meta"
      :theme="theme"
      @toast="showToast"
      @refresh-meta="refreshMeta"
      @set-theme="setTheme"
    />

    <!-- MODE 2A: PLAYER PAIRING GATE (Port 8080, Unverified) -->
    <div v-else-if="!meta.authenticated" class="gate-screen">
      <div class="gate-card">
        <div class="gate-icon">
          <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
            <path d="m9 12 2 2 4-4" />
          </svg>
        </div>
        <h1 style="font-size:22px;font-weight:700;letter-spacing:-0.3px;">CS2 皮肤工坊 · 安全认领</h1>
        <p style="font-size:13px;color:var(--label-secondary);margin-top:8px;line-height:1.6;">
          无需登录 Steam 网页账号，同时防止他人仅凭公开 SteamID64 篡改你的皮肤。请使用以下任一方式完成一次绑定（浏览器自动记住 30 天）：
        </p>

        <div class="settings-group" style="text-align:left;margin-top:18px;">
          <div class="settings-row">
            <div style="font-size:12.5px;font-weight:600;">方式 1：游戏内获取 6 位配对码（推荐）</div>
            <div style="font-size:12px;color:var(--label-secondary);">
              进入 CS2 服务器后，在聊天栏输入 <code style="color:var(--accent);font-weight:600;">!bind</code>，系统会仅向你私聊发送 6 位配对码。
            </div>
          </div>
          <div class="settings-row">
            <div style="font-size:12.5px;font-weight:600;">方式 2：房主私密专属链接</div>
            <div style="font-size:12px;color:var(--label-secondary);">
              直接打开房主在管理端为你生成的一次性专属免登链接，点开即自动绑定。
            </div>
          </div>
        </div>

        <div style="margin-top:18px;display:flex;flex-direction:column;gap:10px;">
          <input
            v-model="pairCode"
            class="code-input"
            maxlength="6"
            placeholder="000000"
            @keyup.enter="redeemCode"
          />
          <button class="btn btn-accent" style="justify-content:center;padding:11px;font-size:14px;" :disabled="redeeming" @click="redeemCode">
            {{ redeeming ? '验证中...' : '验证配对码并进入工坊' }}
          </button>
        </div>

        <div style="margin-top:18px;display:flex;align-items:center;justify-content:space-between;font-size:12px;color:var(--label-secondary);">
          <span style="display:flex;align-items:center;gap:6px;">
            <span class="status-dot" :class="{ online: meta.server?.online }"></span>
            <span v-if="meta.server?.online">{{ meta.server.map }} · {{ meta.server.players }}/{{ meta.server.max_players }}</span>
            <span v-else>游戏服待机中</span>
          </span>
          <div class="segmented">
            <button :class="{ active: theme === 'light' }" @click="setTheme('light')">浅色</button>
            <button :class="{ active: theme === 'dark' }" @click="setTheme('dark')">深色</button>
          </div>
        </div>
      </div>
    </div>

    <!-- MODE 2B: AUTHENTICATED PLAYER SKIN WORKSHOP (Port 8080) -->
    <div v-else class="app-shell">
      <aside class="sidebar">
        <div class="sidebar-brand">
          <div class="app-icon">CS2</div>
          <div>
            <div class="brand-name">皮肤工坊</div>
            <div class="brand-meta">
              <span class="status-dot" :class="{ online: meta.server?.online }"></span>
              <span v-if="meta.server?.online" class="tabular">
                {{ meta.server.map }} · {{ meta.server.players }}/{{ meta.server.max_players }}
              </span>
              <span v-else>游戏服待机</span>
            </div>
          </div>
        </div>

        <div>
          <div class="nav-section-label">装备库</div>
          <nav class="nav-list">
            <button class="nav-item" :class="{ active: tab === 'weapons' }" @click="tab = 'weapons'">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75"><path d="M3 10h13l3-2h2v4h-3l-2 2h-4l-1 4H8l1-4H3z" /></svg>
              <span>武器皮肤</span>
              <span class="nav-count">{{ catalog.weapons.length }}</span>
            </button>
            <button class="nav-item" :class="{ active: tab === 'knives' }" @click="tab = 'knives'">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75"><path d="M19 5 5 19" /><path d="M14 5h5v5" /><path d="m7 17-3 3" /></svg>
              <span>匕首类型</span>
              <span class="nav-count">{{ catalog.knives.length }}</span>
            </button>
            <button class="nav-item" :class="{ active: tab === 'gloves' }" @click="tab = 'gloves'">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75"><path d="M18 11V6a2 2 0 0 0-4 0v4" /><path d="M14 10V4a2 2 0 0 0-4 0v6" /><path d="M10 10.5V6a2 2 0 0 0-4 0v8a6 6 0 0 0 12 0v-3a2 2 0 0 0-4 0" /></svg>
              <span>手套</span>
              <span class="nav-count">{{ catalog.gloves.length }}</span>
            </button>
            <button class="nav-item" :class="{ active: tab === 'agents' }" @click="tab = 'agents'">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75"><circle cx="12" cy="8" r="4" /><path d="M5 20a7 7 0 0 1 14 0" /></svg>
              <span>探员</span>
              <span class="nav-count">{{ catalog.agents.length }}</span>
            </button>
            <button class="nav-item" :class="{ active: tab === 'music' }" @click="tab = 'music'">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75"><path d="M9 18V5l12-2v13" /><circle cx="6" cy="18" r="3" /><circle cx="18" cy="16" r="3" /></svg>
              <span>音乐盒</span>
              <span class="nav-count">{{ catalog.music.length }}</span>
            </button>
            <button class="nav-item" :class="{ active: tab === 'pins' }" @click="tab = 'pins'">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75"><circle cx="12" cy="9" r="5" /><path d="m9 13.5-1 7 4-2 4 2-1-7" /></svg>
              <span>收藏徽章</span>
              <span class="nav-count">{{ catalog.pins.length }}</span>
            </button>
          </nav>
        </div>

        <div>
          <div class="nav-section-label">已认证玩家身份</div>
          <div class="sidebar-card">
            <div style="font-size:11.5px;color:var(--green);font-weight:600;">已通过安全配对认证</div>
            <div class="tabular" style="font-size:13px;font-weight:600;word-break:break-all;">{{ meta.steamid }}</div>
            <button class="btn btn-sm" style="justify-content:center;" @click="playerLogout">解除当前浏览器绑定</button>
          </div>
        </div>

        <div class="sidebar-footer">
          <div class="segmented" style="width:100%;">
            <button style="flex:1;" :class="{ active: theme === 'light' }" @click="setTheme('light')">浅色</button>
            <button style="flex:1;" :class="{ active: theme === 'dark' }" @click="setTheme('dark')">深色</button>
          </div>
          <button class="btn btn-destructive btn-sm" style="justify-content:center;" @click="resetAll">重置我的全部皮肤</button>
        </div>
      </aside>

      <div class="main-area">
        <header class="toolbar">
          <div class="toolbar-left">
            <div class="segmented">
              <button :class="{ active: teamMode === 0 }" @click="teamMode = 0">双阵营同步</button>
              <button :class="{ active: teamMode === 2 }" @click="teamMode = 2">T 阵营</button>
              <button :class="{ active: teamMode === 3 }" @click="teamMode = 3">CT 阵营</button>
            </div>
          </div>
          <div class="toolbar-right">
            <div class="search-field">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg>
              <input v-model="searchQuery" :placeholder="searchPlaceholder" />
              <button v-if="searchQuery" class="search-clear" @click="searchQuery = ''">×</button>
            </div>
            <button class="btn btn-tinted" @click="copyText('!wp', '已复制 !wp，进服聊天栏发送即可实时刷新')">
              复制 !wp 刷新指令
            </button>
          </div>
        </header>

        <main class="content">
          <section v-if="tab === 'weapons'">
            <div class="page-header">
              <div>
                <h1 class="page-title">武器与匕首皮肤</h1>
                <p class="page-subtitle">仅对你已认证的账号生效，保存后在游戏聊天栏输入 !wp 即刻应用</p>
              </div>
              <div class="segmented">
                <button v-for="c in weaponCategories" :key="c.key" :class="{ active: weaponCat === c.key }" @click="weaponCat = c.key">
                  {{ c.label }}
                </button>
              </div>
            </div>
            <div class="grid">
              <div v-for="w in filteredWeapons" :key="w.defindex" class="card" :class="{ equipped: !!equippedSkin(w.defindex) }" @click="modalWeapon = w">
                <span v-if="equippedSkin(w.defindex)" class="card-check"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><path d="M20 6 9 17l-5-5" /></svg></span>
                <div class="card-art"><img :src="weaponCardImage(w)" :alt="w.display_name" loading="lazy" /></div>
                <div>
                  <div class="card-title">{{ w.display_name }}</div>
                  <div class="card-subtitle">{{ weaponCardSkinName(w) }}</div>
                  <div class="card-pills" v-if="equippedSkin(w.defindex)">
                    <span class="pill-tag blue tabular">{{ formatWear(equippedSkin(w.defindex).weapon_wear) }}</span>
                    <span class="pill-tag tabular">#{{ equippedSkin(w.defindex).weapon_seed }}</span>
                    <span v-if="equippedSkin(w.defindex).weapon_stattrak" class="pill-tag orange">ST™</span>
                  </div>
                </div>
              </div>
            </div>
          </section>

          <section v-else-if="tab === 'knives'">
            <div class="page-header">
              <div>
                <h1 class="page-title">匕首类型</h1>
                <p class="page-subtitle">当前装备：{{ currentKnifeLabel }}</p>
              </div>
            </div>
            <div class="grid">
              <div class="card" :class="{ equipped: !currentKnifeName }" @click="saveKnife('weapon_knife')">
                <span v-if="!currentKnifeName" class="card-check"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><path d="M20 6 9 17l-5-5" /></svg></span>
                <div class="card-art" style="color:var(--label-tertiary);font-size:13px;">默认原版</div>
                <div><div class="card-title">默认原版匕首</div><div class="card-subtitle">恢复阵营初始刀型</div></div>
              </div>
              <div v-for="k in filteredKnives" :key="k.weapon_name" class="card" :class="{ equipped: currentKnifeName === k.weapon_name }" @click="saveKnife(k.weapon_name)">
                <span v-if="currentKnifeName === k.weapon_name" class="card-check"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><path d="M20 6 9 17l-5-5" /></svg></span>
                <div class="card-art"><img :src="k.image" :alt="k.display_name" loading="lazy" /></div>
                <div>
                  <div class="card-title">{{ k.display_name }}</div>
                  <div style="display:flex;align-items:center;justify-content:space-between;margin-top:8px;">
                    <span class="card-subtitle">{{ k.weapon_name.replace('weapon_', '') }}</span>
                    <button class="btn btn-sm btn-tinted" @click.stop="jumpToKnifeSkin(k.defindex)">定制涂装</button>
                  </div>
                </div>
              </div>
            </div>
          </section>

          <section v-else-if="tab === 'gloves'">
            <div class="page-header"><div><h1 class="page-title">手套</h1><p class="page-subtitle">调节磨损后点击即可同步</p></div></div>
            <div class="inline-bar">
              <span style="font-size:13px;font-weight:500;">手套磨损</span>
              <input type="range" min="0.0001" max="0.80" step="0.001" v-model.number="gloveWear" style="width:160px;" />
              <span class="row-value tabular" style="width:54px;">{{ gloveWear.toFixed(4) }}</span>
              <button class="btn btn-sm" @click="gloveWear = 0.0001">极品崭新</button>
              <button class="btn btn-sm" @click="gloveWear = 0.075">略有磨损</button>
              <button class="btn btn-sm" @click="gloveWear = 0.16">久经沙场</button>
            </div>
            <div class="grid">
              <div v-for="g in filteredGloves" :key="g.weapon_defindex + '-' + g.paint" class="card" :class="{ equipped: isGloveEquipped(g) }" @click="saveGlove(g)">
                <span v-if="isGloveEquipped(g)" class="card-check"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><path d="M20 6 9 17l-5-5" /></svg></span>
                <div class="card-art"><img v-if="g.image" :src="g.image" :alt="g.name" loading="lazy" /><span v-else style="font-size:13px;color:var(--label-tertiary);">默认原版</span></div>
                <div class="card-title">{{ g.name }}</div>
              </div>
            </div>
          </section>

          <section v-else-if="tab === 'agents'">
            <div class="page-header">
              <div><h1 class="page-title">探员</h1><p class="page-subtitle">分别定制 T 与 CT 阵营模型</p></div>
              <div class="segmented">
                <button :class="{ active: agentTeam === 2 }" @click="agentTeam = 2">T 阵营</button>
                <button :class="{ active: agentTeam === 3 }" @click="agentTeam = 3">CT 阵营</button>
              </div>
            </div>
            <div class="grid">
              <div v-for="a in filteredAgents" :key="a.team + '-' + a.model" class="card" :class="{ equipped: currentAgentModel === a.model }" @click="saveAgent(a)">
                <span v-if="currentAgentModel === a.model" class="card-check"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><path d="M20 6 9 17l-5-5" /></svg></span>
                <div class="card-art"><img v-if="a.image" :src="a.image" :alt="a.name" loading="lazy" /><span v-else style="font-size:13px;color:var(--label-tertiary);">默认探员</span></div>
                <div class="card-title">{{ a.name }}</div>
              </div>
            </div>
          </section>

          <section v-else-if="tab === 'music'">
            <div class="page-header"><div><h1 class="page-title">音乐盒</h1><p class="page-subtitle">点击即装备</p></div></div>
            <div class="grid">
              <div v-for="m in filteredMusic" :key="m.id" class="card" :class="{ equipped: currentMusicId === m.id }" @click="saveMusic(m.id)">
                <span v-if="currentMusicId === m.id" class="card-check"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><path d="M20 6 9 17l-5-5" /></svg></span>
                <div class="card-art"><img v-if="m.image" :src="m.image" :alt="m.name" loading="lazy" /><span v-else style="font-size:13px;color:var(--label-tertiary);">默认原声</span></div>
                <div class="card-title">{{ m.name }}</div>
              </div>
            </div>
          </section>

          <section v-else-if="tab === 'pins'">
            <div class="page-header"><div><h1 class="page-title">收藏徽章</h1><p class="page-subtitle">默认展示前 120 项，顶部搜索可检索全部</p></div></div>
            <div class="grid">
              <div v-for="p in filteredPins" :key="p.id" class="card" :class="{ equipped: currentPinId === p.id }" @click="savePin(p.id)">
                <span v-if="currentPinId === p.id" class="card-check"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><path d="M20 6 9 17l-5-5" /></svg></span>
                <div class="card-art"><img v-if="p.image" :src="p.image" :alt="p.name" loading="lazy" /><span v-else style="font-size:13px;color:var(--label-tertiary);">无徽章</span></div>
                <div class="card-title">{{ p.name }}</div>
              </div>
            </div>
          </section>
        </main>
      </div>

      <SkinSheet
        v-if="modalWeapon"
        :weapon="modalWeapon"
        :initial="equippedSkin(modalWeapon.defindex)"
        :team-mode-label="teamModeLabel"
        :saving="saving"
        @close="modalWeapon = null"
        @save="saveWeaponSkin"
      />
    </div>

    <div v-if="toast.show" class="hud-toast" :class="{ error: toast.error }">
      <span>{{ toast.msg }}</span>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import SkinSheet from './components/SkinSheet.vue'
import AdminApp from './components/AdminApp.vue'

const booting = ref(true)
const saving = ref(false)
const redeeming = ref(false)
const pairCode = ref('')
const tab = ref('weapons')
const teamMode = ref(0)
const theme = ref('light')
const searchQuery = ref('')

const meta = ref({ mode: 'player', authenticated: false, steamid: '', server: {} })
const catalog = ref({ weapons: [], knives: [], gloves: [], agents: [], music: [], pins: [] })
const loadout = ref({ skins_t: {}, skins_ct: {}, knife_t: '', knife_ct: '', glove_t: 0, glove_ct: 0, agent_t: '', agent_ct: '', music_t: 0, music_ct: 0, pin_t: 0, pin_ct: 0 })

const weaponCat = ref('all')
const gloveWear = ref(0.0001)
const agentTeam = ref(2)
const modalWeapon = ref(null)

const toast = ref({ show: false, msg: '', error: false })
let toastTimer = null

const weaponCategories = [
  { key: 'all', label: '全部' }, { key: 'rifles', label: '步枪' },
  { key: 'snipers', label: '狙击' }, { key: 'pistols', label: '手枪' },
  { key: 'smgs', label: '冲锋枪' }, { key: 'heavy', label: '重武器' },
  { key: 'knives', label: '匕首' }
]

const searchPlaceholder = computed(() => ({
  weapons: '搜索武器或涂装', knives: '搜索匕首', gloves: '搜索手套',
  agents: '搜索探员', music: '搜索音乐盒', pins: '搜索徽章'
}[tab.value] || '搜索'))

const teamModeLabel = computed(() =>
  teamMode.value === 2 ? '仅 T 阵营' : teamMode.value === 3 ? '仅 CT 阵营' : 'T + CT 双阵营'
)

function setTheme(t) {
  theme.value = t
  document.documentElement.setAttribute('data-theme', t)
  localStorage.setItem('cs2_theme', t)
}

function showToast(msg, error = false) {
  toast.value = { show: true, msg, error }
  clearTimeout(toastTimer)
  toastTimer = setTimeout(() => { toast.value.show = false }, 2800)
}

function activeSkinsMap() {
  return teamMode.value === 3 ? (loadout.value.skins_ct || {}) : (loadout.value.skins_t || {})
}
function equippedSkin(defindex) { return activeSkinsMap()[defindex] || null }
function weaponCardImage(w) {
  const eq = equippedSkin(w.defindex)
  if (eq) {
    const f = w.skins.find(s => s.paint_id === eq.weapon_paint_id)
    if (f?.image) return f.image
  }
  return w.default_image
}
function weaponCardSkinName(w) {
  const eq = equippedSkin(w.defindex)
  if (!eq) return '默认原版'
  const f = w.skins.find(s => s.paint_id === eq.weapon_paint_id)
  return f ? f.short_name : `涂装 #${eq.weapon_paint_id}`
}
function formatWear(w) { return Number(w ?? 0).toFixed(3) }

const filteredWeapons = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return catalog.value.weapons.filter(w =>
    (weaponCat.value === 'all' || w.category === weaponCat.value) &&
    (!q || w.display_name.toLowerCase().includes(q) || w.weapon_name.toLowerCase().includes(q) || weaponCardSkinName(w).toLowerCase().includes(q))
  )
})
const currentKnifeName = computed(() => teamMode.value === 3 ? loadout.value.knife_ct : loadout.value.knife_t)
const currentKnifeLabel = computed(() => {
  if (!currentKnifeName.value) return '默认原版匕首'
  const k = catalog.value.knives.find(x => x.weapon_name === currentKnifeName.value)
  return k ? k.display_name : currentKnifeName.value
})
const filteredKnives = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return catalog.value.knives.filter(k => !q || k.display_name.toLowerCase().includes(q) || k.weapon_name.toLowerCase().includes(q))
})
const filteredGloves = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return catalog.value.gloves.filter(g => !q || g.name.toLowerCase().includes(q))
})
function isGloveEquipped(g) {
  const curDef = teamMode.value === 3 ? loadout.value.glove_ct : loadout.value.glove_t
  if (g.weapon_defindex === 0) return !curDef
  if (curDef !== g.weapon_defindex) return false
  const skin = equippedSkin(g.weapon_defindex)
  return skin && skin.weapon_paint_id === g.paint
}
const currentAgentModel = computed(() => agentTeam.value === 3 ? (loadout.value.agent_ct || '') : (loadout.value.agent_t || ''))
const filteredAgents = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return catalog.value.agents.filter(a => a.team === agentTeam.value && (!q || a.name.toLowerCase().includes(q)))
})
const currentMusicId = computed(() => teamMode.value === 3 ? (loadout.value.music_ct || 0) : (loadout.value.music_t || 0))
const filteredMusic = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return catalog.value.music.filter(m => !q || m.name.toLowerCase().includes(q))
})
const currentPinId = computed(() => teamMode.value === 3 ? (loadout.value.pin_ct || 0) : (loadout.value.pin_t || 0))
const filteredPins = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  const list = catalog.value.pins.filter(p => !q || p.name.toLowerCase().includes(q))
  return q ? list.slice(0, 200) : list.slice(0, 120)
})

function jumpToKnifeSkin(defindex) {
  const w = catalog.value.weapons.find(x => x.defindex === defindex)
  if (w) { tab.value = 'weapons'; weaponCat.value = 'knives'; modalWeapon.value = w }
}

async function apiReq(url, opts = {}) {
  const res = await fetch(url, { headers: { 'Content-Type': 'application/json' }, ...opts })
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || '请求失败')
  return data
}

async function refreshMeta() {
  meta.value = await apiReq('/api/meta')
  if (meta.value.mode === 'player' && meta.value.authenticated) {
    if (!catalog.value.weapons.length) catalog.value = await apiReq('/api/catalog')
    loadout.value = await apiReq('/api/loadout')
  }
}

async function redeemCode() {
  const c = pairCode.value.trim()
  if (!/^\d{6}$/.test(c)) { showToast('请输入 6 位数字配对码', true); return }
  redeeming.value = true
  try {
    await apiReq('/api/auth/redeem', { method: 'POST', body: JSON.stringify({ code: c }) })
    pairCode.value = ''
    await refreshMeta()
    showToast('配对成功！已为本浏览器安全绑定 30 天')
  } catch (e) { showToast(e.message, true) } finally { redeeming.value = false }
}

async function playerLogout() {
  await apiReq('/api/auth/logout', { method: 'POST' })
  await refreshMeta()
  showToast('已解除当前浏览器绑定')
}

async function saveWeaponSkin(payload) {
  saving.value = true
  try {
    const res = await apiReq('/api/loadout/skin', { method: 'PUT', body: JSON.stringify({ team: teamMode.value, ...payload }) })
    loadout.value = res.loadout; modalWeapon.value = null
    showToast('已保存涂装，游戏内输入 !wp 立即生效')
  } catch (e) { showToast(e.message, true) } finally { saving.value = false }
}
async function saveKnife(knife) {
  try {
    const res = await apiReq('/api/loadout/knife', { method: 'PUT', body: JSON.stringify({ team: teamMode.value, knife }) })
    loadout.value = res.loadout; showToast('已切换匕首，游戏内输入 !wp 生效')
  } catch (e) { showToast(e.message, true) }
}
async function saveGlove(g) {
  try {
    const res = await apiReq('/api/loadout/glove', { method: 'PUT', body: JSON.stringify({ team: teamMode.value, weapon_defindex: g.weapon_defindex, weapon_paint_id: g.paint, weapon_wear: gloveWear.value, weapon_seed: 0 }) })
    loadout.value = res.loadout; showToast('已更新手套，游戏内输入 !wp 生效')
  } catch (e) { showToast(e.message, true) }
}
async function saveAgent(a) {
  try {
    const res = await apiReq('/api/loadout/agent', { method: 'PUT', body: JSON.stringify({ team: a.team, model: a.model }) })
    loadout.value = res.loadout; showToast('已更新探员模型')
  } catch (e) { showToast(e.message, true) }
}
async function saveMusic(id) {
  try {
    const res = await apiReq('/api/loadout/music', { method: 'PUT', body: JSON.stringify({ team: teamMode.value, music_id: id }) })
    loadout.value = res.loadout; showToast('已更新音乐盒，游戏内输入 !wp 生效')
  } catch (e) { showToast(e.message, true) }
}
async function savePin(id) {
  try {
    const res = await apiReq('/api/loadout/pin', { method: 'PUT', body: JSON.stringify({ team: teamMode.value, id }) })
    loadout.value = res.loadout; showToast('已更新收藏徽章，游戏内输入 !wp 生效')
  } catch (e) { showToast(e.message, true) }
}
async function resetAll() {
  if (!confirm('确定清空你的全部皮肤配置吗？')) return
  try {
    const res = await apiReq('/api/loadout', { method: 'DELETE' })
    loadout.value = res.loadout; showToast('已重置全部皮肤配置')
  } catch (e) { showToast(e.message, true) }
}
async function copyText(text, msg) {
  try { await navigator.clipboard.writeText(text); showToast(msg) } catch { showToast(text) }
}

onMounted(async () => {
  const savedTheme = localStorage.getItem('cs2_theme')
  const prefersDark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches
  setTheme(savedTheme || (prefersDark ? 'dark' : 'light'))

  try {
    const params = new URLSearchParams(window.location.search)
    const tok = params.get('token')
    if (tok) {
      try {
        await apiReq('/api/auth/redeem', { method: 'POST', body: JSON.stringify({ token: tok }) })
        window.history.replaceState({}, '', '/')
        showToast('专属链接验证成功！已绑定本浏览器 30 天')
      } catch (e) {
        window.history.replaceState({}, '', '/')
        showToast(e.message, true)
      }
    }
    await refreshMeta()
  } catch (e) {
    showToast('连接失败: ' + e.message, true)
  } finally {
    booting.value = false
  }
})
</script>
