<script setup lang="ts">
const { t } = useI18n()
</script>

<template>
  <div class="site-data-loader" role="status" aria-live="polite" :aria-label="t('site.dataLoading')">
    <div class="site-data-loader__field" aria-hidden="true">
      <span class="site-data-loader__grid" />
      <span class="site-data-loader__orbit site-data-loader__orbit--outer" />
      <span class="site-data-loader__orbit site-data-loader__orbit--inner" />
      <span class="site-data-loader__core" />
      <span v-for="index in 8" :key="index" class="site-data-loader__node" :style="{ '--node-index': index }" />
    </div>
    <p class="site-data-loader__label">{{ t('site.dataLoading') }}</p>
    <span class="site-data-loader__progress" aria-hidden="true"><span /></span>
  </div>
</template>

<style scoped>
.site-data-loader {
  display: grid;
  min-height: 16rem;
  place-items: center;
  align-content: center;
  gap: 0.75rem;
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--clay) 22%, var(--line));
  border-radius: var(--radius-card);
  background:
    radial-gradient(circle at 50% 42%, color-mix(in srgb, var(--clay) 10%, transparent), transparent 34%),
    var(--surface);
}

.site-data-loader__field {
  position: relative;
  width: 10rem;
  height: 7rem;
  perspective: 24rem;
}

.site-data-loader__grid {
  position: absolute;
  inset: 1rem 0.25rem 0.5rem;
  opacity: 0.45;
  background-image: linear-gradient(var(--line) 1px, transparent 1px), linear-gradient(90deg, var(--line) 1px, transparent 1px);
  background-size: 1rem 1rem;
  transform: perspective(10rem) rotateX(58deg) scale(1.15);
  mask-image: linear-gradient(to bottom, transparent, black 28%, transparent);
}

.site-data-loader__orbit {
  position: absolute;
  top: 50%;
  left: 50%;
  display: block;
  border: 1px solid color-mix(in srgb, var(--clay) 72%, transparent);
  border-radius: 999px;
  transform: translate(-50%, -50%) rotateX(64deg) rotateZ(0deg);
  animation: loader-spin 3.8s linear infinite;
}

.site-data-loader__orbit--outer {
  width: 8rem;
  height: 3.6rem;
  opacity: 0.65;
}

.site-data-loader__orbit--inner {
  width: 5.1rem;
  height: 2.3rem;
  opacity: 0.9;
  animation-duration: 2.7s;
  animation-direction: reverse;
}

.site-data-loader__core {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 1rem;
  height: 1rem;
  border-radius: 50%;
  background: var(--clay);
  box-shadow: 0 0 0 0 color-mix(in srgb, var(--clay) 42%, transparent);
  transform: translate(-50%, -50%);
  animation: loader-pulse 1.8s ease-out infinite;
}

.site-data-loader__node {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 0.3rem;
  height: 0.3rem;
  border-radius: 50%;
  background: var(--clay);
  opacity: 0.35;
  transform: translate(-50%, -50%);
  animation: loader-node 1.8s ease-in-out calc(var(--node-index) * -130ms) infinite;
}

.site-data-loader__node:nth-of-type(5) { transform: translate(3.65rem, -0.15rem); }
.site-data-loader__node:nth-of-type(6) { transform: translate(2.55rem, 1.55rem); }
.site-data-loader__node:nth-of-type(7) { transform: translate(0.15rem, 1.8rem); }
.site-data-loader__node:nth-of-type(8) { transform: translate(-2.25rem, 1.55rem); }
.site-data-loader__node:nth-of-type(9) { transform: translate(-3.65rem, -0.15rem); }
.site-data-loader__node:nth-of-type(10) { transform: translate(-2.55rem, -1.85rem); }
.site-data-loader__node:nth-of-type(11) { transform: translate(-0.15rem, -2.1rem); }
.site-data-loader__node:nth-of-type(12) { transform: translate(2.25rem, -1.85rem); }

.site-data-loader__label {
  margin: 0;
  color: var(--muted);
  font-family: var(--font-mono);
  font-size: 0.7rem;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.site-data-loader__progress {
  width: 7rem;
  height: 2px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--line);
}

.site-data-loader__progress span {
  display: block;
  width: 45%;
  height: 100%;
  border-radius: inherit;
  background: var(--clay);
  animation: loader-progress 1.4s ease-in-out infinite;
}

@keyframes loader-spin {
  to { transform: translate(-50%, -50%) rotateX(64deg) rotateZ(360deg); }
}

@keyframes loader-pulse {
  70% { box-shadow: 0 0 0 1.2rem color-mix(in srgb, var(--clay) 0%, transparent); }
  100% { box-shadow: 0 0 0 1.6rem color-mix(in srgb, var(--clay) 0%, transparent); }
}

@keyframes loader-node {
  50% { opacity: 1; }
}

@keyframes loader-progress {
  0% { transform: translateX(-160%); }
  100% { transform: translateX(360%); }
}

@media (prefers-reduced-motion: reduce) {
  .site-data-loader__orbit,
  .site-data-loader__core,
  .site-data-loader__node,
  .site-data-loader__progress span {
    animation: none;
  }
}
</style>
