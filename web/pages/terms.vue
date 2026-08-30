<script setup lang="ts">
interface Section { id: string; titleKey: string; blocks: { kind: 'p' | 'ul'; keys: string[] }[] }

const { t } = useI18n()
const { settings } = useSiteSettings()
const contactEmail = computed(() => settings.value.contact_email?.trim() || t('site.legalEmail'))

useHead({
  title: () => `${t('site.termsMetaTitle')} · ${settings.value.name}`,
  meta: [{ name: 'description', content: () => t('site.termsMetaDescription') }],
})

const SECTIONS: Section[] = [
  { id: 'service', titleKey: 'site.termsS1Title', blocks: [{ kind: 'p', keys: ['site.termsS1P1', 'site.termsS1P2'] }] },
  {
    id: 'accounts',
    titleKey: 'site.termsS2Title',
    blocks: [
      { kind: 'p', keys: ['site.termsS2P1'] },
      { kind: 'ul', keys: ['site.termsS2L1', 'site.termsS2L2', 'site.termsS2L3', 'site.termsS2L4', 'site.termsS2L5'] },
    ],
  },
  {
    id: 'acceptable-use',
    titleKey: 'site.termsS3Title',
    blocks: [
      { kind: 'p', keys: ['site.termsS3P1'] },
      { kind: 'ul', keys: ['site.termsS3L1', 'site.termsS3L2', 'site.termsS3L3', 'site.termsS3L4', 'site.termsS3L5', 'site.termsS3L6'] },
      { kind: 'p', keys: ['site.termsS3P2'] },
    ],
  },
  {
    id: 'billing',
    titleKey: 'site.termsS4Title',
    blocks: [
      { kind: 'p', keys: ['site.termsS4P1'] },
      { kind: 'ul', keys: ['site.termsS4L1', 'site.termsS4L2', 'site.termsS4L3', 'site.termsS4L4', 'site.termsS4L5'] },
    ],
  },
  { id: 'availability', titleKey: 'site.termsS5Title', blocks: [{ kind: 'p', keys: ['site.termsS5P1', 'site.termsS5P2'] }] },
  { id: 'ip', titleKey: 'site.termsS6Title', blocks: [{ kind: 'p', keys: ['site.termsS6P1'] }] },
  { id: 'liability', titleKey: 'site.termsS7Title', blocks: [{ kind: 'p', keys: ['site.termsS7P1', 'site.termsS7P2'] }] },
  { id: 'termination', titleKey: 'site.termsS8Title', blocks: [{ kind: 'p', keys: ['site.termsS8P1'] }] },
  { id: 'changes', titleKey: 'site.termsS9Title', blocks: [{ kind: 'p', keys: ['site.termsS9P1'] }] },
]

const toc = computed(() => [
  ...SECTIONS.map(section => ({ id: section.id, label: t(section.titleKey) })),
  { id: 'contact', label: t('site.termsS10Title') },
])
</script>

<template>
  <SiteLegalDoc
    :title="t('site.termsTitle')"
    :updated="t('site.termsUpdatedAt')"
    :intro="t('site.termsIntro')"
    :toc="toc"
  >
    <section v-for="section in SECTIONS" :id="section.id" :key="section.id">
      <h2>{{ t(section.titleKey) }}</h2>
      <template v-for="(block, index) in section.blocks" :key="index">
        <template v-if="block.kind === 'p'">
          <p v-for="key in block.keys" :key="key">{{ t(key) }}</p>
        </template>
        <ul v-else>
          <li v-for="key in block.keys" :key="key">{{ t(key) }}</li>
        </ul>
      </template>
    </section>

    <section id="contact">
      <h2>{{ t('site.termsS10Title') }}</h2>
      <p>{{ t('site.termsS10P1', { email: contactEmail }) }}</p>
      <p>
        <NuxtLink to="/privacy" class="text-clay underline-offset-4 hover:underline">{{ t('nav.privacy') }}</NuxtLink>
      </p>
    </section>
  </SiteLegalDoc>
</template>
