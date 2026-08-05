<script setup lang="ts">
/**
 * AI 输出结果面板
 *
 * - Markdown 渲染：markdown-it（支持代码块 / 标题 / 列表，html: false 防 XSS）
 * - 复制按钮：优先 navigator.clipboard（非 HTTPS 自动降级 execCommand）
 * - 状态：加载中（AI 请求中）/ 空状态 / 渲染内容
 */
import { computed, ref } from 'vue'
import MarkdownIt from 'markdown-it'
import { ElMessage } from 'element-plus'
import { CopyDocument } from '@element-plus/icons-vue'

const props = withDefaults(
  defineProps<{
    /** AI 返回的 Markdown 文本 */
    content: string
    /** 是否正在请求 AI（展示 loading 态） */
    loading?: boolean
    /** 空状态占位文案 */
    placeholder?: string
    /** 面板高度（number 视为 px，或任意 CSS 长度） */
    height?: number | string
  }>(),
  {
    loading: false,
    placeholder: 'AI 输出结果将显示在这里',
    height: 480,
  },
)

const md = new MarkdownIt({
  html: false, // 禁止内嵌 HTML，避免 XSS
  linkify: true, // 自动识别链接
  breaks: true, // 换行渲染为 <br>
})

const renderedHtml = computed(() => (props.content ? md.render(props.content) : ''))

const copyLoading = ref(false)

/** 复制文本：Clipboard API 不可用时降级为 execCommand */
async function writeToClipboard(text: string): Promise<void> {
  if (navigator.clipboard && window.isSecureContext) {
    await navigator.clipboard.writeText(text)
    return
  }
  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.select()
  try {
    if (!document.execCommand('copy')) {
      throw new Error('execCommand copy 失败')
    }
  } finally {
    document.body.removeChild(textarea)
  }
}

/** 复制 AI 输出到剪贴板 */
async function handleCopy() {
  if (!props.content) return
  copyLoading.value = true
  try {
    await writeToClipboard(props.content)
    ElMessage.success('已复制到剪贴板')
  } catch {
    ElMessage.error('复制失败，请手动复制')
  } finally {
    copyLoading.value = false
  }
}

/** 高度样式（number 视为 px） */
function heightStyle(): string {
  return typeof props.height === 'number' ? `${props.height}px` : props.height
}
</script>

<template>
  <div class="ai-result" :style="{ height: heightStyle() }">
    <!-- 复制按钮（有输出时显示） -->
    <el-button
      v-if="content"
      class="ai-result__copy"
      type="primary"
      plain
      size="small"
      :icon="CopyDocument"
      :loading="copyLoading"
      :disabled="loading"
      @click="handleCopy"
    >
      复制结果
    </el-button>

    <!-- 已有内容：渲染 Markdown；加载中叠加遮罩 -->
    <div v-if="content" class="ai-result__scroll">
      <div class="ai-result__markdown" v-html="renderedHtml" />
      <div v-if="loading" v-loading="true" class="ai-result__mask" />
    </div>

    <!-- 无内容 + 加载中 -->
    <div v-else-if="loading" v-loading="true" class="ai-result__state">
      <span class="ai-result__state-tip">AI 正在思考中，请稍候…</span>
    </div>

    <!-- 空状态 -->
    <div v-else class="ai-result__state">
      <el-empty :description="placeholder" :image-size="80" />
    </div>
  </div>
</template>

<style scoped lang="scss">
.ai-result {
  position: relative;
  width: 100%;
  overflow: hidden;
  border: 1px solid var(--el-border-color);
  border-radius: 4px;
  background-color: #ffffff;

  &__copy {
    position: absolute;
    top: 8px;
    right: 8px;
    z-index: 10;
  }

  &__scroll {
    height: 100%;
    overflow: auto;
    padding: 16px;
    position: relative;
  }

  &__mask {
    position: absolute;
    inset: 0;
    z-index: 5;
  }

  &__state {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
  }

  &__state-tip {
    font-size: 13px;
    color: var(--el-text-color-secondary);
  }

  // ============ Markdown 渲染样式 ============
  &__markdown {
    font-size: 14px;
    line-height: 1.7;
    color: #303133;
    word-break: break-word;

    :deep(h1),
    :deep(h2),
    :deep(h3),
    :deep(h4),
    :deep(h5),
    :deep(h6) {
      margin: 18px 0 10px;
      font-weight: 600;
      line-height: 1.4;
      color: #1f2329;
    }

    :deep(h1) {
      font-size: 20px;
      padding-bottom: 8px;
      border-bottom: 1px solid var(--el-border-color-lighter);
    }

    :deep(h2) {
      font-size: 17px;
    }

    :deep(h3) {
      font-size: 15px;
    }

    :deep(p) {
      margin: 8px 0;
    }

    :deep(ul),
    :deep(ol) {
      margin: 8px 0;
      padding-left: 24px;
    }

    :deep(li) {
      margin: 4px 0;
    }

    :deep(blockquote) {
      margin: 12px 0;
      padding: 8px 12px;
      color: #606266;
      background-color: #f5f7fa;
      border-left: 4px solid #c0c4cc;
      border-radius: 0 4px 4px 0;
    }

    :deep(a) {
      color: #2563eb;
      text-decoration: none;

      &:hover {
        text-decoration: underline;
      }
    }

    :deep(code) {
      padding: 2px 6px;
      font-family: Consolas, Monaco, 'Courier New', monospace;
      font-size: 12.5px;
      color: #c7254e;
      background-color: #f9f2f4;
      border-radius: 3px;
    }

    :deep(pre) {
      position: relative;
      margin: 12px 0;
      padding: 14px 16px;
      overflow: auto;
      background-color: #1e1e1e;
      border-radius: 6px;

      code {
        display: block;
        padding: 0;
        font-size: 13px;
        line-height: 1.6;
        color: #d4d4d4;
        background-color: transparent;
      }
    }

    :deep(hr) {
      margin: 16px 0;
      border: none;
      border-top: 1px solid var(--el-border-color-lighter);
    }

    :deep(table) {
      margin: 12px 0;
      border-collapse: collapse;
      width: 100%;

      th,
      td {
        padding: 8px 12px;
        border: 1px solid var(--el-border-color-lighter);
        text-align: left;
      }

      th {
        background-color: #f5f7fa;
        font-weight: 600;
      }
    }
  }
}
</style>

