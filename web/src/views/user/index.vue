<script setup lang="ts">
/**
 * 用户管理页面
 *
 * 后端接口（见 go-admin/docs/API.md）：
 *   - GET    /api/user?page=&pageSize=   用户列表（服务端分页）→ { list, total }
 *   - PUT    /api/user/:id               编辑用户（dto.EditUserReq：username / status 可选）
 *   - DELETE /api/user/:id               删除用户
 *   - PATCH  /api/user/:id/status        切换用户状态（status：1=正常 / 2=禁用）
 *
 * 注意：GET /api/user 仅支持 page / pageSize，后端没有 keyword 关键词搜索参数，
 *       因此搜索框仅保留并提示，不会发起 keyword 请求。
 */
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
import { Delete, Edit, Refresh, Search } from '@element-plus/icons-vue'

import { deleteUser, editUser, getUserList, switchUserStatus } from '@/api/user'
import { useUserStore } from '@/stores/modules/user'
import type { UserListItem } from '@/types/user'

const userStore = useUserStore()

/** 当前登录用户 ID（用于禁止对自身执行删除 / 禁用） */
const currentUserId = computed(() => userStore.userInfo?.id)

// ==================== 列表（服务端分页） ====================

const loading = ref(false)
const userList = ref<UserListItem[]>([])
const total = ref(0)
const query = reactive({ page: 1, pageSize: 10 })

/** 拉取用户列表：GET /api/user?page=&pageSize= */
async function fetchUserList() {
  loading.value = true
  try {
    const res = await getUserList({ page: query.page, pageSize: query.pageSize })
    userList.value = res.list
    total.value = res.total
  } catch {
    // 失败提示已由 request.ts 响应拦截器统一处理
    userList.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

// ==================== 搜索（后端不支持 keyword，仅提示） ====================

const keyword = ref('')

/** 搜索：当前接口不支持关键词搜索；无输入时刷新第一页 */
function handleSearch() {
  if (keyword.value.trim()) {
    ElMessage.info('当前接口不支持关键词搜索，仅支持分页浏览')
    return
  }
  query.page = 1
  fetchUserList()
}

/** 重置搜索条件并刷新第一页 */
function handleReset() {
  keyword.value = ''
  query.page = 1
  fetchUserList()
}

// ==================== 分页 ====================

function handleSizeChange(size: number) {
  query.pageSize = size
  query.page = 1
  fetchUserList()
}

function handleCurrentChange(page: number) {
  query.page = page
  fetchUserList()
}

// ==================== 编辑 ====================

interface EditForm {
  id: number
  username: string
  status: number
}

const editVisible = ref(false)
const submitLoading = ref(false)
const editFormRef = ref<FormInstance>()
const editForm = reactive<EditForm>({ id: 0, username: '', status: 1 })

const editRules: FormRules<EditForm> = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 2, max: 20, message: '用户名长度为 2~20 位', trigger: 'blur' },
  ],
  status: [{ required: true, message: '请选择状态', trigger: 'change' }],
}

function handleEdit(row: UserListItem) {
  editForm.id = row.id
  editForm.username = row.username
  editForm.status = row.status
  editVisible.value = true
}

async function handleSubmitEdit() {
  if (!editFormRef.value) return
  const valid = await editFormRef.value.validate().catch(() => false)
  if (!valid) return

  // 不允许通过编辑将当前登录账号置为禁用（避免自我锁定）
  if (editForm.id === currentUserId.value && editForm.status === 2) {
    ElMessage.warning('不能禁用当前登录账号')
    return
  }

  submitLoading.value = true
  try {
    // 请求体字段与 dto.EditUserReq 一致：{ username, status }
    await editUser(editForm.id, { username: editForm.username, status: editForm.status })
    ElMessage.success('更新成功')
    editVisible.value = false
    await fetchUserList()
  } catch {
    // 失败提示已由 request.ts 响应拦截器统一处理
  } finally {
    submitLoading.value = false
  }
}

// ==================== 删除 ====================

async function handleDelete(row: UserListItem) {
  if (row.id === currentUserId.value) {
    ElMessage.warning('不能删除当前登录账号')
    return
  }

  try {
    await ElMessageBox.confirm(`确定要删除用户「${row.username}」吗？删除后不可恢复。`, '删除确认', {
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch {
    return
  }

  await deleteUser(row.id)
  ElMessage.success('删除成功')
  // 当前页删空且不在第一页时回退一页
  if (userList.value.length === 1 && query.page > 1) {
    query.page -= 1
  }
  await fetchUserList()
}

// ==================== 状态切换 ====================

async function handleToggleStatus(row: UserListItem) {
  const targetStatus = row.status === 1 ? 2 : 1
  if (targetStatus === 2 && row.id === currentUserId.value) {
    ElMessage.warning('不能禁用当前登录账号')
    return
  }
  const action = targetStatus === 2 ? '禁用' : '启用'
  try {
    await ElMessageBox.confirm(`确定要${action}用户「${row.username}」吗？`, '状态切换', {
      confirmButtonText: action,
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch {
    return
  }

  // 请求体字段与 dto.SwitchStatusReq 一致：{ status }
  await switchUserStatus(row.id, { status: targetStatus })
  ElMessage.success('状态更新成功')
  await fetchUserList()
}

// ==================== 工具 ====================

/** 后端时间字段为 RFC3339 字符串（如 2026-08-03T10:00:00+08:00），格式化为本地时间 */
function formatTime(value?: string): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

onMounted(() => {
  // 确保能拿到当前用户 ID（Navbar 未拉取成功时兜底）
  if (!userStore.userInfo) {
    userStore.getUserInfo().catch(() => {})
  }
  fetchUserList()
})
</script>

<template>
  <div class="page-container">
    <el-card shadow="never">
      <!-- 搜索区域：后端不支持 keyword，仅保留输入框并提示 -->
      <el-form inline class="search-bar" @submit.prevent>
        <el-form-item label="用户名">
          <el-input
            v-model="keyword"
            placeholder="当前接口不支持关键词搜索"
            clearable
            style="width: 240px"
            @keyup.enter="handleSearch"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" @click="handleSearch">搜索</el-button>
          <el-button :icon="Refresh" @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>

      <!-- 表格区域 -->
      <el-table v-loading="loading" :data="userList" border stripe>
        <el-table-column prop="id" label="ID" width="80" align="center" />
        <el-table-column prop="username" label="用户名" min-width="160" />
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'danger'">
              {{ row.status === 1 ? '正常' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="180">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="更新时间" width="180">
          <template #default="{ row }">{{ formatTime(row.updated_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="220" align="center" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link :icon="Edit" @click="handleEdit(row)">编辑</el-button>
            <el-button type="danger" link :icon="Delete" @click="handleDelete(row)">删除</el-button>
            <el-button :type="row.status === 1 ? 'warning' : 'success'" link @click="handleToggleStatus(row)">
              {{ row.status === 1 ? '禁用' : '启用' }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 分页区域 -->
      <div class="pagination-bar">
        <el-pagination
          v-model:current-page="query.page"
          v-model:page-size="query.pageSize"
          :total="total"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          background
          @current-change="handleCurrentChange"
          @size-change="handleSizeChange"
        />
      </div>
    </el-card>

    <!-- 编辑弹窗 -->
    <el-dialog v-model="editVisible" title="编辑用户" width="480px" :close-on-click-modal="false">
      <el-form ref="editFormRef" :model="editForm" :rules="editRules" label-width="80px">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="editForm.username" placeholder="请输入用户名（2~20 位）" maxlength="20" />
        </el-form-item>
        <el-form-item label="状态" prop="status">
          <el-select v-model="editForm.status" placeholder="请选择状态" style="width: 100%">
            <el-option label="正常" :value="1" />
            <el-option label="禁用" :value="2" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitLoading" @click="handleSubmitEdit">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped lang="scss">
.search-bar {
  margin-bottom: 16px;
}

.pagination-bar {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>


