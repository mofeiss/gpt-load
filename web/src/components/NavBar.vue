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
  border-radius: var(--border-radius-md);
  transition: all 0.3s ease;
  font-weight: 500;
  cursor: pointer;
  position: relative;
  min-width: 100px;
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
  border-radius: var(--border-radius-md);
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

/* Hover 状态 - 作用在 nav-menu-item 上 */
:deep(.n-menu-item:hover .nav-menu-item) {
  background: rgba(102, 126, 234, 0.15) !important;
  transform: translateY(-1px);
  box-shadow: 0 2px 8px rgba(102, 126, 234, 0.2);
}

:deep(.n-menu-item:hover .nav-item-icon) {
  transform: scale(1.05);
}

/* Active 状态 - 作用在 nav-menu-item 上 */
:deep(.nav-menu-item--active) {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%) !important;
  color: white !important;
  font-weight: 600;
  box-shadow:
    0 4px 6px -1px rgba(0, 0, 0, 0.1),
    0 2px 4px -1px rgba(0, 0, 0, 0.06);
  transform: translateY(-1px);
}

:deep(.nav-menu-item--active .nav-item-icon) {
  transform: scale(1.1);
  filter: drop-shadow(0 1px 2px rgba(0, 0, 0, 0.1));
  color: white !important;
}

:deep(.nav-menu-item--active .nav-item-text) {
  font-weight: 600;
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.1);
  color: white !important;
}

/* Active hover 状态 */
:deep(.n-menu-item:hover .nav-menu-item--active) {
  background: linear-gradient(135deg, #5a6fd8 0%, #6a4190 100%) !important;
  transform: translateY(-2px);
  box-shadow:
    0 10px 15px -3px rgba(0, 0, 0, 0.1),
    0 4px 6px -2px rgba(0, 0, 0, 0.05);
}

/* 添加底部指示器 */
:deep(.nav-menu-item--active::after) {
  content: "";
  position: absolute;
  bottom: -2px;
  left: 50%;
  transform: translateX(-50%);
  width: 20px;
  height: 2px;
  background: white;
  border-radius: 1px;
  opacity: 0.8;
}

:deep(.n-menu--vertical .nav-menu-item--active::after) {
  bottom: auto;
  right: -2px;
  left: auto;
  top: 50%;
  transform: translateY(-50%);
  width: 2px;
  height: 20px;
}
</style>
