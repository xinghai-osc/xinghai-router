<script setup lang="ts">
const props = defineProps<{ name?: string; iconUrl?: string; compact?: boolean }>()

const { t } = useI18n()
const displayName = computed(() => props.name || t('common.brand'))
</script>

<template>
  <NuxtLink to="/" class="group inline-flex min-w-0 items-center gap-2.5" :aria-label="t('common.home')">
    <img v-if="iconUrl" :src="iconUrl" alt="" class="size-7 rounded-[8px] object-cover transition-transform duration-150 group-hover:scale-105">
    <svg v-else class="size-7 shrink-0 transition-transform duration-150 group-hover:scale-105" viewBox="0 0 28 28" fill="none" aria-hidden="true">
      <rect width="28" height="28" rx="8" fill="var(--clay)" />
      <path
        d="M8 19c0-6.075 4.925-11 11-11"
        stroke="var(--clay-ink)"
        stroke-width="2"
        stroke-linecap="round"
        opacity=".55"
      />
      <path d="M8 19h5.5a5.5 5.5 0 0 0 5.5-5.5V8" stroke="var(--clay-ink)" stroke-width="2" stroke-linecap="round" />
      <circle cx="19" cy="8" r="1.75" fill="var(--clay-ink)" />
    </svg>
    <span v-if="!compact" class="max-w-36 truncate text-[15px] font-semibold tracking-tight whitespace-nowrap text-ink sm:max-w-52 lg:max-w-64">
      {{ displayName }}
    </span>
  </NuxtLink>
</template>
