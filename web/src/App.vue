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

        <div v-if="meta.db_connected === false" class="settings-group" style="text-align:left;margin-top:14px;border-color:rgba(255,149,0,0.45);background:rgba(255,149,0,0.08);">
          <div class="settings-row">
            <div style="font-size:13px;font-weight:700;color:var(--orange);">首次使用提示 · 服务器尚未初始化</div>
            <div style="font-size:12px;color:var(--label-secondary);line-height:1.55;margin-top:4px;">
              检测到本地皮肤数据库与 CS2 服务端还未部署。你是第一次使用吗？无需任何命令行，请打开管理端使用「新手引导」一键全自动下载安装（支持自动配置 MariaDB、SteamCMD、CS2 与皮肤插件）。
            </div>
            <div style="margin-top:8px;display:flex;align-items:center;justify-content:space-between;gap:8px;flex-wrap:wrap;">
              <a class="btn btn-sm btn-accent" :href="`http://${windowHost}:${meta.admin_port || '9090'}`" target="_blank" style="text-decoration:none;">
                打开管理端「新手引导」 ↗
              </a>
              <span style="font-size:11px;color:var(--label-tertiary);">管理员密码见启动黑框或 panel-config.json</span>
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
              <span class="nav-count">{{ gunWeapons.length }}</span>
            </button>
            <button class="nav-item" :class="{ active: tab === 'knives' }" @click="tab = 'knives'">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75"><path d="M19 5 5 19" /><path d="M14 5h5v5" /><path d="m7 17-3 3" /></svg>
              <span>匕首</span>
              <span class="nav-count">{{ knifeWeapons.length }}</span>
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
            <button class="btn btn-tinted" :disabled="syncing" @click="manualRefresh">
              {{ syncing ? '同步中...' : '同步皮肤到游戏' }}
            </button>
          </div>
        </header>

        <main class="content">
          <!-- GUN SKINS TAB -->
          <section v-if="tab === 'weapons'">
            <div class="page-header">
              <div>
                <h1 class="page-title">武器皮肤</h1>
                <p class="page-subtitle">仅对你已认证的账号生效，点击保存即自动刷新游戏内皮肤（无需手动输入 !wp）</p>
              </div>
              <div class="segmented">
                <button v-for="c in weaponCategories" :key="c.key" :class="{ active: weaponCat === c.key }" @click="weaponCat = c.key">
                  {{ c.label }}
                </button>
              </div>
            </div>
            <div v-if="filteredWeapons.length" class="grid">
              <div v-for="w in filteredWeapons" :key="w.defindex" class="card" :class="{ equipped: !!equippedSkin(w.defindex) }" @click="modalWeapon = w">
                <span v-if="equippedSkin(w.defindex)" class="card-check"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><path d="M20 6 9 17l-5-5" /></svg></span>
                <div class="card-art"><img :src="weaponCardImage(w)" :alt="w.display_name" loading="lazy" /></div>
                <div>
                  <div class="card-title">{{ w.display_name }}</div>
                  <div class="card-subtitle">{{ weaponCardSkinName(w) }}</div>
                  <div class="card-pills" v-if="equippedSkin(w.defindex) || isTeamDiffSkin(w.defindex)">
                    <span v-if="equippedSkin(w.defindex)" class="pill-tag blue tabular">{{ formatWear(equippedSkin(w.defindex).weapon_wear) }}</span>
                    <span v-if="equippedSkin(w.defindex)" class="pill-tag tabular">#{{ equippedSkin(w.defindex).weapon_seed }}</span>
                    <span v-if="equippedSkin(w.defindex)?.weapon_stattrak" class="pill-tag orange">ST™</span>
                    <span v-if="isTeamDiffSkin(w.defindex)" class="pill-tag">T/CT 不同</span>
                  </div>
                </div>
              </div>
            </div>
            <div v-else class="empty-state">
              <div>未找到匹配「{{ searchQuery }}」的武器或涂装</div>
              <button class="btn btn-sm btn-tinted" @click="searchQuery = ''; weaponCat = 'all'">重置筛选与搜索</button>
            </div>
          </section>

          <!-- UNIFIED KNIFE TAB -->
          <section v-else-if="tab === 'knives'">
            <div class="page-header">
              <div>
                <h1 class="page-title">匕首与涂装</h1>
                <p class="page-subtitle">每次只能手持 1 把主战匕首（{{ teamModeLabel }}），点击任意匕首卡片即可原地定制涂装并自动装备</p>
              </div>
            </div>

            <div class="active-knife-bar">
              <div class="active-knife-main">
                <div class="active-knife-thumb">
                  <img v-if="currentKnifeWeapon" :src="weaponCardImage(currentKnifeWeapon)" :alt="currentKnifeWeapon.display_name" />
                  <span v-else style="font-size:12px;color:var(--label-tertiary);">原版刀</span>
                </div>
                <div style="min-width:0;">
                  <div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap;">
                    <span class="pill-tag green">当前使用中</span>
                    <span v-if="hasTeamKnifeSplit" class="pill-tag orange">T / CT 阵营刀型不同</span>
                  </div>
                  <div class="active-knife-title">{{ activeKnifeHeadline }}</div>
                  <div class="active-knife-sub">
                    <template v-if="currentKnifeWeapon">
                      涂装：{{ weaponCardSkinName(currentKnifeWeapon) }}
                      <template v-if="equippedSkin(currentKnifeWeapon.defindex)">
                        · 磨损 {{ formatWear(equippedSkin(currentKnifeWeapon.defindex).weapon_wear) }}
                        · #{{ equippedSkin(currentKnifeWeapon.defindex).weapon_seed }}
                      </template>
                    </template>
                    <template v-else>
                      未装备定制匕首，当前使用 CS2 阵营默认原版匕首
                    </template>
                  </div>
                </div>
              </div>
              <div class="active-knife-actions">
                <button v-if="currentKnifeWeapon" class="btn btn-accent" @click="modalWeapon = currentKnifeWeapon">
                  定制当前涂装
                </button>
                <button v-if="hasAnyCustomKnife" class="btn" @click="saveKnife('weapon_knife')">
                  恢复默认原版刀
                </button>
              </div>
            </div>

            <div v-if="filteredKnifeWeapons.length || !searchQuery.trim()" class="grid">
              <div v-if="!searchQuery.trim()" class="card" :class="{ equipped: isDefaultKnifeActive }" @click="saveKnife('weapon_knife')">
                <span v-if="isDefaultKnifeActive" class="card-check"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><path d="M20 6 9 17l-5-5" /></svg></span>
                <span v-else-if="defaultKnifeTeamBadge" class="card-status-badge is-active">{{ defaultKnifeTeamBadge }}</span>
                <div class="card-art" style="color:var(--label-tertiary);font-size:13px;">默认原版</div>
                <div>
                  <div class="card-title">默认原版匕首</div>
                  <div class="card-subtitle">恢复阵营初始刀型</div>
                  <div class="knife-card-footer">
                    <span v-if="isDefaultKnifeActive" class="pill-tag green">使用中</span>
                    <button v-else class="btn btn-sm" @click.stop="saveKnife('weapon_knife')">切回原版</button>
                  </div>
                </div>
              </div>

              <div
                v-for="w in filteredKnifeWeapons"
                :key="w.defindex"
                class="card"
                :class="{
                  equipped: isKnifeActive(w.weapon_name),
                  'saved-only': !isKnifeActive(w.weapon_name) && !!equippedSkin(w.defindex)
                }"
                @click="modalWeapon = w"
              >
                <span v-if="isKnifeActive(w.weapon_name)" class="card-check"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><path d="M20 6 9 17l-5-5" /></svg></span>
                <span v-else-if="equippedSkin(w.defindex)" class="card-status-badge is-saved">已存涂装</span>

                <div class="card-art"><img :src="weaponCardImage(w)" :alt="w.display_name" loading="lazy" /></div>
                <div>
                  <div class="card-title">{{ w.display_name }}</div>
                  <div class="card-subtitle">{{ weaponCardSkinName(w) }}</div>
                  <div class="card-pills" v-if="equippedSkin(w.defindex)">
                    <span class="pill-tag blue tabular">{{ formatWear(equippedSkin(w.defindex).weapon_wear) }}</span>
                    <span class="pill-tag tabular">#{{ equippedSkin(w.defindex).weapon_seed }}</span>
                    <span v-if="equippedSkin(w.defindex).weapon_stattrak" class="pill-tag orange">ST™</span>
                  </div>
                  <div class="knife-card-footer">
                    <template v-if="isKnifeActive(w.weapon_name)">
                      <span class="pill-tag green">{{ knifeTeamBadge(w.weapon_name) }}</span>
                      <button class="btn btn-sm btn-tinted" @click.stop="modalWeapon = w">换涂装</button>
                    </template>
                    <template v-else>
                      <button class="btn btn-sm btn-accent" @click.stop="saveKnife(w.weapon_name)">装备此刀</button>
                      <button class="btn btn-sm" @click.stop="modalWeapon = w">选涂装</button>
                    </template>
                  </div>
                </div>
              </div>
            </div>
            <div v-else class="empty-state">
              <div>未找到匹配「{{ searchQuery }}」的匕首或涂装</div>
              <button class="btn btn-sm btn-tinted" @click="searchQuery = ''">清空搜索</button>
            </div>
          </section>

          <!-- GLOVES TAB -->
          <section v-else-if="tab === 'gloves'">
            <div class="page-header"><div><h1 class="page-title">手套</h1><p class="page-subtitle">调节磨损后点击即可自动同步到游戏</p></div></div>
            <div class="inline-bar">
              <span style="font-size:13px;font-weight:500;">手套磨损</span>
              <input type="range" min="0.0001" max="0.80" step="0.001" v-model.number="gloveWear" style="width:160px;" />
              <span class="row-value tabular" style="width:54px;">{{ gloveWear.toFixed(4) }}</span>
              <button class="btn btn-sm" @click="gloveWear = 0.0001">极品崭新</button>
              <button class="btn btn-sm" @click="gloveWear = 0.075">略有磨损</button>
              <button class="btn btn-sm" @click="gloveWear = 0.16">久经沙场</button>
            </div>
            <div v-if="filteredGloves.length" class="grid">
              <div v-for="g in filteredGloves" :key="g.weapon_defindex + '-' + g.paint" class="card" :class="{ equipped: isGloveEquipped(g) }" @click="saveGlove(g)">
                <span v-if="isGloveEquipped(g)" class="card-check"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><path d="M20 6 9 17l-5-5" /></svg></span>
                <div class="card-art"><img v-if="g.image" :src="g.image" :alt="g.name" loading="lazy" /><span v-else style="font-size:13px;color:var(--label-tertiary);">默认原版</span></div>
                <div>
                  <div class="card-title">{{ g.name }}</div>
                  <div v-if="g.name_en && g.name_en !== g.name" class="card-subtitle">{{ g.name_en }}</div>
                  <div v-if="isGloveEquipped(g) && g.weapon_defindex !== 0 && equippedSkin(g.weapon_defindex)" class="card-pills">
                    <span class="pill-tag blue tabular">{{ formatWear(equippedSkin(g.weapon_defindex).weapon_wear) }}</span>
                  </div>
                </div>
              </div>
            </div>
            <div v-else class="empty-state">
              <div>未找到匹配「{{ searchQuery }}」的手套</div>
              <button class="btn btn-sm btn-tinted" @click="searchQuery = ''">清空搜索</button>
            </div>
          </section>

          <!-- AGENTS TAB -->
          <section v-else-if="tab === 'agents'">
            <div class="page-header">
              <div><h1 class="page-title">探员</h1><p class="page-subtitle">分别定制 T 与 CT 阵营模型</p></div>
              <div class="segmented">
                <button :class="{ active: agentTeam === 2 }" @click="agentTeam = 2">T 阵营</button>
                <button :class="{ active: agentTeam === 3 }" @click="agentTeam = 3">CT 阵营</button>
              </div>
            </div>
            <div v-if="filteredAgents.length" class="grid">
              <div v-for="a in filteredAgents" :key="a.team + '-' + a.model" class="card" :class="{ equipped: currentAgentModel === a.model }" @click="saveAgent(a)">
                <span v-if="currentAgentModel === a.model" class="card-check"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><path d="M20 6 9 17l-5-5" /></svg></span>
                <div class="card-art"><img v-if="a.image" :src="a.image" :alt="a.name" loading="lazy" /><span v-else style="font-size:13px;color:var(--label-tertiary);">默认探员</span></div>
                <div class="card-title">{{ a.name }}</div><div v-if="a.name_en && a.name_en !== a.name" class="card-subtitle">{{ a.name_en }}</div>
              </div>
            </div>
            <div v-else class="empty-state">
              <div>未找到匹配「{{ searchQuery }}」的探员</div>
              <button class="btn btn-sm btn-tinted" @click="searchQuery = ''">清空搜索</button>
            </div>
          </section>

          <!-- MUSIC TAB -->
          <section v-else-if="tab === 'music'">
            <div class="page-header"><div><h1 class="page-title">音乐盒</h1><p class="page-subtitle">点击即装备并自动同步到游戏</p></div></div>
            <div v-if="filteredMusic.length" class="grid">
              <div v-for="m in filteredMusic" :key="m.id" class="card" :class="{ equipped: currentMusicId === m.id }" @click="saveMusic(m.id)">
                <span v-if="currentMusicId === m.id" class="card-check"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><path d="M20 6 9 17l-5-5" /></svg></span>
                <div class="card-art"><img v-if="m.image" :src="m.image" :alt="m.name" loading="lazy" /><span v-else style="font-size:13px;color:var(--label-tertiary);">默认原声</span></div>
                <div class="card-title">{{ m.name }}</div><div v-if="m.name_en && m.name_en !== m.name" class="card-subtitle">{{ m.name_en }}</div>
              </div>
            </div>
            <div v-else class="empty-state">
              <div>未找到匹配「{{ searchQuery }}」的音乐盒</div>
              <button class="btn btn-sm btn-tinted" @click="searchQuery = ''">清空搜索</button>
            </div>
          </section>

          <!-- PINS TAB -->
          <section v-else-if="tab === 'pins'">
            <div class="page-header"><div><h1 class="page-title">收藏徽章</h1><p class="page-subtitle">默认展示前 120 项，顶部搜索可检索全部中英文徽章</p></div></div>
            <div v-if="filteredPins.length" class="grid">
              <div v-for="p in filteredPins" :key="p.id" class="card" :class="{ equipped: currentPinId === p.id }" @click="savePin(p.id)">
                <span v-if="currentPinId === p.id" class="card-check"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><path d="M20 6 9 17l-5-5" /></svg></span>
                <div class="card-art"><img v-if="p.image" :src="p.image" :alt="p.name" loading="lazy" /><span v-else style="font-size:13px;color:var(--label-tertiary);">无徽章</span></div>
                <div class="card-title">{{ p.name }}</div><div v-if="p.name_en && p.name_en !== p.name" class="card-subtitle">{{ p.name_en }}</div>
              </div>
            </div>
            <div v-else class="empty-state">
              <div>未找到匹配「{{ searchQuery }}」的收藏徽章</div>
              <button class="btn btn-sm btn-tinted" @click="searchQuery = ''">清空搜索</button>
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
        :is-knife="modalWeapon.category === 'knives'"
        :is-active-knife="modalWeapon.category === 'knives' && isKnifeActive(modalWeapon.weapon_name)"
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
import { ref, computed, watch, onMounted } from 'vue'
import SkinSheet from './components/SkinSheet.vue'
import AdminApp from './components/AdminApp.vue'

const booting = ref(true)
const saving = ref(false)
const syncing = ref(false)
const windowHost = computed(() => window.location.hostname || '127.0.0.1')
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
  { key: 'all', label: '全部' },
  { key: 'rifles', label: '步枪' },
  { key: 'snipers', label: '狙击' },
  { key: 'pistols', label: '手枪' },
  { key: 'smgs', label: '冲锋枪' },
  { key: 'heavy', label: '重武器' }
]

const searchPlaceholder = computed(() => ({
  weapons: '搜索武器或涂装（支持中英文/如 巨龙传说）',
  knives: '搜索匕首或涂装（如 蝴蝶刀 / 多普勒）',
  gloves: '搜索手套（中英文）',
  agents: '搜索探员（中英文）',
  music: '搜索音乐盒（中英文）',
  pins: '搜索徽章（中英文）'
}[tab.value] || '搜索'))

const teamModeLabel = computed(() =>
  teamMode.value === 2 ? '仅 T 阵营' : teamMode.value === 3 ? '仅 CT 阵营' : 'T + CT 双阵营'
)

watch(tab, () => {
  searchQuery.value = ''
})

watch(teamMode, (v) => {
  if (v === 2 || v === 3) {
    agentTeam.value = v
  }
})

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

function equippedSkin(defindex) {
  const t = loadout.value.skins_t || {}
  const ct = loadout.value.skins_ct || {}
  if (teamMode.value === 3) return ct[defindex] || null
  if (teamMode.value === 2) return t[defindex] || null
  return t[defindex] || ct[defindex] || null
}

function isTeamDiffSkin(defindex) {
  if (teamMode.value !== 0) return false
  const t = (loadout.value.skins_t || {})[defindex]
  const ct = (loadout.value.skins_ct || {})[defindex]
  if (!t && !ct) return false
  if (!t || !ct) return true
  return t.weapon_paint_id !== ct.weapon_paint_id
}

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

function matchesWeaponQuery(w, q) {
  if (!q) return true
  if (
    w.display_name.toLowerCase().includes(q) ||
    w.weapon_name.toLowerCase().includes(q) ||
    (w.display_name_en || '').toLowerCase().includes(q) ||
    weaponCardSkinName(w).toLowerCase().includes(q)
  ) {
    return true
  }
  return (w.skins || []).some(s =>
    s.short_name.toLowerCase().includes(q) ||
    s.name.toLowerCase().includes(q) ||
    (s.short_name_en || '').toLowerCase().includes(q) ||
    (s.name_en || '').toLowerCase().includes(q) ||
    String(s.paint_id) === q
  )
}

const gunWeapons = computed(() =>
  catalog.value.weapons.filter(w => w.category !== 'knives')
)

const knifeWeapons = computed(() =>
  catalog.value.weapons.filter(w => w.category === 'knives')
)

const filteredWeapons = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return gunWeapons.value.filter(w =>
    (weaponCat.value === 'all' || w.category === weaponCat.value) &&
    matchesWeaponQuery(w, q)
  )
})

const filteredKnifeWeapons = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return knifeWeapons.value.filter(w => matchesWeaponQuery(w, q))
})

const knifeT = computed(() => loadout.value.knife_t || '')
const knifeCT = computed(() => loadout.value.knife_ct || '')
const hasTeamKnifeSplit = computed(() => teamMode.value === 0 && knifeT.value !== knifeCT.value)
const hasAnyCustomKnife = computed(() => {
  if (teamMode.value === 2) return !!knifeT.value
  if (teamMode.value === 3) return !!knifeCT.value
  return !!knifeT.value || !!knifeCT.value
})

const currentKnifeName = computed(() => {
  if (teamMode.value === 3) return knifeCT.value
  if (teamMode.value === 2) return knifeT.value
  return knifeT.value || knifeCT.value
})

const currentKnifeWeapon = computed(() => {
  if (!currentKnifeName.value) return null
  return knifeWeapons.value.find(w => w.weapon_name === currentKnifeName.value) || null
})

function knifeDisplayByWeaponName(wn) {
  if (!wn) return '默认原版匕首'
  const k = knifeWeapons.value.find(x => x.weapon_name === wn) || catalog.value.knives.find(x => x.weapon_name === wn)
  return k ? k.display_name : wn
}

const activeKnifeHeadline = computed(() => {
  if (hasTeamKnifeSplit.value) {
    return `T：${knifeDisplayByWeaponName(knifeT.value)} · CT：${knifeDisplayByWeaponName(knifeCT.value)}`
  }
  return knifeDisplayByWeaponName(currentKnifeName.value)
})

const isDefaultKnifeActive = computed(() => {
  if (teamMode.value === 2) return !knifeT.value
  if (teamMode.value === 3) return !knifeCT.value
  return !knifeT.value && !knifeCT.value
})

const defaultKnifeTeamBadge = computed(() => {
  if (teamMode.value !== 0) return ''
  if (!knifeT.value && knifeCT.value) return 'T 原版中'
  if (knifeT.value && !knifeCT.value) return 'CT 原版中'
  return ''
})

function knifeTeamBadge(weaponName) {
  if (teamMode.value === 2) return knifeT.value === weaponName ? '使用中' : ''
  if (teamMode.value === 3) return knifeCT.value === weaponName ? '使用中' : ''
  const onT = knifeT.value === weaponName
  const onCT = knifeCT.value === weaponName
  if (onT && onCT) return '使用中'
  if (onT) return 'T 使用中'
  if (onCT) return 'CT 使用中'
  return ''
}

function isKnifeActive(weaponName) {
  return !!knifeTeamBadge(weaponName)
}

const filteredGloves = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return catalog.value.gloves.filter(g => !q || g.name.toLowerCase().includes(q) || (g.name_en || '').toLowerCase().includes(q))
})
function isGloveEquipped(g) {
  const curDef = teamMode.value === 3
    ? loadout.value.glove_ct
    : teamMode.value === 2
      ? loadout.value.glove_t
      : (loadout.value.glove_t || loadout.value.glove_ct)
  if (g.weapon_defindex === 0) return !curDef
  if (curDef !== g.weapon_defindex) return false
  const skin = equippedSkin(g.weapon_defindex)
  return skin && skin.weapon_paint_id === g.paint
}

const currentAgentModel = computed(() => agentTeam.value === 3 ? (loadout.value.agent_ct || '') : (loadout.value.agent_t || ''))
const filteredAgents = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return catalog.value.agents.filter(a => a.team === agentTeam.value && (!q || a.name.toLowerCase().includes(q) || (a.name_en || '').toLowerCase().includes(q)))
})

const currentMusicId = computed(() =>
  teamMode.value === 3
    ? (loadout.value.music_ct || 0)
    : teamMode.value === 2
      ? (loadout.value.music_t || 0)
      : (loadout.value.music_t || loadout.value.music_ct || 0)
)
const filteredMusic = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return catalog.value.music.filter(m => !q || m.name.toLowerCase().includes(q) || (m.name_en || '').toLowerCase().includes(q))
})

const currentPinId = computed(() =>
  teamMode.value === 3
    ? (loadout.value.pin_ct || 0)
    : teamMode.value === 2
      ? (loadout.value.pin_t || 0)
      : (loadout.value.pin_t || loadout.value.pin_ct || 0)
)
const filteredPins = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  const list = catalog.value.pins.filter(p => !q || p.name.toLowerCase().includes(q) || (p.name_en || '').toLowerCase().includes(q))
  return q ? list.slice(0, 200) : list.slice(0, 120)
})

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
    loadout.value = res.loadout
    modalWeapon.value = null
    if (payload.equip_knife) {
      showToast(res.auto_refreshed ? '已切换装备此匕首并同步涂装（无需输入 !wp）' : '已切换装备此匕首并保存涂装，进服后自动生效')
    } else {
      showToast(res.message || (res.auto_refreshed ? '已保存并自动刷新游戏内皮肤（无需输入 !wp）' : '已保存皮肤配置（进服后自动生效）'))
    }
  } catch (e) { showToast(e.message, true) } finally { saving.value = false }
}

async function saveKnife(knife) {
  try {
    const res = await apiReq('/api/loadout/knife', { method: 'PUT', body: JSON.stringify({ team: teamMode.value, knife }) })
    loadout.value = res.loadout
    showToast(res.message || (res.auto_refreshed ? '已切换匕首并自动刷新生效' : '已切换匕首，进服后自动生效'))
  } catch (e) { showToast(e.message, true) }
}

async function saveGlove(g) {
  try {
    const res = await apiReq('/api/loadout/glove', { method: 'PUT', body: JSON.stringify({ team: teamMode.value, weapon_defindex: g.weapon_defindex, weapon_paint_id: g.paint, weapon_wear: gloveWear.value, weapon_seed: 0 }) })
    loadout.value = res.loadout
    showToast(res.message || (res.auto_refreshed ? '已更新手套并自动刷新生效' : '已更新手套，进服后自动生效'))
  } catch (e) { showToast(e.message, true) }
}

async function saveAgent(a) {
  try {
    const res = await apiReq('/api/loadout/agent', { method: 'PUT', body: JSON.stringify({ team: a.team, model: a.model }) })
    loadout.value = res.loadout
    showToast(res.message || (res.auto_refreshed ? '已更新探员并自动刷新生效' : '已更新探员模型'))
  } catch (e) { showToast(e.message, true) }
}

async function saveMusic(id) {
  try {
    const res = await apiReq('/api/loadout/music', { method: 'PUT', body: JSON.stringify({ team: teamMode.value, music_id: id }) })
    loadout.value = res.loadout
    showToast(res.message || (res.auto_refreshed ? '已更新音乐盒并自动刷新生效' : '已更新音乐盒，进服后自动生效'))
  } catch (e) { showToast(e.message, true) }
}

async function savePin(id) {
  try {
    const res = await apiReq('/api/loadout/pin', { method: 'PUT', body: JSON.stringify({ team: teamMode.value, id }) })
    loadout.value = res.loadout
    showToast(res.message || (res.auto_refreshed ? '已更新徽章并自动刷新生效' : '已更新收藏徽章，进服后自动生效'))
  } catch (e) { showToast(e.message, true) }
}

async function resetAll() {
  if (!confirm('确定清空你的全部皮肤配置吗？')) return
  try {
    const res = await apiReq('/api/loadout', { method: 'DELETE' })
    loadout.value = res.loadout
    showToast(res.message || '已重置全部皮肤配置')
  } catch (e) { showToast(e.message, true) }
}

async function manualRefresh() {
  syncing.value = true
  try {
    const res = await apiReq('/api/loadout/refresh', { method: 'POST' })
    showToast(res.message || (res.auto_refreshed ? '已同步皮肤到游戏内' : '当前未在服内或待机，进服后将自动生效'))
  } catch (e) { showToast(e.message, true) } finally { syncing.value = false }
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
