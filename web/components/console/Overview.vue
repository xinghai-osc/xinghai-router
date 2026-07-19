<script setup lang="ts">
import { Check, ChevronRight, KeyRound, WalletCards, Activity, ReceiptText, TerminalSquare } from 'lucide-vue-next'
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'

const store = useConsoleStore()
const { t, account, accountKeys, personalRequests, personalTokens, personalCost, setupProgress, apiEndpoint, usageChart, usageLinePoints, openConsole, setupCollapsed } = store
</script>

<template>
  <Button variant="ghost" class="w-fit" @click="setupCollapsed = !setupCollapsed">
    <ChevronRight :size="16" :class="{ 'rotate-90': !setupCollapsed }" />
    <span>{{ t('quickStartHeading') }} / {{ t('firstApiRequest') }}</span>
    <Badge variant="secondary" class="ml-2">{{ setupProgress }} / 3</Badge>
  </Button>
  <section v-show="!setupCollapsed" class="mt-3 grid gap-4 lg:grid-cols-2">
    <Card>
      <CardContent class="pt-6">
        <span class="text-xs uppercase tracking-wide text-muted-foreground">{{ t('quickStartHeading') }}</span>
        <h2 class="text-lg font-semibold">{{ account?.name || '' }}{{ t('sendModelRequest') }}</h2>
        <p class="text-sm text-muted-foreground">{{ t('completeThreeSteps') }}</p>
        <div class="mt-4 flex flex-col gap-2">
          <button class="flex items-center gap-3 rounded-md border border-border p-3 text-left transition-colors hover:bg-accent" :class="accountKeys.some((item) => !item.revoked_at) ? 'border-green-500/40 bg-green-500/5' : ''" @click="openConsole('account')">
            <span class="flex h-6 w-6 items-center justify-center rounded-full text-xs" :class="accountKeys.some((item) => !item.revoked_at) ? 'bg-green-600 text-white' : 'bg-muted text-muted-foreground'">
              <Check v-if="accountKeys.some((item) => !item.revoked_at)" :size="14" /><span v-else>1</span>
            </span>
            <span class="flex-1">
              <div class="text-sm font-medium">{{ t('createApiKey') }}</div>
              <div class="text-xs text-muted-foreground">{{ t('issueAccessCredentials') }}</div>
            </span>
            <ChevronRight :size="16" class="text-muted-foreground" />
          </button>
          <button class="flex items-center gap-3 rounded-md border border-border p-3 text-left transition-colors hover:bg-accent" :class="Number(account?.balance ?? 0) > 0 ? 'border-green-500/40 bg-green-500/5' : ''" @click="openConsole('wallet')">
            <span class="flex h-6 w-6 items-center justify-center rounded-full text-xs" :class="Number(account?.balance ?? 0) > 0 ? 'bg-green-600 text-white' : 'bg-muted text-muted-foreground'">
              <Check v-if="Number(account?.balance ?? 0) > 0" :size="14" /><span v-else>2</span>
            </span>
            <span class="flex-1">
              <div class="text-sm font-medium">{{ t('checkAccountBalance') }}</div>
              <div class="text-xs text-muted-foreground">{{ t('prepareAvailableQuota') }}</div>
            </span>
            <ChevronRight :size="16" class="text-muted-foreground" />
          </button>
          <button class="flex items-center gap-3 rounded-md border border-border p-3 text-left transition-colors hover:bg-accent" :class="personalRequests > 0 ? 'border-green-500/40 bg-green-500/5' : ''" @click="openConsole('usage')">
            <span class="flex h-6 w-6 items-center justify-center rounded-full text-xs" :class="personalRequests > 0 ? 'bg-green-600 text-white' : 'bg-muted text-muted-foreground'">
              <Check v-if="personalRequests > 0" :size="14" /><span v-else>3</span>
            </span>
            <span class="flex-1">
              <div class="text-sm font-medium">{{ t('sendModelRequest') }}</div>
              <div class="text-xs text-muted-foreground">{{ t('confirmCallResults') }}</div>
            </span>
            <ChevronRight :size="16" class="text-muted-foreground" />
          </button>
        </div>
      </CardContent>
    </Card>
    <Card>
      <CardContent class="pt-6">
        <div class="flex items-center justify-between">
          <span class="flex items-center gap-2 text-sm font-medium"><TerminalSquare :size="16" />{{ t('firstApiRequest') }}</span>
          <code class="rounded bg-muted px-2 py-0.5 font-mono text-xs">curl</code>
        </div>
        <pre class="mt-3 overflow-x-auto rounded-md bg-muted/50 p-3 font-mono text-xs leading-relaxed"><span>curl {{ apiEndpoint }} \</span>
<span>  -H <i>"Authorization: Bearer sk-xh-..."</i> \</span>
<span>  -H <i>"Content-Type: application/json"</i> \</span>
<span>  -d <i>'{"model":"kimi-m3",</i></span>
<span><i>       "messages":[{"role":"user","content":"你好"}]}'</i></span></pre>
        <div class="mt-3 flex items-center justify-between">
          <span class="flex items-center gap-2 text-xs text-muted-foreground"><span class="h-2 w-2 animate-pulse rounded-full bg-green-500" />{{ t('gatewayOnline') }}</span>
          <Button variant="link" size="sm" @click="openConsole('account')">{{ t('viewKeys') }} <ChevronRight :size="14" /></Button>
        </div>
      </CardContent>
    </Card>
  </section>

  <div class="mt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
    <Card>
      <CardContent class="pt-6">
        <span class="text-xs text-muted-foreground">{{ t('availableBalance') }}</span>
        <div class="text-2xl font-semibold">{{ Number(account?.balance ?? 0).toFixed(4) }}</div>
        <p class="mt-1 flex items-center gap-1 text-xs text-muted-foreground"><WalletCards :size="13" />{{ t('exclusiveOfReserved') }}</p>
      </CardContent>
    </Card>
    <Card>
      <CardContent class="pt-6">
        <span class="text-xs text-muted-foreground">{{ t('validApiKeys') }}</span>
        <div class="text-2xl font-semibold">{{ accountKeys.filter((item) => !item.revoked_at).length }}</div>
        <p class="mt-1 flex items-center gap-1 text-xs text-muted-foreground"><KeyRound :size="13" />{{ t('currentAccountKeys') }}</p>
      </CardContent>
    </Card>
    <Card>
      <CardContent class="pt-6">
        <span class="text-xs text-muted-foreground">{{ t('recentCalls') }}</span>
        <div class="text-2xl font-semibold">{{ personalRequests }}</div>
        <p class="mt-1 flex items-center gap-1 text-xs text-muted-foreground"><Activity :size="13" />{{ t('recent100Records') }}</p>
      </CardContent>
    </Card>
    <Card>
      <CardContent class="pt-6">
        <span class="text-xs text-muted-foreground">{{ t('meteredTokens') }}</span>
        <div class="text-2xl font-semibold">{{ personalTokens.toLocaleString() }}</div>
        <p class="mt-1 flex items-center gap-1 text-xs text-muted-foreground"><ReceiptText :size="13" />{{ t('cumulativeCost') }} {{ personalCost.toFixed(6) }}</p>
      </CardContent>
    </Card>
  </div>

  <div class="mt-4 grid gap-4 lg:grid-cols-2">
    <Card>
      <CardContent class="pt-6">
        <div class="mb-4 flex items-center justify-between">
          <div>
            <h2 class="text-sm font-semibold">{{ t('usageTrend') }}</h2>
            <p class="text-xs text-muted-foreground">{{ t('last7DaysTokenUsage') }}</p>
          </div>
          <Button variant="link" size="sm" @click="openConsole('usage')">{{ t('viewAll') }}</Button>
        </div>
        <div class="relative">
          <svg viewBox="0 0 100 100" preserveAspectRatio="none" class="h-40 w-full" :aria-label="t('last7DaysTokens')">
            <defs>
              <linearGradient id="usageFill" x1="0" x2="0" y1="0" y2="1">
                <stop offset="0%" stop-color="#65a986" stop-opacity=".34" />
                <stop offset="100%" stop-color="#65a986" stop-opacity="0" />
              </linearGradient>
            </defs>
            <path :d="`M 0,100 L ${usageLinePoints} L 100,100 Z`" fill="url(#usageFill)" />
            <polyline :points="usageLinePoints" fill="none" stroke="#2d7657" stroke-linecap="round" stroke-linejoin="round" stroke-width="2" vector-effect="non-scaling-stroke" />
          </svg>
        </div>
        <div class="mt-3 grid grid-cols-7 gap-1 text-center">
          <span v-for="day in usageChart" :key="day.key" class="flex flex-col text-xs text-muted-foreground">
            {{ day.label }}<b class="font-mono text-foreground">{{ day.tokens ? day.tokens.toLocaleString() : '-' }}</b>
          </span>
        </div>
      </CardContent>
    </Card>
    <Card>
      <CardContent class="pt-6">
        <div class="mb-4 flex items-center justify-between">
          <div>
            <h2 class="text-sm font-semibold">{{ t('accessKeys') }}</h2>
            <p class="text-xs text-muted-foreground">{{ t('currentAccountKeysDesc') }}</p>
          </div>
          <Button variant="link" size="sm" @click="openConsole('account')">{{ t('myAccount') }}</Button>
        </div>
        <div v-if="accountKeys.length" class="flex flex-col gap-2">
          <div v-for="key in accountKeys.slice(0, 5)" :key="key.id" class="flex items-center gap-3 rounded-md border border-border p-2">
            <code class="font-mono text-xs">{{ key.key_prefix }}...</code>
            <span class="flex-1 text-sm">{{ key.name }}</span>
            <Badge :variant="key.revoked_at ? 'destructive' : 'secondary'">{{ key.revoked_at ? t('revoked') : t('valid') }}</Badge>
          </div>
        </div>
        <Empty v-else :text="t('noApiKeysYet')" />
      </CardContent>
    </Card>
  </div>
</template>
