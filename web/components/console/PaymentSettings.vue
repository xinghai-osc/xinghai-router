<script setup lang="ts">
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'

const store = useConsoleStore()
const { t, busy, paymentSettings, paymentSettingsForm, paymentMethodForm, savePaymentSettings, createPaymentMethod, updatePaymentMethod, deletePaymentMethod } = store
</script>

<template>
  <section class="toolbar">
    <div>
      <h2>{{ t('paymentSettings') }}</h2>
      <p>{{ t('paymentSettingsDesc') }}</p>
    </div>
  </section>
  <form class="panel payment-settings-form" @submit.prevent="savePaymentSettings">
    <label class="payment-enabled"><input v-model="paymentSettingsForm.enabled" type="checkbox" />{{ t('enableOnlinePayment') }}</label>
    <label>{{ t('epayBaseUrl') }}<input v-model="paymentSettingsForm.base_url" type="url" required placeholder="https://pay.example.com" /></label>
    <label>{{ t('publicBaseUrl') }}<input v-model="paymentSettingsForm.public_base_url" type="url" required placeholder="https://router.example.com" /></label>
    <label>{{ t('merchantId') }}<input v-model="paymentSettingsForm.merchant_id" required /></label>
    <label>{{ t('merchantKey') }}<input v-model="paymentSettingsForm.merchant_key" type="password" :required="!paymentSettings.has_merchant_key" :placeholder="paymentSettings.has_merchant_key ? t('leaveBlankUnchanged') : t('requiredField')" autocomplete="new-password" /></label>
    <button class="button primary" :disabled="busy">{{ t('saveSettings') }}</button>
  </form>
  <form class="panel payment-method-create" @submit.prevent="createPaymentMethod">
    <div>
      <h3>{{ t('addPaymentMethod') }}</h3>
      <p>{{ t('paymentMethodCodeHint') }}</p>
    </div>
    <label>{{ t('paymentMethodCode') }}<input v-model="paymentMethodForm.code" required maxlength="50" placeholder="alipay" /></label>
    <label>{{ t('paymentMethodName') }}<input v-model="paymentMethodForm.name" required maxlength="100" placeholder="支付宝" /></label>
    <label class="method-enabled"><input v-model="paymentMethodForm.enabled" type="checkbox" />{{ t('enabled') }}</label>
    <button class="button primary" :disabled="busy">{{ t('addPaymentMethod') }}</button>
  </form>
  <section class="panel table-panel">
    <div class="panel-title">
      <div>
        <h2>{{ t('paymentMethods') }}</h2>
        <p>{{ t('paymentMethodsDesc') }}</p>
      </div>
    </div>
    <table>
      <thead>
        <tr>
          <th>{{ t('paymentMethodCode') }}</th>
          <th>{{ t('paymentMethodName') }}</th>
          <th>{{ t('accountStatus') }}</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="method in paymentSettings.methods" :key="method.id">
          <td><input v-model="method.code" maxlength="50" /></td>
          <td><input v-model="method.name" maxlength="100" /></td>
          <td><label class="method-enabled"><input v-model="method.enabled" type="checkbox" />{{ method.enabled ? t('enabled') : t('disabled') }}</label></td>
          <td>
            <button class="text-button" @click="updatePaymentMethod(method)">{{ t('saveLabel') }}</button>
            <button class="text-button danger" @click="deletePaymentMethod(method)">{{ t('remove') }}</button>
          </td>
        </tr>
      </tbody>
    </table>
    <Empty v-if="!paymentSettings.methods.length" :text="t('noPaymentMethods')" />
  </section>
</template>
