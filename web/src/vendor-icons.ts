import alibaba from '@lobehub/icons-static-svg/icons/alibaba.svg?url&no-inline'
import anthropic from '@lobehub/icons-static-svg/icons/anthropic.svg?url&no-inline'
import bytedance from '@lobehub/icons-static-svg/icons/bytedance.svg?url&no-inline'
import deepseek from '@lobehub/icons-static-svg/icons/deepseek.svg?url&no-inline'
import google from '@lobehub/icons-static-svg/icons/google.svg?url&no-inline'
import meta from '@lobehub/icons-static-svg/icons/meta.svg?url&no-inline'
import minimax from '@lobehub/icons-static-svg/icons/minimax.svg?url&no-inline'
import mistral from '@lobehub/icons-static-svg/icons/mistral.svg?url&no-inline'
import moonshot from '@lobehub/icons-static-svg/icons/moonshot.svg?url&no-inline'
import openai from '@lobehub/icons-static-svg/icons/openai.svg?url&no-inline'
import stepfun from '@lobehub/icons-static-svg/icons/stepfun.svg?url&no-inline'
import xai from '@lobehub/icons-static-svg/icons/xai.svg?url&no-inline'
import zhipu from '@lobehub/icons-static-svg/icons/zhipu.svg?url&no-inline'

const VENDOR_ICONS = new Map<string, string>([
  ['openai', openai],
  ['anthropic', anthropic],
  ['claude', anthropic],
  ['google', google],
  ['gemini', google],
  ['deepseek', deepseek],
  ['深度求索', deepseek],
  ['alibaba', alibaba],
  ['aliyun', alibaba],
  ['qwen', alibaba],
  ['通义千问', alibaba],
  ['通義千問', alibaba],
  ['阿里巴巴', alibaba],
  ['zhipu', zhipu],
  ['zhipuai', zhipu],
  ['zai', zhipu],
  ['glm', zhipu],
  ['智谱', zhipu],
  ['智譜', zhipu],
  ['mistral', mistral],
  ['mistralai', mistral],
  ['xai', xai],
  ['grok', xai],
  ['meta', meta],
  ['llama', meta],
  ['moonshot', moonshot],
  ['moonshotai', moonshot],
  ['kimi', moonshot],
  ['月之暗面', moonshot],
  ['minimax', minimax],
  ['stepfun', stepfun],
  ['阶跃星辰', stepfun],
  ['階躍星辰', stepfun],
  ['bytedance', bytedance],
  ['doubao', bytedance],
  ['字节跳动', bytedance],
  ['字節跳動', bytedance],
  ['豆包', bytedance],
])

function normalizeVendor(value: string): string {
  return value.normalize('NFKC').trim().toLowerCase().replace(/[\s._-]+/g, '')
}

export function vendorIcon(name: string, slug?: string): string | undefined {
  return VENDOR_ICONS.get(normalizeVendor(slug ?? '')) ?? VENDOR_ICONS.get(normalizeVendor(name))
}
