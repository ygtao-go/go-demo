import 'vue-router'

declare module 'vue-router' {
  interface RouteMeta {
    /** 页面标题（用于菜单 / 面包屑 / document.title） */
    title?: string
    /** Element Plus 图标名（用于侧边栏菜单） */
    icon?: string
    /** 是否为公开页面（无需登录） */
    public?: boolean
  }
}
