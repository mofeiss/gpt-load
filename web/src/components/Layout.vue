<script setup lang="ts">
import AppFooter from "@/components/AppFooter.vue";
import GlobalTaskProgressBar from "@/components/GlobalTaskProgressBar.vue";
import Logout from "@/components/Logout.vue";
import NavBar from "@/components/NavBar.vue";
// 导入所有页面组件
import Dashboard from "@/views/Dashboard.vue";
import Keys from "@/views/Keys.vue";
import CCR from "@/views/CCR.vue";
import Logs from "@/views/Logs.vue";
import Settings from "@/views/Settings.vue";
import { useMediaQuery } from "@vueuse/core";
import { ref, watch, computed, onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";

const isMenuOpen = ref(false);
const isMobile = useMediaQuery("(max-width: 768px)");
const route = useRoute();
const router = useRouter();

// 当前激活的 tab
const activeTab = ref<string>("dashboard");

// tab 定义
const tabs = [
  { name: "dashboard", label: "仪表盘", icon: "📊", component: Dashboard },
  { name: "keys", label: "密钥管理", icon: "🔑", component: Keys },
  { name: "ccr", label: "CCR", icon: "🌐", component: CCR },
  { name: "logs", label: "日志", icon: "📋", component: Logs },
  { name: "settings", label: "系统设置", icon: "⚙️", component: Settings },
];

// 检查是否为 CCR 页面，需要全屏布局
const isFullscreenPage = computed(() => {
  return activeTab.value === "ccr";
});

// 同步 URL 与 tab
const syncUrlWithTab = (tabName: string) => {
  const tabRoute = tabName === "dashboard" ? "/" : `/${tabName}`;
  if (route.path !== tabRoute) {
    router.push(tabRoute);
  }
};

// tab 切换处理
const handleTabChange = (tabName: string) => {
  activeTab.value = tabName;
  syncUrlWithTab(tabName);
  // 保存当前视图到 localStorage
  localStorage.setItem("lastActiveView", tabName);
  // 移动端关闭菜单
  if (isMobile.value) {
    isMenuOpen.value = false;
  }
};

// 监听路由变化并同步到 tab
watch(
  () => route.name,
  newRouteName => {
    if (newRouteName && typeof newRouteName === "string" && newRouteName !== activeTab.value) {
      activeTab.value = newRouteName;
    }
  },
  { immediate: true }
);

// 监听移动端变化
watch(isMobile, value => {
  if (!value) {
    isMenuOpen.value = false;
  }
});

// 恢复上次访问的页面（仅在首页时执行）
onMounted(() => {
  const savedView = localStorage.getItem("lastActiveView");
  // 只有当前在根路径时才恢复上次访问的页面，避免刷新时强制跳转
  if (savedView && tabs.some(tab => tab.name === savedView) && route.path === "/") {
    activeTab.value = savedView;
    syncUrlWithTab(savedView);
  }
});

const toggleMenu = () => {
  isMenuOpen.value = !isMenuOpen.value;
};
</script>

<template>
  <n-layout class="main-layout">
    <n-layout-header class="layout-header">
      <div class="header-content">
        <div class="header-brand">
          <div class="brand-icon">
            <img src="@/assets/logo.png" alt="" />
          </div>
          <h1 v-if="!isMobile" class="brand-title">GPT Load</h1>
        </div>

        <nav v-if="!isMobile" class="header-nav">
          <nav-bar :active-tab="activeTab" @tab-change="handleTabChange" />
        </nav>

        <div class="header-actions">
          <logout v-if="!isMobile" />
          <n-button v-else text @click="toggleMenu">
            <svg viewBox="0 0 24 24" width="24" height="24">
              <path fill="currentColor" d="M3,6H21V8H3V6M3,11H21V13H3V11M3,16H21V18H3V16Z" />
            </svg>
          </n-button>
        </div>
      </div>
    </n-layout-header>

    <n-drawer v-model:show="isMenuOpen" :width="240" placement="right">
      <n-drawer-content title="GPT Load" body-content-style="padding: 0;">
        <nav-bar :active-tab="activeTab" @tab-change="handleTabChange" mode="vertical" />
        <div class="mobile-actions">
          <logout />
        </div>
      </n-drawer-content>
    </n-drawer>

    <n-layout-content class="layout-content">
      <div class="content-container" :class="{ 'fullscreen-mode': isFullscreenPage }">
        <!-- 所有页面组件，统一使用 v-show 控制显示 -->
        <template v-for="tab in tabs" :key="tab.name">
          <div
            v-show="activeTab === tab.name"
            :class="{
              'content-wrapper': tab.name !== 'ccr',
              'fullscreen-content': tab.name === 'ccr',
            }"
          >
            <component :is="tab.component" />
          </div>
        </template>
      </div>
    </n-layout-content>
    <app-footer v-if="!isFullscreenPage" />
  </n-layout>

  <!-- 全局任务进度条 -->
  <global-task-progress-bar />
</template>

<style scoped>
.main-layout {
  background: transparent;
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

.layout-header {
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(20px);
  border-bottom: 1px solid rgba(0, 0, 0, 0.08);
  box-shadow: var(--shadow-sm);
  position: sticky;
  top: 0;
  z-index: 100;
  padding: 0 12px;
}

.header-content {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 0;
  overflow-x: auto;
  max-width: 1200px;
  margin: 0 auto;
}

.header-brand {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.brand-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 35px;
  height: 35px;
  img {
    height: 100%;
    width: 100%;
  }
}

.brand-title {
  font-size: 1.4rem;
  font-weight: 700;
  background: var(--primary-gradient);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  margin: 0;
  letter-spacing: -0.3px;
}

.header-actions {
  flex-shrink: 0;
  display: flex;
  align-items: center;
}

.mobile-actions {
  padding: 12px;
  border-top: 1px solid rgba(0, 0, 0, 0.08);
}

.layout-content {
  flex: 1;
  overflow: auto;
  background: transparent;
  max-width: 1200px;
  margin: 0 auto;
  width: 100%;
}

.content-container {
  position: relative;
  width: 100%;
  height: 100%;
}

.content-wrapper {
  padding: 16px;
  min-height: calc(100vh - 111px);
}

.fullscreen-content {
  padding: 16px; /* 与其他页面保持一致的padding */
  height: calc(100vh - 65px); /* 减去顶部导航栏高度 */
  position: relative;
}

/* 全屏模式下的容器样式调整 */
.content-container.fullscreen-mode {
  overflow: hidden;
}

.layout-footer {
  background: transparent;
  padding: 0;
}
</style>
