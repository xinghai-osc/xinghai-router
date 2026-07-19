<script setup lang="ts">
import { useConsoleStore } from '~/composables/useConsoleStore'

const store = useConsoleStore()
const { t, account, ownGroups, avatarUrlInput, avatarInput, leaderboardPrefs, saveLeaderboardPrefs, chooseAvatar, removeAvatar, saveAvatarUrl } = store
</script>

<template>
  <section class="profile-layout">
    <section class="panel account-card">
      <div class="profile-avatar">{{ account?.name?.slice(0, 1) || '?' }}</div>
      <div>
        <span class="overview-kicker">{{ t('accountProfile') }}</span>
        <h2>{{ account?.name }}</h2>
        <p>{{ account?.email }}</p>
      </div>
    </section>
    <section class="panel profile-details">
      <div>
        <span>{{ t('accountRole') }}</span>
        <b>{{ account?.role }}</b>
      </div>
      <div>
        <span>{{ t('userGroups') }}</span>
        <div v-if="ownGroups.length" class="profile-group-tags">
          <span v-for="group in ownGroups" :key="group">{{ group }}</span>
        </div>
        <b v-else>{{ t('notInAnyGroup') }}</b>
      </div>
      <div>
        <span>{{ t('accountId') }}</span>
        <code>{{ account?.id }}</code>
      </div>
      <div>
        <span>{{ t('permissionScope') }}</span>
        <b>{{ account?.role === 'admin' ? t('allPermissions') : account?.permissions.join(', ') || t('regularAccount') }}</b>
      </div>
    </section>
    <section class="panel avatar-settings">
      <div>
        <h2>{{ t('avatarSection') }}</h2>
        <p>{{ t('avatarDesc') }}</p>
      </div>
      <div class="avatar-current">
        <img v-if="account?.avatar_url" class="profile-avatar" :src="account.avatar_url" :alt="t('avatarSection')" />
        <span v-else class="profile-avatar">{{ account?.name?.slice(0, 1) || '?' }}</span>
      </div>
      <div class="avatar-actions">
        <form class="avatar-url-form" @submit.prevent="saveAvatarUrl">
          <input v-model="avatarUrlInput" type="url" :placeholder="t('avatarUrlPlaceholder')" />
          <button class="button ghost" type="submit">{{ t('setAvatar') }}</button>
        </form>
        <div class="avatar-upload-row">
          <input ref="avatarInput" class="visually-hidden" type="file" accept="image/png,image/jpeg,image/gif,image/webp" @change="chooseAvatar" />
          <button class="button ghost" type="button" @click="avatarInput?.click()">{{ t('uploadAvatar') }}</button>
          <button v-if="account?.avatar_url" class="text-button danger" type="button" @click="removeAvatar">{{ t('remove') }}</button>
        </div>
      </div>
    </section>
    <section class="panel leaderboard-settings">
      <div>
        <h2>{{ t('leaderboardSettings') }}</h2>
        <p>{{ t('leaderboardSettingsDesc') }}</p>
      </div>
      <div class="setting-row">
        <div>
          <b>{{ t('leaderboardOptIn') }}</b>
          <small>{{ t('leaderboardOptInDesc') }}</small>
        </div>
        <button class="toggle" :class="{ on: leaderboardPrefs.opt_in }" type="button" :aria-label="t('leaderboardOptIn')" @click="leaderboardPrefs.opt_in = !leaderboardPrefs.opt_in; saveLeaderboardPrefs()"><i></i></button>
      </div>
      <div class="setting-row">
        <div>
          <b>{{ t('leaderboardMaskName') }}</b>
          <small>{{ t('leaderboardMaskNameDesc') }}</small>
        </div>
        <button class="toggle" :class="{ on: leaderboardPrefs.mask_name }" type="button" :aria-label="t('leaderboardMaskName')" @click="leaderboardPrefs.mask_name = !leaderboardPrefs.mask_name; saveLeaderboardPrefs()"><i></i></button>
      </div>
    </section>
  </section>
</template>
