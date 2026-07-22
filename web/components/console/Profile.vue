<script setup lang="ts">
import { useConsoleStore } from '~/composables/useConsoleStore'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

const store = useConsoleStore()
const { t, account, ownGroups, avatarUrlInput, avatarInput, passwordForm, passwordMessage, leaderboardPrefs, saveLeaderboardPrefs, chooseAvatar, removeAvatar, saveAvatarUrl, changePassword } = store
</script>

<template>
  <section class="grid gap-4 lg:grid-cols-2">
    <Card>
      <CardContent class="flex items-center gap-4 pt-6">
        <div class="flex h-16 w-16 items-center justify-center rounded-full bg-primary text-xl font-bold text-primary-foreground">{{ account?.name?.slice(0, 1) || '?' }}</div>
        <div>
          <span class="text-xs uppercase tracking-wide text-muted-foreground">{{ t('accountProfile') }}</span>
          <h2 class="text-lg font-semibold">{{ account?.name }}</h2>
          <p class="text-sm text-muted-foreground">{{ account?.email }}</p>
        </div>
      </CardContent>
    </Card>
    <Card>
      <CardContent class="grid gap-4 pt-6 sm:grid-cols-2">
        <div>
          <span class="text-xs text-muted-foreground">{{ t('accountRole') }}</span>
          <div class="font-medium">{{ account?.role }}</div>
        </div>
        <div>
          <span class="text-xs text-muted-foreground">{{ t('userGroups') }}</span>
          <div v-if="ownGroups.length" class="mt-1 flex flex-wrap gap-1">
            <Badge v-for="group in ownGroups" :key="group" variant="outline">{{ group }}</Badge>
          </div>
          <div v-else class="font-medium">{{ t('notInAnyGroup') }}</div>
        </div>
        <div>
          <span class="text-xs text-muted-foreground">{{ t('accountId') }}</span>
          <div><code class="font-mono text-xs">{{ account?.id }}</code></div>
        </div>
        <div>
          <span class="text-xs text-muted-foreground">{{ t('permissionScope') }}</span>
          <div class="text-sm">{{ account?.role === 'admin' ? t('allPermissions') : account?.permissions.join(', ') || t('regularAccount') }}</div>
        </div>
      </CardContent>
    </Card>
  </section>

  <Card class="mt-4">
    <CardHeader>
      <CardTitle>{{ t('avatarSection') }}</CardTitle>
      <CardDescription>{{ t('avatarDesc') }}</CardDescription>
    </CardHeader>
    <CardContent class="flex flex-col gap-4">
      <div class="flex items-center gap-4">
        <img v-if="account?.avatar_url" class="h-16 w-16 rounded-full object-cover" :src="account.avatar_url" :alt="t('avatarSection')">
        <span v-else class="flex h-16 w-16 items-center justify-center rounded-full bg-primary text-xl font-bold text-primary-foreground">{{ account?.name?.slice(0, 1) || '?' }}</span>
      </div>
      <form class="flex items-end gap-2" @submit.prevent="saveAvatarUrl">
        <div class="flex flex-1 flex-col gap-2">
          <Label>{{ t('setAvatar') }}</Label>
          <Input v-model="avatarUrlInput" type="url" :placeholder="t('avatarUrlPlaceholder')" />
        </div>
        <Button variant="outline" type="submit">{{ t('setAvatar') }}</Button>
      </form>
      <div class="flex gap-2">
        <input ref="avatarInput" class="hidden" type="file" accept="image/png,image/jpeg,image/gif,image/webp" @change="chooseAvatar">
        <Button variant="outline" type="button" @click="avatarInput?.click()">{{ t('uploadAvatar') }}</Button>
        <Button v-if="account?.avatar_url" variant="link" class="text-destructive" type="button" @click="removeAvatar">{{ t('remove') }}</Button>
      </div>
    </CardContent>
  </Card>

  <Card class="mt-4">
    <CardHeader>
      <CardTitle>{{ t('changePasswordSection') }}</CardTitle>
      <CardDescription>{{ t('changePasswordDesc') }}</CardDescription>
    </CardHeader>
    <CardContent>
      <form class="grid max-w-md gap-3" @submit.prevent="changePassword">
        <div class="grid gap-2">
          <Label>{{ t('currentPassword') }}</Label>
          <Input v-model="passwordForm.current_password" type="password" autocomplete="current-password" required minlength="8" />
        </div>
        <div class="grid gap-2">
          <Label>{{ t('newPassword') }}</Label>
          <Input v-model="passwordForm.new_password" type="password" autocomplete="new-password" required minlength="8" :placeholder="t('passwordMinLength')" />
        </div>
        <div class="grid gap-2">
          <Label>{{ t('confirmNewPassword') }}</Label>
          <Input v-model="passwordForm.confirm_password" type="password" autocomplete="new-password" required minlength="8" />
        </div>
        <p v-if="passwordMessage" class="text-sm text-muted-foreground">{{ passwordMessage }}</p>
        <div>
          <Button type="submit">{{ t('changePassword') }}</Button>
        </div>
      </form>
    </CardContent>
  </Card>

  <Card class="mt-4">
    <CardHeader>
      <CardTitle>{{ t('leaderboardSettings') }}</CardTitle>
      <CardDescription>{{ t('leaderboardSettingsDesc') }}</CardDescription>
    </CardHeader>
    <CardContent class="flex flex-col gap-4">
      <div class="flex items-center justify-between gap-4">
        <div class="flex flex-col">
          <span class="font-medium">{{ t('leaderboardOptIn') }}</span>
          <span class="text-xs text-muted-foreground">{{ t('leaderboardOptInDesc') }}</span>
        </div>
        <Switch :model-value="leaderboardPrefs.opt_in" :aria-label="t('leaderboardOptIn')" @update:model-value="() => { leaderboardPrefs.opt_in = !leaderboardPrefs.opt_in; saveLeaderboardPrefs() }" />
      </div>
      <div class="flex items-center justify-between gap-4">
        <div class="flex flex-col">
          <span class="font-medium">{{ t('leaderboardMaskName') }}</span>
          <span class="text-xs text-muted-foreground">{{ t('leaderboardMaskNameDesc') }}</span>
        </div>
        <Switch :model-value="leaderboardPrefs.mask_name" :aria-label="t('leaderboardMaskName')" @update:model-value="() => { leaderboardPrefs.mask_name = !leaderboardPrefs.mask_name; saveLeaderboardPrefs() }" />
      </div>
    </CardContent>
  </Card>
</template>
