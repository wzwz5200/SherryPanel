<template>
  <div class="sheet-backdrop" @click.self="$emit('close')">
    <div class="sheet">
      <div class="sheet-header">
        <div>
          <div class="sheet-title">{{ weapon.display_name }}</div>
          <div class="sheet-sub">
            共 {{ weapon.skins.length }} 款涂装 · 应用范围：{{ teamModeLabel }} · 保存即自动刷新
          </div>
        </div>
        <div style="display:flex;gap:8px;">
          <button class="btn" @click="$emit('close')">取消</button>
          <button class="btn btn-accent" :disabled="saving" @click="submit">
            {{ saving ? '保存中...' : '保存并应用' }}
          </button>
        </div>
      </div>

      <div class="sheet-body">
        <div class="sheet-inspector">
          <div class="preview-stage">
            <img :src="selectedSkin?.image || weapon.default_image" :alt="weapon.display_name" />
            <div style="font-size:15px;font-weight:600;margin-top:10px;letter-spacing:-0.15px;">
              {{ selectedSkin?.short_name || '默认原版' }}<div v-if='selectedSkin?.short_name_en && selectedSkin.short_name_en !== selectedSkin.short_name' style='font-size:12px;color:var(--label-secondary);font-weight:400;margin-top:2px;'>{{ selectedSkin.short_name_en }}</div>
            </div>
            <div class="tabular" style="font-size:12px;color:var(--label-secondary);margin-top:2px;">
              涂装编号 #{{ form.paintId }}
            </div>
          </div>

          <div class="settings-group">
            <div class="settings-row">
              <div class="row-label">
                <span>磨损度 (Float)</span>
                <span class="row-value tabular">{{ wearLabel(form.wear) }} · {{ Number(form.wear).toFixed(4) }}</span>
              </div>
              <input type="range" min="0.0001" max="1" step="0.0001" v-model.number="form.wear" />
              <div class="wear-grid">
                <button class="wear-btn" @click="form.wear = 0.0001">崭新 0.0001</button>
                <button class="wear-btn" @click="form.wear = 0.075">略磨 0.075</button>
                <button class="wear-btn" @click="form.wear = 0.16">酒精 0.16</button>
                <button class="wear-btn" @click="form.wear = 0.39">破损 0.39</button>
                <button class="wear-btn" @click="form.wear = 0.65">战痕 0.65</button>
                <button class="wear-btn" @click="form.wear = 0.99">极限 0.99</button>
              </div>
            </div>

            <div class="settings-row">
              <div class="row-label">
                <span>图案模板 (Seed)</span>
                <button class="btn btn-sm btn-tinted" @click="form.seed = Math.floor(Math.random() * 1001)">
                  随机模板
                </button>
              </div>
              <input class="input-apple tabular" type="number" min="0" max="1000" v-model.number="form.seed" />
            </div>

            <div class="settings-row">
              <div class="row-label">
                <span>自定义名称标签</span>
              </div>
              <input class="input-apple" type="text" maxlength="32" v-model="form.nametag" placeholder="留空不显示标签" />
            </div>

            <div class="settings-row-inline">
              <div>
                <div style="font-size:13px;font-weight:500;">StatTrak™ 暗金计数</div>
                <div style="font-size:11.5px;color:var(--label-secondary);">在武器上显示击杀计数器</div>
              </div>
              <label class="ios-switch">
                <input type="checkbox" v-model="form.stattrak" />
                <span class="ios-slider"></span>
              </label>
            </div>
          </div>
        </div>

        <div class="sheet-picker">
          <div class="search-field" style="width:100%;">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" />
            </svg>
            <input
              v-model="search"
              style="width:100%;"
              placeholder="中英文搜索涂装或编号（如 火神 / Vulcan / Doppler / 巨龙传说）"
            />
            <button v-if="search" class="search-clear" @click="search = ''">×</button>
          </div>

          <div class="skin-grid">
            <div
              v-for="s in filteredSkins"
              :key="s.paint_id"
              class="skin-tile"
              :class="{ selected: form.paintId === s.paint_id }"
              @click="form.paintId = s.paint_id"
            >
              <img :src="s.image" :alt="s.short_name" loading="lazy" />
              <div class="skin-tile-name" :title="s.name">{{ s.short_name }}</div>
              <div v-if="s.short_name_en && s.short_name_en !== s.short_name" style="font-size:10.5px;color:var(--label-tertiary);white-space:nowrap;overflow:hidden;text-overflow:ellipsis;">{{ s.short_name_en }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'

const props = defineProps({
  weapon: { type: Object, required: true },
  initial: { type: Object, default: null },
  teamModeLabel: { type: String, required: true },
  saving: { type: Boolean, default: false }
})

const emit = defineEmits(['close', 'save'])

const search = ref('')
const form = ref({
  paintId: 0,
  wear: 0.0001,
  seed: 0,
  nametag: '',
  stattrak: false
})

watch(() => [props.weapon, props.initial], () => {
  search.value = ''
  form.value = {
    paintId: props.initial ? props.initial.weapon_paint_id : 0,
    wear: props.initial ? Number(props.initial.weapon_wear) : 0.0001,
    seed: props.initial ? props.initial.weapon_seed : 0,
    nametag: props.initial ? (props.initial.weapon_nametag || '') : '',
    stattrak: props.initial ? !!props.initial.weapon_stattrak : false
  }
}, { immediate: true })

const filteredSkins = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return props.weapon.skins
  return props.weapon.skins.filter(s =>
    s.name.toLowerCase().includes(q) ||
    s.short_name.toLowerCase().includes(q) ||
    (s.name_en || '').toLowerCase().includes(q) ||
    (s.short_name_en || '').toLowerCase().includes(q) ||
    String(s.paint_id).includes(q)
  )
})

const selectedSkin = computed(() =>
  props.weapon.skins.find(s => s.paint_id === form.value.paintId) || props.weapon.skins[0]
)

function wearLabel(w) {
  const v = Number(w)
  if (v < 0.07) return '崭新出厂'
  if (v < 0.15) return '略有磨损'
  if (v < 0.38) return '久经沙场'
  if (v < 0.45) return '破损不堪'
  return '战痕累累'
}

function submit() {
  emit('save', {
    weapon_defindex: props.weapon.defindex,
    weapon_paint_id: Number(form.value.paintId),
    weapon_wear: Number(form.value.wear),
    weapon_seed: Number(form.value.seed),
    weapon_nametag: form.value.nametag,
    weapon_stattrak: !!form.value.stattrak,
    weapon_stattrak_count: 0
  })
}
</script>
