<script setup lang="ts">
/**
 * AI 模式选择器
 *
 * 四个模式（与后端 4 个 AI 接口一一对应）：
 *   - generate  代码生成 → POST /api/ai/generate（请求体：{ prompt }）
 *   - explain   代码解释 → POST /api/ai/explain（请求体：{ code }）
 *   - fix       代码修复 → POST /api/ai/fix（请求体：{ code }）
 *   - optimize  代码优化 → POST /api/ai/optimize（请求体：{ code }）
 */
import { MagicStick, Reading, Tools, TrendCharts } from '@element-plus/icons-vue'
import type { AIMode } from '@/types/ai'

const props = defineProps<{
  modelValue: AIMode
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: AIMode): void
}>()

interface ModeOption {
  value: AIMode
  label: string
  desc: string
  icon: typeof MagicStick
}

const modes: ModeOption[] = [
  { value: 'generate', label: '代码生成', desc: '根据需求描述生成代码', icon: MagicStick },
  { value: 'explain', label: '代码解释', desc: '解释代码的含义与逻辑', icon: Reading },
  { value: 'fix', label: '代码修复', desc: '修复代码中的错误', icon: Tools },
  { value: 'optimize', label: '代码优化', desc: '让代码更简洁高效', icon: TrendCharts },
]

function handleSelect(value: AIMode) {
  emit('update:modelValue', value)
}
</script>

<template>
  <div class="ai-mode-selector" role="radiogroup" aria-label="AI 模式选择">
    <button
      v-for="mode in modes"
      :key="mode.value"
      type="button"
      class="ai-mode-selector__item"
      :class="{ 'is-active': mode.value === props.modelValue }"
      role="radio"
      :aria-checked="mode.value === props.modelValue"
      @click="handleSelect(mode.value)"
    >
      <el-icon :size="18" class="ai-mode-selector__icon"><component :is="mode.icon" /></el-icon>
      <span class="ai-mode-selector__label">{{ mode.label }}</span>
      <span class="ai-mode-selector__desc">{{ mode.desc }}</span>
    </button>
  </div>
</template>

<style scoped lang="scss">
.ai-mode-selector {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;

  &__item {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 148px;
    padding: 10px 14px;
    cursor: pointer;
    background-color: #ffffff;
    border: 1px solid var(--el-border-color);
    border-radius: 6px;
    transition: all 0.2s ease;
    text-align: left;

    &:hover {
      border-color: var(--el-color-primary-light-5);
      box-shadow: 0 2px 8px rgba(37, 99, 235, 0.1);
    }

    &.is-active {
      border-color: var(--el-color-primary);
      background-color: var(--el-color-primary-light-9);
      box-shadow: 0 2px 8px rgba(37, 99, 235, 0.15);

      .ai-mode-selector__icon {
        color: var(--el-color-primary);
      }

      .ai-mode-selector__label {
        color: var(--el-color-primary);
      }
    }
  }

  &__icon {
    color: var(--el-text-color-secondary);
    transition: color 0.2s ease;
  }

  &__label {
    font-size: 14px;
    font-weight: 600;
    color: var(--el-text-color-primary);
    transition: color 0.2s ease;
  }

  &__desc {
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }
}
</style>
