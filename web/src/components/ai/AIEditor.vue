<script setup lang="ts">
/**
 * AI 代码输入编辑器
 *
 * 对外暴露 v-model 接口，页面只需使用 <AIEditor v-model="code" /> 即可。
 *
 * 编辑器实现（可替换策略）：
 *   - 优先使用 monaco-editor（含语言服务 Worker，Vite 原生支持 `?worker` 导入）；
 *   - 若 Monaco 动态加载失败（如运行时环境受限），自动降级为 Element Plus textarea，
 *     页面无需任何改动 —— 组件结构保持可替换。
 */
import { nextTick, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'

// Monaco 语言服务 Worker（静态导入，Vite 原生支持 `?worker` 后缀编译为独立 chunk）
// 注意：monaco-editor@0.56 的 package.json exports 已把 `monaco-editor/xxx` 映射到 `./esm/vs/xxx`，
//       因此这里不能写 `monaco-editor/esm/vs/...`（会路径翻倍导致 Rollup 无法解析）。
import EditorWorker from 'monaco-editor/editor/editor.worker?worker'
import JsonWorker from 'monaco-editor/language/json/json.worker?worker'
import CssWorker from 'monaco-editor/language/css/css.worker?worker'
import HtmlWorker from 'monaco-editor/language/html/html.worker?worker'
import TsWorker from 'monaco-editor/language/typescript/ts.worker?worker'

const props = withDefaults(
  defineProps<{
    /** 双向绑定：当前输入内容 */
    modelValue: string
    /** Monaco 语言标识（如 plaintext / javascript / go），降级 textarea 时忽略 */
    language?: string
    /** 输入占位提示（textarea 降级时展示） */
    placeholder?: string
    /** 是否只读（AI 请求期间禁用编辑） */
    disabled?: boolean
    /** 编辑器高度（number 视为 px，或任意 CSS 长度） */
    height?: number | string
  }>(),
  {
    language: 'plaintext',
    placeholder: '',
    disabled: false,
    height: 480,
  },
)

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
}>()

// ==================== Monaco 动态加载（失败降级 textarea） ====================

type MonacoApi = typeof import('monaco-editor')
type MonacoEditor = import('monaco-editor').editor.IStandaloneCodeEditor
type MonacoDisposable = import('monaco-editor').IDisposable

/** 编辑器实现状态：loading=加载中 / monaco / textarea（降级） */
type EditorImpl = 'loading' | 'monaco' | 'textarea'

const editorImpl = ref<EditorImpl>('loading')
const containerRef = ref<HTMLDivElement>()
const monacoApi = shallowRef<MonacoApi | null>(null)
const editorInstance = shallowRef<MonacoEditor | null>(null)
const contentChangeDisposable = shallowRef<MonacoDisposable | null>(null)

/**
 * 配置 Monaco 语言服务 Worker 并加载编辑器。
 * Worker 已在模块顶部静态导入（Vite `?worker`），避免动态 import 无法被 Rollup 解析。
 */
async function loadMonaco(): Promise<MonacoApi | null> {
  try {
    // 类型声明来自 monaco-editor 自带的 monaco.d.ts（declare global）
    globalThis.MonacoEnvironment = {
      getWorker(_workerId: string, label: string): Worker {
        if (label === 'json') return new JsonWorker()
        if (label === 'css' || label === 'scss' || label === 'less') return new CssWorker()
        if (label === 'html' || label === 'handlebars' || label === 'razor') return new HtmlWorker()
        if (label === 'typescript' || label === 'javascript') return new TsWorker()
        return new EditorWorker()
      },
    }

    return await import('monaco-editor')
  } catch {
    // Monaco 加载失败 → 降级 textarea
    return null
  }
}

async function initMonaco() {
  const api = await loadMonaco()
  if (!api || !containerRef.value) {
    editorImpl.value = 'textarea'
    return
  }

  monacoApi.value = api
  editorImpl.value = 'monaco'
  await nextTick()

  const editor = api.editor.create(containerRef.value, {
    value: props.modelValue,
    language: props.language,
    theme: 'vs-dark',
    automaticLayout: true,
    minimap: { enabled: false },
    fontSize: 13,
    lineNumbers: 'on',
    scrollBeyondLastLine: false,
    tabSize: 2,
    wordWrap: 'on',
    padding: { top: 8, bottom: 8 },
    readOnly: props.disabled,
  })

  contentChangeDisposable.value = editor.onDidChangeModelContent(() => {
    emit('update:modelValue', editor.getValue())
  })

  editorInstance.value = editor
}

onMounted(() => {
  void initMonaco()
})

onBeforeUnmount(() => {
  contentChangeDisposable.value?.dispose()
  editorInstance.value?.dispose()
})

// ==================== 外部属性变化同步 ====================

watch(
  () => props.modelValue,
  (val) => {
    const editor = editorInstance.value
    if (editor && editor.getValue() !== val) {
      editor.setValue(val)
    }
  },
)

watch(
  () => props.language,
  (lang) => {
    const editor = editorInstance.value
    const api = monacoApi.value
    const model = editor?.getModel()
    if (editor && api && model) {
      api.editor.setModelLanguage(model, lang)
    }
  },
)

watch(
  () => props.disabled,
  (disabled) => {
    editorInstance.value?.updateOptions({ readOnly: disabled })
  },
)

function handleTextareaInput(value: string) {
  emit('update:modelValue', value)
}

/** 高度样式（number 视为 px） */
function heightStyle(): string {
  return typeof props.height === 'number' ? `${props.height}px` : props.height
}
</script>


<template>
  <div class="ai-editor" :style="{ height: heightStyle() }">
    <!-- Monaco 容器 -->
    <div v-if="editorImpl === 'monaco'" ref="containerRef" class="ai-editor__monaco" />
    <!-- Monaco 加载中占位 -->
    <div v-else-if="editorImpl === 'loading'" v-loading="true" class="ai-editor__loading">
      <span class="ai-editor__loading-tip">编辑器加载中…</span>
    </div>
    <!-- textarea 降级 -->
    <el-input
      v-else
      :model-value="modelValue"
      type="textarea"
      :placeholder="placeholder"
      :disabled="disabled"
      resize="vertical"
      class="ai-editor__textarea"
      @update:model-value="handleTextareaInput"
    />
  </div>
</template>

<style scoped lang="scss">
.ai-editor {
  width: 100%;
  overflow: hidden;

  &__monaco {
    width: 100%;
    height: 100%;
    border: 1px solid #303133;
    border-radius: 4px;
    overflow: hidden;
  }

  &__loading {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 100%;
    height: 100%;
    border: 1px solid var(--el-border-color);
    border-radius: 4px;
  }

  &__loading-tip {
    font-size: 13px;
    color: var(--el-text-color-secondary);
  }

  &__textarea {
    width: 100%;
    height: 100%;

    :deep(.el-textarea__inner) {
      height: 100% !important;
      font-family: Consolas, Monaco, 'Courier New', 'JetBrains Mono', monospace;
      font-size: 13px;
      line-height: 1.6;
      background-color: #1e1e1e;
      color: #d4d4d4;
      border-color: #303133;
      border-radius: 4px;
    }

    :deep(.el-textarea__inner::placeholder) {
      color: #6b7280;
    }
  }
}
</style>
