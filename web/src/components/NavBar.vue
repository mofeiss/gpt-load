<script setup lang="ts">
import { type MenuOption } from "naive-ui";
import { computed, h } from "vue";

const props = defineProps({
  mode: {
    type: String,
    default: "horizontal",
  },
  activeTab: {
    type: String,
    required: true,
  },
});

const emit = defineEmits<{
  tabChange: [tabName: string];
}>();

const menuOptions = computed<MenuOption[]>(() => {
  const options: MenuOption[] = [
    renderMenuItem("dashboard", "仪表盘", "📊"),
    renderMenuItem("keys", "密钥管理", "🔑"),
    renderMenuItem("ccr", "CCR", "🌐"),
    renderMenuItem("logs", "日志", "📋"),
    renderMenuItem("settings", "系统设置", "⚙️"),
  ];

  return options;
});

// tab 切换处理
const handleTabClick = (tabName: string) => {
  emit("tabChange", tabName);
};

function renderMenuItem(key: string, label: string, icon: string): MenuOption {
  const isActive = key === props.activeTab;
  return {
    label: () =>
      h(
        "div",
        {
          class: ["nav-menu-item", isActive ? "nav-menu-item--active" : ""],
          onClick: () => handleTabClick(key),
        },
        [h("span", { class: "nav-item-icon" }, icon), h("span", { class: "nav-item-text" }, label)]
      ),
    key,
  };
}
</script>

<template>
  <div>
    <n-menu
      :mode="props.mode"
      :options="menuOptions"
      :value="props.activeTab"
      class="modern-menu"
    />
  </div>
</template>

<style scoped>
:deep(.nav-menu-item) {
  display: flex;
  align-items: center;
  gap: 8px;
  text-decoration: none;
  color: inherit;
  padding: 8px 12px;
  transition: all 0.3s ease;
  font-weight: 500;
  cursor: pointer;
  position: relative;
  min-width: 80px;
  justify-content: center;
}

:deep(.nav-item-icon) {
  transition: all 0.3s ease;
  font-size: 1.1em;
}

:deep(.nav-item-text) {
  transition: all 0.3s ease;
}

:deep(.n-menu-item) {
  cursor: pointer;
  position: relative;
  transition: all 0.3s ease;
  margin: 2px 4px;
  padding: 0 !important; /* 移除 Naive UI 的默认 padding */
}

:deep(.n-menu--horizontal .n-menu-item) {
  margin: 2px 4px;
}

:deep(.n-menu--vertical .n-menu-item-content) {
  justify-content: center;
  padding: 0 !important;
}

:deep(.n-menu--vertical .n-menu-item) {
  margin: 4px 8px;
}

/* Hover 状态 - 只显示底部线条 */
:deep(.n-menu-item:hover .nav-menu-item::after) {
  opacity: 0.5;
}

/* Active 状态 - 只显示颜色变化和底部线条 */
:deep(.nav-menu-item--active) {
  color: #667eea !important;
  font-weight: 600;
}

:deep(.nav-menu-item--active .nav-item-icon) {
  color: #667eea !important;
}

:deep(.nav-menu-item--active .nav-item-text) {
  font-weight: 600;
  color: #667eea !important;
}

/* 添加底部指示器 - 统一使用3px高度的线条 */
:deep(.nav-menu-item::after) {
  content: "";
  position: absolute;
  bottom: 0;
  left: 50%;
  transform: translateX(-50%);
  width: 60%;
  height: 3px;
  background: #667eea;
  border-radius: 2px;
  opacity: 0;
  transition: opacity 0.3s ease;
}

:deep(.nav-menu-item--active::after) {
  opacity: 1;
}

:deep(.n-menu--vertical .nav-menu-item::after) {
  bottom: auto;
  right: 0;
  left: auto;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 60%;
}
</style>
