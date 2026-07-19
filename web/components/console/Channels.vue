<script setup lang="ts">
import { Plus } from 'lucide-vue-next'
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'

const store = useConsoleStore()
const { t, channels, can, showChannel, toggleChannel, editChannel } = store
</script>

<template>
  <section class="toolbar">
    <div>
      <h2>{{ t('upstreamChannels') }}</h2>
      <p>{{ t('channelsDesc') }}</p>
    </div>
    <button v-if="can('channels.manage')" class="button primary" @click="showChannel = true"><Plus :size="16" />{{ t('addChannel') }}</button>
  </section>
  <section class="panel table-panel channel-table-panel">
    <table>
      <thead>
        <tr>
          <th>Status</th>
          <th>Channel name</th>
          <th>Upstream URL</th>
          <th>{{ t('modelLabel') }}</th>
          <th>{{ t('priorityLabel') }}</th>
          <th v-if="can('channels.manage')">{{ t('enableChannelLabel') }}</th>
          <th v-if="can('channels.manage')"></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="channel in channels" :key="channel.id">
          <td>
            <span :class="['state', channel.enabled ? 'good' : 'bad']">
              <i :class="['status-dot', { off: !channel.enabled }]"></i>
              {{ channel.enabled ? t('enabled') : channel.auto_disabled ? t('autoDisabled') : t('disabled') }}
            </span>
            <small v-if="channel.auto_disabled && channel.disabled_reason" class="muted" :title="channel.disabled_reason">{{ channel.disabled_reason }}</small>
          </td>
          <td><b>{{ channel.name }}</b><small>{{ channel.provider }}</small></td>
          <td><code :title="channel.base_url">{{ channel.base_url }}</code></td>
          <td><div class="model-tags"><span v-for="model in channel.models" :key="model">{{ model }}</span></div></td>
          <td>{{ channel.priority }}</td>
          <td v-if="can('channels.manage')">
            <button class="toggle" :class="{ on: channel.enabled }" :aria-label="channel.enabled ? t('disableChannelLabel') : t('enableChannelLabel')" @click="toggleChannel(channel)"><i></i></button>
          </td>
          <td v-if="can('channels.manage')">
            <button class="text-button" @click="editChannel(channel)">{{ t('edit') }}</button>
          </td>
        </tr>
      </tbody>
    </table>
    <Empty v-if="!channels.length" :text="t('addOpenAICompatibleUpstream')" />
  </section>
</template>
