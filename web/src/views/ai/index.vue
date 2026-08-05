<script setup lang="ts">
/**
 * AI 助手页面（AI Coding Assistant）
 *
 * 页面职责：只负责「组合」三个 AI 组件 + 按模式分发请求。
 *   - AIModeSelector：顶部 AI 模式选择（Generate / Explain / Fix / Optimize）
 *   - AIEditor：左栏代码输入区域（Monaco，失败自动降级 textarea）
 *   - AIResult：右栏 AI 输出区域（Markdown 渲染 + 复制结果）
 *
 * 后端接口（见 go-admin/docs/API.md 第 3.8~3.11 节，均需 JWT）：
 *   - POST /api/ai/generate/stream  生成模式 → SSE 流式输出（请求体：{ prompt }）
 *   - POST /api/ai/explain   请求体：{ code }
 *   - POST /api/ai/fix       请求体：{ code }
 *   - POST /api/ai/optimize  请求体：{ code }
 *
 * 生成模式使用 SSE 流式（fetch + ReadableStream，见 src/api/ai.ts generateStream）：
 * 每收到一个 content 帧立即追加到 result，实现「边生成边显示」；
 * 其余三个模式仍走 axios（src/utils/request.ts），一次性返回 Markdown 文本，
 * 错误提示统一由拦截器处理。
 */
import { computed, onBeforeUnmount, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Delete, Promotion } from '@element-plus/icons-vue'

import { explainCode, fixCode, generateStream, optimizeCode } from '@/api/ai'
import AIModeSelector from '@/components/ai/AIModeSelector.vue'
import AIEditor from '@/components/ai/AIEditor.vue'
import AIResult from '@/components/ai/AIResult.vue'
import type { AIMode } from '@/types/ai'

// ==================== 状态 ====================

const mode = ref<AIMode>('generate')
const inputCode = ref('')
const result = ref('')
const loading = ref(false)

/** 当前流式请求的 AbortController（切换模式 / 离开页面时中断） */
let streamController: AbortController | null = null

// ==================== 模式相关配置 ====================

/** 各模式输入区占位提示 */
const MODE_PLACEHOLDER: Record<AIMode, string> = {
  generate: '请输入你的代码需求描述，例如：用 Go 实现一个 JWT 登录中间件，支持过期刷新',
  explain: '粘贴需要解释的代码，AI 将分析其含义、逻辑与关键点…',
  fix: '粘贴需要修复的代码，AI 将定位问题并给出修复后的版本…',
  optimize: '粘贴需要优化的代码，AI 将给出更简洁高效的版本…',
}

/** 各模式执行按钮文案 */
const MODE_EXECUTE_LABEL: Record<AIMode, string> = {
  generate: '生成代码',
  explain: '解释代码',
  fix: '修复代码',
  optimize: '优化代码',
}

const placeholder = computed(() => MODE_PLACEHOLDER[mode.value])
const executeLabel = computed(() => MODE_EXECUTE_LABEL[mode.value])

/** 切换模式：中断在途流式请求并清空旧输出，避免不同模式结果混淆（保留输入内容） */
function handleModeChange(value: AIMode) {
  mode.value = value
  result.value = ''
  streamController?.abort()
  streamController = null
}

// ==================== 执行 ====================

/** 根据当前模式调用对应 AI 接口；生成模式走 SSE 流式实时输出 */
async function handleExecute() {
  const content = inputCode.value.trim()
  if (!content) {
    ElMessage.warning('请先输入代码或需求描述')
    return
  }
  if (loading.value) return

  loading.value = true
  result.value = ''

  try {
    if (mode.value === 'generate') {
      // SSE 流式生成：每收到一个 content 帧立即追加，实现「边生成边显示」
      const controller = new AbortController()
      streamController = controller
      await generateStream(
        { prompt: content },
        {
          signal: controller.signal,
          onChunk: (chunk) => {
            result.value += chunk
          },
        },
      )
    } else {
      switch (mode.value) {
        case 'explain':
          result.value = await explainCode({ code: content })
          break
        case 'fix':
          result.value = await fixCode({ code: content })
          break
        case 'optimize':
          result.value = await optimizeCode({ code: content })
          break
      }
    }
  } catch (error) {
    // 主动中断（切换模式 / 离开页面）：静默处理
    if (error instanceof DOMException && error.name === 'AbortError') return
    // 请求失败提示（HTTP 状态 / SSE error 帧 / 网络异常均已转为 Error.message）
    ElMessage.error(error instanceof Error ? error.message : '请求失败，请稍后重试')
  } finally {
    streamController = null
    loading.value = false
  }
}

/** 清空输入与输出 */
function handleClear() {
  inputCode.value = ''
  result.value = ''
}

/** 组件卸载：中断在途流式请求 */
onBeforeUnmount(() => {
  streamController?.abort()
  streamController = null
})
</script>

<template>
  <div class="ai-page">
    <!-- 顶部：标题 + AI 模式选择 -->
    <div class="ai-page__toolbar">
      <div class="ai-page__title-area">
        <h2 class="ai-page__title">AI 编程助手</h2>
        <p class="ai-page__subtitle">基于大模型的代码生成 / 解释 / 修复 / 优化</p>
      </div>
      <AIModeSelector :model-value="mode" @update:model-value="handleModeChange" />
    </div>

    <!-- 中间：左右布局（左：代码输入 / 右：AI 输出） -->
    <el-row :gutter="16" class="ai-page__workspace">
      <el-col :xs="24" :lg="12" class="ai-page__col">
        <el-card class="ai-page__panel" shadow="never">
          <template #header>
            <div class="ai-page__panel-header">
              <span>代码输入</span>
              <span class="ai-page__panel-tip">{{ mode === 'generate' ? '输入需求描述' : '粘贴代码' }}</span>
            </div>
          </template>
          <AIEditor
            v-model="inputCode"
            :placeholder="placeholder"
            :disabled="loading"
            :height="480"
          />
        </el-card>
      </el-col>

      <el-col :xs="24" :lg="12" class="ai-page__col">
        <el-card class="ai-page__panel" shadow="never">
          <template #header>
            <div class="ai-page__panel-header">
              <span>AI 输出</span>
              <span class="ai-page__panel-tip">支持 Markdown 渲染</span>
            </div>
          </template>
          <AIResult :content="result" :loading="loading" :height="480" />
        </el-card>
      </el-col>
    </el-row>

    <!-- 底部：执行按钮 -->
    <div class="ai-page__footer">
      <el-button :icon="Delete" :disabled="loading" @click="handleClear">清空</el-button>
      <el-button
        type="primary"
        size="large"
        :icon="Promotion"
        :loading="loading"
        :disabled="loading || !inputCode.trim()"
        @click="handleExecute"
      >
        {{ loading ? 'AI 处理中…' : executeLabel }}
      </el-button>
    </div>
  </div>
</template>

<style scoped lang="scss">
.ai-page {
  &__toolbar {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 16px;
    margin-bottom: 16px;
  }

  &__title {
    margin: 0;
    font-size: 20px;
    font-weight: 600;
    color: #303133;
  }

  &__subtitle {
    margin: 4px 0 0;
    font-size: 13px;
    color: #909399;
  }

  &__col {
    margin-bottom: 16px;
  }

  &__panel {
    :deep(.el-card__header) {
      padding: 12px 16px;
    }

    :deep(.el-card__body) {
      padding: 12px;
    }
  }

  &__panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 14px;
    font-weight: 600;
    color: #303133;
  }

  &__panel-tip {
    font-size: 12px;
    font-weight: 400;
    color: #909399;
  }

  &__footer {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
  }
}
</style>

