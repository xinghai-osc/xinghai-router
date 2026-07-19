<script setup lang="ts">
import { Check, ChevronRight, KeyRound, WalletCards, Activity, ReceiptText, TerminalSquare } from 'lucide-vue-next'
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'

const store = useConsoleStore()
const { t, account, accountKeys, personalRequests, personalTokens, personalCost, setupProgress, apiEndpoint, usageChart, usageLinePoints, openConsole, setupCollapsed } = store
</script>

<template>
  <button class="setup-toggle" @click="setupCollapsed = !setupCollapsed">
    <ChevronRight :size="16" :class="{ rotated: !setupCollapsed }" />
    <span>{{ t('quickStartHeading') }} / {{ t('firstApiRequest') }}</span>
    <span class="setup-progress">{{ setupProgress }} / 3</span>
  </button>
  <section v-show="!setupCollapsed" class="setup-workspace">
    <div class="setup-guide">
      <div class="setup-heading">
        <div>
          <span class="overview-kicker">{{ t('quickStartHeading') }}</span>
          <h2>{{ account?.name || '' }}{{ t('sendModelRequest') }}</h2>
          <p>{{ t('completeThreeSteps') }}</p>
        </div>
      </div>
      <div class="setup-steps">
        <button :class="{ complete: accountKeys.some((item) => !item.revoked_at) }" @click="openConsole('account')">
          <i><Check v-if="accountKeys.some((item) => !item.revoked_at)" :size="14" /><span v-else>1</span></i>
          <span><b>{{ t('createApiKey') }}</b><small>{{ t('issueAccessCredentials') }}</small></span>
          <ChevronRight :size="16" />
        </button>
        <button :class="{ complete: Number(account?.balance ?? 0) > 0 }" @click="openConsole('wallet')">
          <i><Check v-if="Number(account?.balance ?? 0) > 0" :size="14" /><span v-else>2</span></i>
          <span><b>{{ t('checkAccountBalance') }}</b><small>{{ t('prepareAvailableQuota') }}</small></span>
          <ChevronRight :size="16" />
        </button>
        <button :class="{ complete: personalRequests > 0 }" @click="openConsole('usage')">
          <i><Check v-if="personalRequests > 0" :size="14" /><span v-else>3</span></i>
          <span><b>{{ t('sendModelRequest') }}</b><small>{{ t('confirmCallResults') }}</small></span>
          <ChevronRight :size="16" />
        </button>
      </div>
    </div>
    <div class="request-preview">
      <div class="request-preview-title">
        <span><TerminalSquare :size="16" />{{ t('firstApiRequest') }}</span>
        <code>curl</code>
      </div>
      <pre><span>curl {{ apiEndpoint }} \</span><span>  -H <i>"Authorization: Bearer sk-xh-..."</i> \</span><span>  -H <i>"Content-Type: application/json"</i> \</span><span>  -d <i>'{"model":"kimi-m3",</i></span><span><i>       "messages":[{"role":"user","content":"你好"}]}'</i></span></pre>
      <div class="request-signals">
        <span><i></i>{{ t('gatewayOnline') }}</span>
        <button @click="openConsole('account')">{{ t('viewKeys') }} <ChevronRight :size="14" /></button>
      </div>
    </div>
  </section>
  <div class="metrics">
    <article>
      <span>{{ t('availableBalance') }}</span>
      <strong>{{ Number(account?.balance ?? 0).toFixed(4) }}</strong>
      <p><WalletCards :size="15" />{{ t('exclusiveOfReserved') }}</p>
    </article>
    <article>
      <span>{{ t('validApiKeys') }}</span>
      <strong>{{ accountKeys.filter((item) => !item.revoked_at).length }}</strong>
      <p><KeyRound :size="15" />{{ t('currentAccountKeys') }}</p>
    </article>
    <article>
      <span>{{ t('recentCalls') }}</span>
      <strong>{{ personalRequests }}</strong>
      <p><Activity :size="15" />{{ t('recent100Records') }}</p>
    </article>
    <article>
      <span>{{ t('meteredTokens') }}</span>
      <strong>{{ personalTokens.toLocaleString() }}</strong>
      <p><ReceiptText :size="15" />{{ t('cumulativeCost') }} {{ personalCost.toFixed(6) }}</p>
    </article>
  </div>
  <div class="grid-two">
    <section class="panel usage-line-chart">
      <div class="panel-title">
        <div><h2>{{ t('usageTrend') }}</h2><p>{{ t('last7DaysTokenUsage') }}</p></div>
        <button class="text-button" @click="openConsole('usage')">{{ t('viewAll') }}</button>
      </div>
      <div class="line-plot">
        <svg viewBox="0 0 100 100" preserveAspectRatio="none" :aria-label="t('last7DaysTokens')">
          <defs>
            <linearGradient id="usageFill" x1="0" x2="0" y1="0" y2="1">
              <stop offset="0%" stop-color="#65a986" stop-opacity=".34" />
              <stop offset="100%" stop-color="#65a986" stop-opacity="0" />
            </linearGradient>
          </defs>
          <path :d="`M 0,100 L ${usageLinePoints} L 100,100 Z`" fill="url(#usageFill)" />
          <polyline :points="usageLinePoints" fill="none" stroke="#2d7657" stroke-linecap="round" stroke-linejoin="round" stroke-width="2" vector-effect="non-scaling-stroke" />
        </svg>
        <div class="line-labels">
          <span v-for="day in usageChart" :key="day.key">{{ day.label }}<b>{{ day.tokens ? day.tokens.toLocaleString() : '-' }}</b></span>
        </div>
      </div>
    </section>
    <section class="panel">
      <div class="panel-title">
        <div><h2>{{ t('accessKeys') }}</h2><p>{{ t('currentAccountKeysDesc') }}</p></div>
        <button class="text-button" @click="openConsole('account')">{{ t('myAccount') }}</button>
      </div>
      <div v-if="accountKeys.length" class="compact-list">
        <div v-for="key in accountKeys.slice(0, 5)" :key="key.id">
          <code>{{ key.key_prefix }}...</code>
          <span>{{ key.name }}</span>
          <b :class="key.revoked_at ? 'danger' : 'success'">{{ key.revoked_at ? t('revoked') : t('valid') }}</b>
        </div>
      </div>
      <Empty v-else :text="t('noApiKeysYet')" />
    </section>
  </div>
</template>
