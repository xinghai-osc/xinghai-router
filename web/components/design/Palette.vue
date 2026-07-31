<script setup lang="ts">
const surfaces = [
  { token: 'paper', label: '页面底', class: 'bg-paper' },
  { token: 'surface', label: '卡片', class: 'bg-surface' },
  { token: 'sunken', label: '内嵌', class: 'bg-sunken' },
  { token: 'line', label: '边框', class: 'bg-line' },
]

const accents = [
  { token: 'clay', label: '强调', class: 'bg-clay' },
  { token: 'clay-soft', label: '强调底', class: 'bg-clay-soft' },
  { token: 'success', label: '成功', class: 'bg-success' },
  { token: 'warn', label: '警告', class: 'bg-warn' },
  { token: 'danger', label: '危险', class: 'bg-danger' },
]

const inks = [
  { token: 'ink', label: '主文本', class: 'bg-ink' },
  { token: 'muted', label: '次要', class: 'bg-muted' },
  { token: 'faint', label: '弱化', class: 'bg-faint' },
]
</script>

<template>
  <div class="space-y-6">
    <div v-for="row in [{ name: '表面', items: surfaces }, { name: '强调与语义', items: accents }, { name: '文本', items: inks }]" :key="row.name" class="space-y-2">
      <p class="text-2xs font-medium tracking-wide text-faint uppercase">{{ row.name }}</p>
      <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
        <div v-for="swatch in row.items" :key="swatch.token" class="space-y-1.5">
          <div :class="['h-14 rounded-control border border-line', swatch.class]" />
          <div class="space-y-0.5">
            <p class="text-[13px] text-ink">{{ swatch.label }}</p>
            <p class="numeric text-2xs text-faint">--{{ swatch.token }}</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
