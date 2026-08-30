<script setup lang="ts">
interface Section { id: string; titleKey: string; blocks: { kind: 'p' | 'ul'; keys: string[] }[] }

const { t } = useI18n()
const { settings } = useSiteSettings()
const contactEmail = computed(() => settings.value.contact_email?.trim() || t('site.legalPrivacyEmail'))

useHead({
  title: () => `${t('site.privacyMetaTitle')} · ${settings.value.name}`,
  meta: [{ name: 'description', content: () => t('site.privacyMetaDescription') }],
})

const SECTIONS: Section[] = [
  {
    id: 'collected',
    titleKey: 'site.privacyS1Title',
    blocks: [
      { kind: 'p', keys: ['site.privacyS1P1'] },
      { kind: 'ul', keys: ['site.privacyS1L1', 'site.privacyS1L2', 'site.privacyS1L3', 'site.privacyS1L4', 'site.privacyS1L5'] },
      { kind: 'p', keys: ['site.privacyS1P2'] },
    ],
  },
  {
    id: 'usage',
    titleKey: 'site.privacyS2Title',
    blocks: [
      { kind: 'p', keys: ['site.privacyS2P1'] },
      { kind: 'ul', keys: ['site.privacyS2L1', 'site.privacyS2L2', 'site.privacyS2L3', 'site.privacyS2L4', 'site.privacyS2L5'] },
      { kind: 'p', keys: ['site.privacyS2P2'] },
    ],
  },
  { id: 'content', titleKey: 'site.privacyS3Title', blocks: [{ kind: 'p', keys: ['site.privacyS3P1', 'site.privacyS3P2'] }] },
  {
    id: 'cookies',
    titleKey: 'site.privacyS4Title',
    blocks: [
      { kind: 'p', keys: ['site.privacyS4P1'] },
      { kind: 'ul', keys: ['site.privacyS4L1', 'site.privacyS4L2', 'site.privacyS4L3'] },
    ],
  },
  {
    id: 'retention',
    titleKey: 'site.privacyS5Title',
    blocks: [
      { kind: 'p', keys: ['site.privacyS5P1'] },
      { kind: 'ul', keys: ['site.privacyS5L1', 'site.privacyS5L2', 'site.privacyS5L3', 'site.privacyS5L4'] },
    ],
  },
  {
    id: 'sharing',
    titleKey: 'site.privacyS6Title',
    blocks: [
      { kind: 'p', keys: ['site.privacyS6P1'] },
      { kind: 'ul', keys: ['site.privacyS6L1', 'site.privacyS6L2', 'site.privacyS6L3', 'site.privacyS6L4'] },
    ],
  },
  { id: 'security', titleKey: 'site.privacyS7Title', blocks: [{ kind: 'p', keys: ['site.privacyS7P1'] }] },
  {
    id: 'rights',
    titleKey: 'site.privacyS8Title',
    blocks: [
      { kind: 'p', keys: ['site.privacyS8P1'] },
      { kind: 'ul', keys: ['site.privacyS8L1', 'site.privacyS8L2', 'site.privacyS8L3', 'site.privacyS8L4', 'site.privacyS8L5'] },
      { kind: 'p', keys: ['site.privacyS8P2'] },
    ],
  },
  { id: 'children', titleKey: 'site.privacyS9Title', blocks: [{ kind: 'p', keys: ['site.privacyS9P1'] }] },
]

const toc = computed(() => [
  ...SECTIONS.map(section => ({ id: section.id, label: t(section.titleKey) })),
  { id: 'contact', label: t('site.privacyS10Title') },
])
</script>

<template>
  <SiteLegalDoc
    :title="t('site.privacyTitle')"
    :updated="t('site.privacyUpdatedAt')"
    :intro="t('site.privacyIntro')"
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
      <h2>{{ t('site.privacyS10Title') }}</h2>
      <p>{{ t('site.privacyS10P1', { email: contactEmail }) }}</p>
      <p>
        <NuxtLink to="/terms" class="text-clay underline-offset-4 hover:underline">{{ t('nav.terms') }}</NuxtLink>
      </p>
    </section>
  </SiteLegalDoc>
</template>
