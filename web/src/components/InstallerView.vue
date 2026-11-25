<template>
  <section>
    <div class="page-header">
      <div>
        <h1 class="page-title">自动部署与更新</h1>
        <p class="page-subtitle">新服务器运行二进制后可直接在此一键下载 CS2 服务端（AppID 730）及全套换肤插件栈</p>
      </div>
      <button class="btn" @click="poll">刷新状态</button>
    </div>

    <div class="grid" style="grid-template-columns:repeat(auto-fit,minmax(240px,1fr));margin-bottom:18px;">
      <div class="card" style="cursor:default;">
        <div class="card-subtitle">CS2 服务端 (AppID 730)</div>
        <div style="font-size:18px;font-weight:700;margin-top:6px;">
          {{ metrics.cs2_installed ? '已安装就绪' : '尚未安装' }}
        </div>
        <div class="card-subtitle tabular" style="margin-top:6px;">{{ paths.cs2_dir }}</div>
      </div>

      <div class="card" style="cursor:default;">
        <div class="card-subtitle">全皮肤插件栈</div>
        <div style="font-size:18px;font-weight:700;margin-top:6px;">
          {{ metrics.plugins_ready ? 'Metamod + CSS + WP 已就绪' : '尚未安装' }}
        </div>
        <div class="card-subtitle" style="margin-top:6px;">含 AnyBaseLib / PlayerSettings / MenuManager</div>
      </div>

      <div class="card" style="cursor:default;">
        <div class="card-subtitle">可用磁盘空间</div>
        <div class="tabular" style="font-size:18px;font-weight:700;margin-top:6px;">
          {{ (metrics.disk_free_gb || 0).toFixed(1) }} GB 可用
        </div>
        <div class="card-subtitle" style="margin-top:6px;">完整 CS2 下载建议预留 ≥ 90 GB</div>
      </div>
    </div>

    <div class="settings-group" style="margin-bottom:18px;">
      <div class="settings-row-inline">
        <div>
          <div style="font-size:14px;font-weight:600;">1. 自动下载 / 校验更新 CS2 服务端</div>
          <div style="font-size:12px;color:var(--label-secondary);">自动安装 32 位依赖、SteamCMD、下载/更新 CS2（~71 GB）并修复 steamclient.so</div>
        </div>
        <button class="btn btn-accent" :disabled="inst.running" @click="runTask('cs2')">
          {{ inst.running && inst.task === 'cs2' ? '执行中...' : '下载 / 更新 CS2' }}
        </button>
      </div>

      <div class="settings-row-inline">
        <div>
          <div style="font-size:14px;font-weight:600;">2. 自动安装 / 更新全皮肤插件栈与中文图库</div>
          <div style="font-size:12px;color:var(--label-secondary);">自动拉取最新版 Metamod、CounterStrikeSharp、WeaponPaints、依赖插件及本地皮肤图片</div>
        </div>
        <button class="btn btn-tinted" :disabled="inst.running" @click="runTask('plugins')">
          {{ inst.running && inst.task === 'plugins' ? '执行中...' : '安装 / 更新插件栈' }}
        </button>
      </div>

      <div class="settings-row-inline">
        <div>
          <div style="font-size:14px;font-weight:600;">3. 一键全自动完整部署（全新机器推荐）</div>
          <div style="font-size:12px;color:var(--label-secondary);">按顺序自动完成 CS2 服务端下载 + 全皮肤插件与中文图库部署</div>
        </div>
        <button class="btn btn-accent" :disabled="inst.running" @click="runTask('all')">
          {{ inst.running && inst.task === 'all' ? '执行中...' : '一键全自动部署' }}
        </button>
      </div>
    </div>

    <div class="settings-group">
      <div class="settings-row">
        <div class="row-label">
          <span>实时部署日志 · {{ inst.stage || '空闲' }}</span>
          <span v-if="inst.running" class="row-value">任务运行中（每 2 秒自动刷新）</span>
          <span v-else-if="inst.last_error" style="color:var(--red);font-size:12px;">错误: {{ inst.last_error }}</span>
        </div>
        <pre class="log-box tabular">{{ (inst.logs && inst.logs.length) ? inst.logs.join('\n') : '点击上方任意安装/更新按钮后，此处将实时显示下载与安装进度日志。' }}</pre>
      </div>
    </div>
  </section>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'

const emit = defineEmits(['toast'])

const metrics = ref({})
const paths = ref({})
const inst = ref({ running: false, task: '', stage: '空闲', last_error: '', logs: [] })
let timer = null

async function poll() {
  try {
    const [sRes, iRes] = await Promise.all([
      fetch('/api/server/status').then(r => r.json()),
      fetch('/api/installer/status').then(r => r.json())
    ])
    metrics.value = sRes.metrics || {}
    paths.value = sRes.paths || {}
    inst.value = iRes || inst.value
  } catch {}
}

async function runTask(task) {
  if (!confirm(`确定开始执行任务 [${task}] 吗？如果下载 CS2 完整服务端可能需要较长时间。`)) return
  try {
    const res = await fetch('/api/installer/run', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ task })
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '启动失败')
    emit('toast', `已启动后台任务: ${task}`)
    await poll()
  } catch (e) {
    emit('toast', e.message, true)
  }
}

onMounted(() => {
  poll()
  timer = setInterval(poll, 2500)
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
  min-height: 180px;
  max-height: 360px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
