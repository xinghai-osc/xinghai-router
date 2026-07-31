<script setup lang="ts">
import { KeyRound, Search, Trash2 } from 'lucide-vue-next'
import type { SelectOption } from '~/components/ui/Select.vue'

useHead({ title: '设计系统 · 星海 Router' })

const text = ref('sk-xh-2f9c41')
const note = ref('')
const provider = ref('anthropic')
const enabled = ref(true)
const checked = ref(true)
const tab = ref('overview')
const dialogOpen = ref(false)

const providers: SelectOption[] = [
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'kimi', label: 'Kimi' },
  { value: 'ollama', label: 'Ollama' },
]

const rows = [
  { model: 'claude-sonnet-4', tokens: 1284302, cost: '¥12.84', state: 'success' as const, label: '正常' },
  { model: 'kimi-k2', tokens: 402118, cost: '¥3.21', state: 'warn' as const, label: '限流' },
  { model: 'gpt-4o-mini', tokens: 88240, cost: '¥0.42', state: 'danger' as const, label: '失败' },
]
</script>

<template>
  <div class="shell max-w-4xl space-y-10 py-14">
    <header class="space-y-3">
      <UiBadge tone="clay">P0</UiBadge>
      <h1 class="display text-5xl text-ink">设计系统</h1>
      <p class="max-w-xl text-muted">
        暖纸底色、单一陶土强调色、边框优先于阴影。衬线仅用于英文大字，中文与界面统一无衬线。
      </p>
    </header>

    <DesignSection title="主题" description="预设 × 明暗两个独立轴。切换后本页所有组件实时改变，无需刷新。">
      <DesignThemeShowcase />
    </DesignSection>

    <DesignSection title="色彩" description="全站只有一个强调色；语义色只出现在徽章与告警。三套主题共用同一组语义 token。">
      <DesignPalette />
    </DesignSection>

    <DesignSection title="字体" description="Instrument Serif 承担英文展示字，Inter 承担界面与中文，JetBrains Mono 承担密钥与数字。">
      <div class="space-y-4 rounded-card border border-line bg-surface p-6">
        <p class="display text-5xl text-ink">Unified LLM Gateway</p>
        <p class="text-2xl font-semibold text-ink">统一的大模型网关</p>
        <p class="text-muted">正文 15px / 行高 1.65，留出呼吸感，长段落更易读。</p>
        <p class="numeric text-sm text-ink">sk-xh-9f21c8ad · 1,284,302 tokens · ¥12.84</p>
      </div>
    </DesignSection>

    <DesignSection title="按钮" description="三档层级加一个危险态；同一行内只允许出现一个实心按钮。">
      <div class="flex flex-wrap items-center gap-3">
        <UiButton>主要操作</UiButton>
        <UiButton variant="secondary">次要操作</UiButton>
        <UiButton variant="ghost">幽灵</UiButton>
        <UiButton variant="danger">删除</UiButton>
        <UiButton variant="link">了解更多</UiButton>
        <UiButton loading>处理中</UiButton>
        <UiButton disabled>不可用</UiButton>
      </div>
      <div class="flex flex-wrap items-center gap-3">
        <UiButton size="sm">小</UiButton>
        <UiButton size="md">中</UiButton>
        <UiButton size="lg">大</UiButton>
        <UiButton size="icon" variant="secondary" aria-label="删除">
          <Trash2 class="size-4" />
        </UiButton>
      </div>
    </DesignSection>

    <DesignSection title="表单" description="标签、提示、错误三段式；焦点环统一为陶土色。">
      <div class="grid gap-5 rounded-card border border-line bg-surface p-6 sm:grid-cols-2">
        <UiField label="密钥名称" hint="仅自己可见，便于区分用途" required>
          <UiInput v-model="text" mono placeholder="sk-…">
            <template #leading>
              <KeyRound class="size-4" />
            </template>
          </UiInput>
        </UiField>

        <UiField label="供应商">
          <UiSelect v-model="provider" :options="providers" />
        </UiField>

        <UiField label="搜索" error="至少输入 2 个字符">
          <UiInput v-model="note" invalid placeholder="按模型名筛选">
            <template #leading>
              <Search class="size-4" />
            </template>
          </UiInput>
        </UiField>

        <UiField label="备注">
          <UiTextarea v-model="note" :rows="3" placeholder="可选" />
        </UiField>

        <div class="flex items-center gap-6 sm:col-span-2">
          <label class="flex items-center gap-2.5 text-sm text-ink">
            <UiSwitch v-model="enabled" label="启用渠道" />
            启用渠道
          </label>
          <UiCheckbox v-model="checked" label="加入排行榜" />
        </div>
      </div>
    </DesignSection>

    <DesignSection title="徽章与提示">
      <div class="flex flex-wrap items-center gap-2">
        <UiBadge dot tone="success">运行中</UiBadge>
        <UiBadge dot tone="warn">降级</UiBadge>
        <UiBadge dot tone="danger">已停用</UiBadge>
        <UiBadge tone="clay">Pro</UiBadge>
        <UiBadge tone="neutral">默认分组</UiBadge>
        <UiBadge tone="outline">v1</UiBadge>
        <UiTooltip content="过去 24 小时按 token 计">
          <UiBadge tone="neutral">悬停我</UiBadge>
        </UiTooltip>
      </div>
      <div class="grid gap-3 sm:grid-cols-2">
        <UiAlert tone="info" title="计费说明">流式响应当前不计费。</UiAlert>
        <UiAlert tone="success">渠道已保存。</UiAlert>
        <UiAlert tone="warn" title="余额偏低">当前余额不足以覆盖今日均值。</UiAlert>
        <UiAlert tone="danger" title="上游返回 429" dismissible>已自动切换到备用渠道。</UiAlert>
      </div>
    </DesignSection>

    <DesignSection title="表格" description="无竖线，1px 行分隔，数字列右对齐并使用等宽字。">
      <UiTable>
        <thead>
          <tr>
            <th>模型</th>
            <th class="num">Tokens</th>
            <th class="num">费用</th>
            <th>状态</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.model">
            <td class="font-mono text-[13px]">{{ row.model }}</td>
            <td class="num">{{ row.tokens.toLocaleString() }}</td>
            <td class="num">{{ row.cost }}</td>
            <td><UiBadge dot :tone="row.state">{{ row.label }}</UiBadge></td>
          </tr>
        </tbody>
      </UiTable>
    </DesignSection>

    <DesignSection title="卡片 / 标签页 / 空态">
      <UiTabs v-model="tab" :items="[{ value: 'overview', label: '概览' }, { value: 'keys', label: '密钥', count: 3 }]" />

      <div class="grid gap-4 sm:grid-cols-2">
        <UiCard title="本月用量" description="截至今日">
          <template #actions>
            <UiButton variant="secondary" size="sm">导出</UiButton>
          </template>
          <p class="numeric text-3xl text-ink">1,284,302</p>
          <p class="mt-1 text-[13px] text-muted">较上月 +18%</p>
        </UiCard>

        <UiCard flush>
          <UiEmptyState title="还没有 API 密钥" description="创建第一个密钥即可开始调用网关。">
            <UiButton size="sm" @click="dialogOpen = true">创建密钥</UiButton>
          </UiEmptyState>
        </UiCard>
      </div>

      <UiCard title="加载骨架">
        <UiSkeleton :rows="3" />
      </UiCard>
    </DesignSection>

    <DesignSection title="浮层">
      <div class="flex flex-wrap gap-3">
        <UiButton variant="secondary" @click="dialogOpen = true">打开对话框</UiButton>
        <UiDropdownMenu>
          <template #trigger>
            <UiButton variant="secondary">下拉菜单</UiButton>
          </template>
          <UiDropdownItem as="label">操作</UiDropdownItem>
          <UiDropdownItem>编辑</UiDropdownItem>
          <UiDropdownItem>复制 ID</UiDropdownItem>
          <UiDropdownItem as="separator" />
          <UiDropdownItem danger>吊销</UiDropdownItem>
        </UiDropdownMenu>
      </div>

      <UiDialog v-model:open="dialogOpen" title="创建 API 密钥" description="密钥只会完整显示一次，请立即保存。">
        <UiField label="名称" required>
          <UiInput v-model="text" placeholder="例如：生产环境" />
        </UiField>
        <template #footer>
          <UiButton variant="ghost" @click="dialogOpen = false">取消</UiButton>
          <UiButton @click="dialogOpen = false">创建</UiButton>
        </template>
      </UiDialog>
    </DesignSection>
  </div>
</template>
