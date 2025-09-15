<script setup lang="ts">
import type { Group, Category } from "@/types/models";
import { getGroupDisplayName } from "@/utils/display";
import { Add, Search } from "@vicons/ionicons5";
import { NButton, NCard, NEmpty, NInput, NSpin, NTag, NCollapse, NCollapseItem } from "naive-ui";
import { ref, watch, onMounted, onUnmounted, computed, nextTick } from "vue";
import { categoriesApi } from "@/api/categories";
import GroupFormModal from "./GroupFormModal.vue";
import GroupContextMenu from "./GroupContextMenu.vue";
import GroupCopyModal from "./GroupCopyModal.vue";
import CategoryFormModal from "./CategoryFormModal.vue";
import CategoryContextMenu from "./CategoryContextMenu.vue";
import { VueDraggableNext } from "vue-draggable-next";
import { log, setupGlobalLogExporter } from "@/utils/debug-logger";

// --- START: Persistence Logic ---
const CATEGORIES_EXPANDED_STORAGE_KEY = "gpt-load-categories-expanded";
// --- END: Persistence Logic ---

interface Props {
  groups: Group[];
  selectedGroup: Group | null;
  loading?: boolean;
}

interface Emits {
  (e: "group-select", group: Group): void;
  (e: "refresh"): void;
  (e: "refresh-and-select", groupId: number): void;
  (e: "group-archived", group: Group): void;
  (e: "group-unarchived", group: Group): void;
  (e: "group-updated", group: Group): void;
  (e: "groups-order-updated", groups: Group[]): void;
  (e: "edit", group: Group): void;
}

const props = withDefaults(defineProps<Props>(), {
  loading: false,
});

const emit = defineEmits<Emits>();

const searchText = ref("");
const showGroupModal = ref(false);
const showCopyModal = ref(false);
const selectedGroupForCopy = ref<Group | null>(null);

// 分类相关状态
const categories = ref<Category[]>([]);
const showCategoryModal = ref(false);
const selectedCategoryForEdit = ref<Category | null>(null);

// --- NEW DRAGGABLE STATE MANAGEMENT ---
const localUncategorizedGroups = ref<Group[]>([]);
const localCategoryGroups = ref<Record<number, Group[]>>({});

// 过滤和分组逻辑
const filteredGroups = computed(() => {
  if (!searchText.value) {
    return props.groups;
  }
  const search = searchText.value.toLowerCase();
  return props.groups.filter(group => {
    return (
      group.name.toLowerCase().includes(search) ||
      (group.display_name && group.display_name.toLowerCase().includes(search))
    );
  });
});

// 排序后的分类列表（归档分类固定在最后）
const sortedCategories = computed(() => {
  const archivedCategory = categories.value.find(cat => cat.name === "archived");
  const otherCategories = categories.value.filter(cat => cat.name !== "archived");

  // 其他分类按 sort 排序，归档分类固定在最后
  return [...otherCategories.sort((a, b) => a.sort - b.sort), ...(archivedCategory ? [archivedCategory] : [])];
});

// 监听 props 变化，更新本地状态
watch(
  [filteredGroups, categories],
  ([newGroups, newCategories]) => {
    log(
      "Groups or categories changed, updating local draggable lists",
      newGroups.map(g => ({ id: g.id, name: g.name, archived: g.archived, category_id: g.category_id }))
    );

    // 分类未分类的组（category_id 为 null 且 archived 为 false）
    localUncategorizedGroups.value = newGroups.filter(g => !g.category_id && !g.archived);

    // 按分类分组
    const categoryGroupsMap: Record<number, Group[]> = {};
    newCategories.forEach(cat => {
      if (cat.name === "archived") {
        // 归档分类包含：有 category_id 指向该分类的组 + archived=true 的组
        categoryGroupsMap[cat.id] = [
          ...newGroups.filter(g => g.category_id === cat.id),
          ...newGroups.filter(g => g.archived && !g.category_id)
        ];
      } else {
        // 其他分类只包含明确指定 category_id 的组
        categoryGroupsMap[cat.id] = newGroups.filter(g => g.category_id === cat.id);
      }
    });

    // 确保所有分类都有数组，即使是空的
    newCategories.forEach(cat => {
      if (!categoryGroupsMap[cat.id]) {
        categoryGroupsMap[cat.id] = [];
      }
    });

    localCategoryGroups.value = categoryGroupsMap;
  },
  { immediate: true, deep: true }
);

// 监听搜索文本变化
watch(searchText, () => {
  // filteredGroups 的计算属性会自动触发上面的 watch
});

// 拖拽结束处理
function handleDragEnd() {
  log("handleDragEnd triggered. Processing final state.");

  // 清理拖拽自动展开状态
  handleGlobalDragEnd();

  const uncategorized = localUncategorizedGroups.value;
  const allCategoryGroups = Object.values(localCategoryGroups.value).flat();

  log("Final list state", {
    uncategorized: uncategorized.map(g => ({ id: g.id, name: g.name })),
    categorized: allCategoryGroups.map(g => ({ id: g.id, name: g.name, category_id: g.category_id })),
  });

  // 构建最终的组列表
  let sortIndex = 0;
  const finalPayload: Group[] = [];

  // 未分类的组
  uncategorized.forEach(group => {
    finalPayload.push({
      ...group,
      category_id: null,
      archived: false,
      sort: sortIndex++,
    });
  });

  // 分类的组
  sortedCategories.value.forEach(category => {
    const categoryGroups = localCategoryGroups.value[category.id] || [];
    categoryGroups.forEach(group => {
      if (category.name === "archived") {
        // 归档分类中的组保持 archived=true 状态
        finalPayload.push({
          ...group,
          category_id: null, // 归档组不设置 category_id，通过 archived 字段标识
          archived: true,
          sort: sortIndex++,
        });
      } else {
        // 其他分类的组
        finalPayload.push({
          ...group,
          category_id: category.id,
          archived: false,
          sort: sortIndex++,
        });
      }
    });
  });

  log("Emitting SINGLE 'groups-order-updated' with final payload", finalPayload.map(g => ({
    id: g.id,
    name: g.name,
    archived: g.archived,
    category_id: g.category_id,
    sort: g.sort
  })));

  emit("groups-order-updated", finalPayload);
}
// --- END OF NEW DRAGGABLE STATE MANAGEMENT ---

// 右键菜单相关状态
const contextMenuData = ref<{
  show: boolean;
  x: number;
  y: number;
  group: Group | null;
}>({
  show: false,
  x: 0,
  y: 0,
  group: null,
});

// 分类右键菜单状态
const categoryContextMenuData = ref<{
  show: boolean;
  x: number;
  y: number;
  category: Category | null;
}>({
  show: false,
  x: 0,
  y: 0,
  category: null,
});

// 空白区域右键菜单状态
const blankContextMenuData = ref<{
  show: boolean;
  x: number;
  y: number;
}>({
  show: false,
  x: 0,
  y: 0,
});

// 展开状态管理 - 只保留分类展开状态
const categoryExpandedArray = ref<string[]>([]);

// 拖拽自动展开分类相关状态
const dragExpandTimer = ref<number | null>(null);
const currentDragOverCategory = ref<number | null>(null);

// 初始化
onMounted(async () => {
  setupGlobalLogExporter();

  // 加载分类数据
  await loadCategories();

  // --- START: Persistence Logic ---
  const savedCategoriesState = localStorage.getItem(CATEGORIES_EXPANDED_STORAGE_KEY);
  if (savedCategoriesState !== null) {
    // 从保存的状态中恢复展开的分类
    const savedMap: Record<number, boolean> = JSON.parse(savedCategoriesState);
    const expandedIds = Object.keys(savedMap).filter(id => savedMap[parseInt(id)]);
    categoryExpandedArray.value = expandedIds.map(id => `category-${id}`);
  }
  // --- END: Persistence Logic ---
});

// 组件卸载时清理定时器
onUnmounted(() => {
  handleGlobalDragEnd();
});

// 加载分类数据
async function loadCategories() {
  try {
    const newCategories = await categoriesApi.getCategories();

    // 使用 nextTick 避免在 watch 回调中立即触发响应式更新
    await nextTick();
    categories.value = newCategories;
  } catch (error) {
    console.error("加载分类失败:", error);
  }
}

// 同步分类展开状态 - 简化逻辑，只监听数组变化并持久化
watch(categoryExpandedArray, newValue => {
  // 转换为 map 格式进行持久化
  const mapForStorage: Record<number, boolean> = {};
  categories.value.forEach(cat => {
    mapForStorage[cat.id] = newValue.includes(`category-${cat.id}`);
  });
  localStorage.setItem(CATEGORIES_EXPANDED_STORAGE_KEY, JSON.stringify(mapForStorage));
});

// 拖拽自动展开分类处理函数
function handleCategoryDragEnter(categoryId: number) {
  console.log('Drag enter category:', categoryId);

  // 如果已经是当前悬停的分类，不重复处理
  if (currentDragOverCategory.value === categoryId) {
    return;
  }

  // 更新当前悬停的分类
  currentDragOverCategory.value = categoryId;

  // 如果分类已经展开，不需要处理
  const categoryKey = `category-${categoryId}`;
  if (categoryExpandedArray.value.includes(categoryKey)) {
    console.log('Category already expanded:', categoryKey);
    return;
  }

  // 清除之前的定时器
  if (dragExpandTimer.value) {
    clearTimeout(dragExpandTimer.value);
  }

  console.log('Setting expand timer for category:', categoryId);

  // 设置200ms延迟自动展开
  dragExpandTimer.value = setTimeout(() => {
    console.log('Timer triggered for category:', categoryId, 'Current hover:', currentDragOverCategory.value);
    // 检查是否还在同一个分类上
    if (currentDragOverCategory.value === categoryId) {
      // 自动展开分类
      if (!categoryExpandedArray.value.includes(categoryKey)) {
        console.log('Auto expanding category:', categoryKey);
        categoryExpandedArray.value.push(categoryKey);

        // 展开后，等待一小段时间让DOM更新，然后触发拖拽区域的dragover效果
        setTimeout(() => {
          triggerDragOverInCategory(categoryId);
        }, 50);
      }
    }
  }, 200);
}

function handleCategoryDragLeave(categoryId: number) {
  console.log('Drag leave category:', categoryId);
  // dragleave 事件不可靠，这里只做记录，不清除定时器
}

function handleCategoryDragOver(event: DragEvent, categoryId: number) {
  // 阻止默认行为以允许drop
  event.preventDefault();
  // console.log('Drag over category:', categoryId); // 太频繁了，注释掉

  // 确保当前悬停的分类是正确的
  if (currentDragOverCategory.value !== categoryId) {
    handleCategoryDragEnter(categoryId);
  }
}

// 添加全局拖拽结束事件监听，用于清理状态
function handleGlobalDragEnd() {
  console.log('Global drag end - cleaning up');
  currentDragOverCategory.value = null;
  if (dragExpandTimer.value) {
    clearTimeout(dragExpandTimer.value);
    dragExpandTimer.value = null;
  }

  // 清理所有自动展开占位符
  const placeholders = document.querySelectorAll('.auto-expand-placeholder');
  placeholders.forEach(placeholder => {
    if (placeholder.parentNode) {
      placeholder.parentNode.removeChild(placeholder);
      console.log('Removed auto-expand placeholder');
    }
  });
}

// 在分类展开后触发拖拽区域的dragover效果
function triggerDragOverInCategory(categoryId: number) {
  console.log('Triggering drag over in expanded category:', categoryId);

  // 查找对应分类的拖拽容器DOM元素
  const categoryKey = `category-${categoryId}`;
  const collapseItem = document.querySelector(`[name="${categoryKey}"]`);

  if (collapseItem) {
    // 查找分类内容区域的拖拽容器
    const dragContainer = collapseItem.querySelector('.category-list, .archived-list');

    if (dragContainer) {
      console.log('Found drag container, creating placeholder');

      // 检查是否已经有占位符
      const existingPlaceholder = dragContainer.querySelector('.auto-expand-placeholder');
      if (existingPlaceholder) {
        return;
      }

      // 手动创建一个占位符元素
      const placeholder = document.createElement('div');
      placeholder.className = 'sortable-ghost auto-expand-placeholder';
      placeholder.style.cssText = `
        opacity: 1;
        background: transparent;
        border: 2px dashed #667eea;
        border-radius: 6px;
        height: 40px;
        margin: 4px 0;
        pointer-events: none;
      `;

      // 插入占位符到容器的开头
      dragContainer.insertBefore(placeholder, dragContainer.firstChild);

      console.log('Placeholder created in drag container');

      // 5秒后移除占位符（防止永久存在）
      setTimeout(() => {
        if (placeholder.parentNode) {
          placeholder.parentNode.removeChild(placeholder);
          console.log('Auto-expand placeholder removed');
        }
      }, 5000);

    } else {
      console.log('Drag container not found in category');
    }
  } else {
    console.log('Category collapse item not found');
  }
}

function handleGroupClick(group: Group) {
  emit("group-select", group);
}

// 右键菜单处理
function handleContextMenu(event: MouseEvent, group: Group) {
  event.preventDefault();
  event.stopPropagation(); // 阻止事件冒泡到容器的空白区域菜单
  contextMenuData.value = {
    show: true,
    x: event.clientX,
    y: event.clientY,
    group,
  };
}

// 分类右键菜单处理
function handleCategoryContextMenu(event: MouseEvent, category: Category) {
  event.preventDefault();
  event.stopPropagation(); // 阻止事件冒泡到容器的空白区域菜单
  categoryContextMenuData.value = {
    show: true,
    x: event.clientX,
    y: event.clientY,
    category,
  };
}

// 空白区域右键菜单处理
function handleBlankContextMenu(event: MouseEvent) {
  event.preventDefault();
  blankContextMenuData.value = {
    show: true,
    x: event.clientX,
    y: event.clientY,
  };
}

// 归档分组
async function handleArchiveGroup(group: Group) {
  emit("group-archived", group);
}

// 取消归档分组
async function handleUnarchiveGroup(group: Group) {
  emit("group-unarchived", group);
}

// 获取渠道类型的标签颜色
function getChannelTagType(channelType: string) {
  switch (channelType) {
    case "openai":
      return "success";
    case "gemini":
      return "info";
    case "anthropic":
      return "warning";
    default:
      return "default";
  }
}

function openCreateGroupModal() {
  showGroupModal.value = true;
}

function handleGroupCreated(group: Group) {
  showGroupModal.value = false;
  if (group && group.id) {
    emit("refresh-and-select", group.id);
  }
}

// 处理复制分组
function handleCopyGroup(group: Group) {
  selectedGroupForCopy.value = group;
  showCopyModal.value = true;
}

// 处理编辑分组
function handleEditGroup(group: Group) {
  emit("group-select", group);
  emit("edit", group);
}

// 处理复制成功
function handleCopySuccess(newGroup: Group) {
  showCopyModal.value = false;
  selectedGroupForCopy.value = null;
  if (newGroup.id) {
    emit("refresh-and-select", newGroup.id);
  }
}

// 分类相关处理函数
function openCreateCategoryModal() {
  selectedCategoryForEdit.value = null;
  showCategoryModal.value = true;
}

function handleEditCategory(category: Category) {
  selectedCategoryForEdit.value = category;
  showCategoryModal.value = true;
}

async function handleCategoryUpdated() {
  // 重新加载分类数据，但不立即更新 categories.value
  try {
    const newCategories = await categoriesApi.getCategories();

    // 使用 nextTick 确保在下一个 tick 更新
    await nextTick();
    categories.value = newCategories;

    // 延迟发射 refresh 事件
    await nextTick();
    emit("refresh");
  } catch (error) {
    console.error("更新分类失败:", error);
  }
}

// 为分类组提供安全的双向绑定
function getCategoryGroups(categoryId: number) {
  return localCategoryGroups.value[categoryId] || [];
}

function setCategoryGroups(categoryId: number, groups: Group[]) {
  if (!localCategoryGroups.value[categoryId]) {
    localCategoryGroups.value[categoryId] = [];
  }
  localCategoryGroups.value[categoryId] = groups;
}

function handleCategoryCreatedOrUpdated() {
  showCategoryModal.value = false;
  handleCategoryUpdated();
}
</script>

<template>
  <div class="group-list-container" @contextmenu="handleBlankContextMenu">
    <n-card class="group-list-card modern-card" :bordered="false" size="small">
      <!-- 搜索框 -->
      <div class="search-section">
        <n-input v-model:value="searchText" placeholder="搜索节点名称..." size="small" clearable>
          <template #prefix>
            <n-icon :component="Search" />
          </template>
        </n-input>
      </div>

      <!-- 分组列表 -->
      <div class="groups-section">
        <n-spin :show="loading" size="small">
          <!-- 未分类分组容器 -->
          <div class="uncategorized-groups-container">
            <vue-draggable-next
              v-model="localUncategorizedGroups"
              class="groups-list"
              group="groups"
              :animation="150"
              ghost-class="sortable-ghost"
              handle=".group-item"
              @end="handleDragEnd"
            >
              <div
                v-for="group in localUncategorizedGroups"
                :key="group.id"
                class="group-item"
                :class="{ active: selectedGroup?.id === group.id }"
                @click="handleGroupClick(group)"
                @contextmenu="handleContextMenu($event, group)"
              >
                <div class="group-icon">
                  <span v-if="group.channel_type === 'openai'">🤖</span>
                  <span v-else-if="group.channel_type === 'gemini'">💎</span>
                  <span v-else-if="group.channel_type === 'anthropic'">🧠</span>
                  <span v-else>🔧</span>
                </div>
                <div class="group-content">
                  <div class="group-name">{{ getGroupDisplayName(group) }}</div>
                  <div class="group-meta">
                    <n-tag size="tiny" :type="getChannelTagType(group.channel_type)">
                      {{ group.channel_type }}
                    </n-tag>
                    <span class="group-id">#{{ group.name }}</span>
                  </div>
                </div>
              </div>
            </vue-draggable-next>
            <n-empty
              v-if="localUncategorizedGroups.length === 0 && !loading"
              size="small"
              :description="searchText ? '未找到匹配的节点' : '暂无节点'"
              class="empty-container"
            />
          </div>

          <!-- 分类分组容器 -->
          <div v-if="categories.length > 0" class="categorized-groups-container">
            <n-collapse v-model:expanded-names="categoryExpandedArray">
              <!-- 所有分类，包括归档分类 -->
              <n-collapse-item
                v-for="category in sortedCategories"
                :key="category.id"
                :name="`category-${category.id}`"
                :class="category.name === 'archived' ? 'archived-collapse' : 'category-collapse'"
                @dragenter.prevent="handleCategoryDragEnter(category.id)"
                @dragleave.prevent="handleCategoryDragLeave(category.id)"
                @dragover.prevent="handleCategoryDragOver($event, category.id)"
              >
                <template #header>
                  <div
                    :class="category.name === 'archived' ? 'archived-header' : 'category-header'"
                    @contextmenu="handleCategoryContextMenu($event, category)"
                  >
                    <span
                      :class="category.name === 'archived' ? 'archived-title' : 'category-title'"
                    >
                      {{ category.name === 'archived' ? '归档' : category.name }} ({{ (localCategoryGroups[category.id] || []).length }})
                    </span>
                  </div>
                </template>
                <vue-draggable-next
                  :model-value="getCategoryGroups(category.id)"
                  @update:model-value="(groups: Group[]) => setCategoryGroups(category.id, groups)"
                  :class="category.name === 'archived' ? 'archived-list' : 'category-list'"
                  group="groups"
                  :animation="150"
                  ghost-class="sortable-ghost"
                  handle=".group-item"
                  @end="handleDragEnd"
                >
                  <div
                    v-for="group in localCategoryGroups[category.id] || []"
                    :key="group.id"
                    :class="[
                      'group-item',
                      category.name === 'archived' ? 'archived-item' : 'categorized-item',
                      { active: selectedGroup?.id === group.id }
                    ]"
                    @click="handleGroupClick(group)"
                    @contextmenu="handleContextMenu($event, group)"
                  >
                    <div
                      :class="[
                        'group-icon',
                        category.name === 'archived' ? 'archived-icon' : 'categorized-icon'
                      ]"
                    >
                      <span v-if="group.channel_type === 'openai'">🤖</span>
                      <span v-else-if="group.channel_type === 'gemini'">💎</span>
                      <span v-else-if="group.channel_type === 'anthropic'">🧠</span>
                      <span v-else>🔧</span>
                    </div>
                    <div
                      :class="[
                        'group-content',
                        category.name === 'archived' ? 'archived-content' : 'categorized-content'
                      ]"
                    >
                      <div class="group-name">{{ getGroupDisplayName(group) }}</div>
                      <div class="group-meta">
                        <n-tag size="tiny" :type="getChannelTagType(group.channel_type)">
                          {{ group.channel_type }}
                        </n-tag>
                      </div>
                    </div>
                  </div>
                </vue-draggable-next>
              </n-collapse-item>
            </n-collapse>
          </div>
        </n-spin>
      </div>

      <!-- 添加分组按钮 -->
      <div class="add-section">
        <n-button type="primary" size="small" block @click="openCreateGroupModal">
          <template #icon>
            <n-icon :component="Add" />
          </template>
          创建节点
        </n-button>
      </div>
    </n-card>

    <!-- 分组右键菜单 -->
    <group-context-menu
      v-if="contextMenuData.group"
      v-model:show="contextMenuData.show"
      :x="contextMenuData.x"
      :y="contextMenuData.y"
      :group="contextMenuData.group"
      @archived="handleArchiveGroup"
      @unarchived="handleUnarchiveGroup"
      @group-updated="group => emit('group-updated', group)"
      @delete="group => emit('group-updated', group)"
      @copy="handleCopyGroup"
      @edit="handleEditGroup"
    />

    <!-- 分类右键菜单 -->
    <category-context-menu
      v-if="categoryContextMenuData.category"
      v-model:show="categoryContextMenuData.show"
      :x="categoryContextMenuData.x"
      :y="categoryContextMenuData.y"
      :category="categoryContextMenuData.category"
      @edit="handleEditCategory"
      @category-updated="handleCategoryUpdated"
    />

    <!-- 空白区域右键菜单 -->
    <n-dropdown
      v-if="blankContextMenuData.show"
      :options="[{ label: '增加分类', key: 'add-category' }]"
      :show="blankContextMenuData.show"
      :x="blankContextMenuData.x"
      :y="blankContextMenuData.y"
      placement="bottom-start"
      @clickoutside="blankContextMenuData.show = false"
      @select="(key: string) => { if (key === 'add-category') openCreateCategoryModal(); blankContextMenuData.show = false; }"
    />

    <!-- 分组创建/编辑模态框 -->
    <group-form-modal v-model:show="showGroupModal" @success="handleGroupCreated" />

    <!-- 分组复制模态框 -->
    <group-copy-modal
      v-model:show="showCopyModal"
      :source-group="selectedGroupForCopy"
      @success="handleCopySuccess"
    />

    <!-- 分类创建/编辑模态框 -->
    <category-form-modal
      v-model:show="showCategoryModal"
      :category="selectedCategoryForEdit"
      @success="handleCategoryCreatedOrUpdated"
    />
  </div>
</template>

<style scoped>
:deep(.n-card__content) {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.groups-section::-webkit-scrollbar {
  display: none;
}

.group-list-container {
  height: 100%;
}

.group-list-card {
  height: 100%;
}

.group-list-card:hover {
  transform: none;
  box-shadow: var(--shadow-lg);
}

.search-section {
  height: 41px;
}

.groups-section {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow-y: auto;
}

.empty-container {
  padding: 20px 0;
}

.uncategorized-groups-container {
  display: flex;
  flex-direction: column;
}

.categorized-groups-container {
  display: flex;
  flex-direction: column;
  border-top: 1px solid rgba(0, 0, 0, 0.06);
  padding-top: 12px;
}

.groups-list,
.category-list,
.archived-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.group-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px;
  border-radius: 6px;
  cursor: pointer;
  transition:
    background-color 0.2s ease,
    border-color 0.2s ease;
  border: 1px solid transparent;
  font-size: 12px;
}

.group-item:hover {
  background: rgba(102, 126, 234, 0.1);
  border-color: rgba(102, 126, 234, 0.2);
}

.group-item.active {
  background: var(--primary-gradient);
  color: white;
  border-color: transparent;
  box-shadow: var(--shadow-md);
}

.group-icon {
  font-size: 16px;
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(102, 126, 234, 0.1);
  border-radius: 6px;
  flex-shrink: 0;
}

.group-item.active .group-icon {
  background: rgba(255, 255, 255, 0.2);
}

.group-content {
  flex: 1;
  min-width: 0;
}

.group-name {
  font-weight: 600;
  font-size: 14px;
  line-height: 1.2;
  margin-bottom: 4px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.group-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 10px;
}

.group-id {
  opacity: 0.7;
  color: #64748b;
}

.group-item.active .group-id {
  opacity: 0.8;
  color: rgba(255, 255, 255, 0.8);
}

.add-section {
  border-top: 1px solid rgba(0, 0, 0, 0.06);
  padding-top: 12px;
}

/* 隐藏滚动条 */
.groups-list::-webkit-scrollbar,
.category-list::-webkit-scrollbar,
.archived-list::-webkit-scrollbar {
  display: none;
}

/* 分类样式 */
.category-header {
  display: flex;
  align-items: center;
  gap: 8px;
}

.category-title {
  font-size: 12px;
  font-weight: 600;
  color: #64748b;
}

.categorized-item {
  padding: 4px 8px;
  font-size: 11px;
}

.categorized-icon {
  width: 20px;
  height: 20px;
  font-size: 12px;
  background: rgba(148, 163, 184, 0.1);
}

.categorized-content {
  gap: 2px;
}

.categorized-item .group-name {
  font-size: 12px;
  margin-bottom: 2px;
}

.categorized-item .group-meta {
  font-size: 9px;
}

.categorized-item:hover {
  background: rgba(148, 163, 184, 0.1);
  border-color: rgba(148, 163, 184, 0.2);
}

.categorized-item.active {
  background: rgba(148, 163, 184, 0.2);
  color: #475569;
  border-color: rgba(148, 163, 184, 0.3);
}

.categorized-item.active .categorized-icon {
  background: rgba(255, 255, 255, 0.2);
}

/* 归档分组样式 */
.archived-header {
  display: flex;
  align-items: center;
  gap: 8px;
}

.archived-title {
  font-size: 12px;
  font-weight: 600;
  color: #64748b;
}

.archived-item {
  padding: 4px 8px;
  font-size: 11px;
}

.archived-icon {
  width: 20px;
  height: 20px;
  font-size: 12px;
  background: rgba(148, 163, 184, 0.1);
}

.archived-content {
  gap: 2px;
}

.archived-item .group-name {
  font-size: 12px;
  margin-bottom: 2px;
}

.archived-item .group-meta {
  font-size: 9px;
}

.archived-item:hover {
  background: rgba(148, 163, 184, 0.1);
  border-color: rgba(148, 163, 184, 0.2);
}

.archived-item.active {
  background: rgba(148, 163, 184, 0.2);
  color: #475569;
  border-color: rgba(148, 163, 184, 0.3);
}

.archived-item.active .archived-icon {
  background: rgba(255, 255, 255, 0.2);
}

:deep(.archived-collapse .n-collapse-item__header) {
  padding: 8px 0;
}

:deep(.category-collapse .n-collapse-item__header) {
  padding: 8px 0;
}

/* 确保整个标题行都能响应拖拽事件 */
:deep(.n-collapse-item__header) {
  width: 100%;
  min-height: 32px;
  display: flex;
  align-items: center;
}

:deep(.n-collapse-item__header-main) {
  flex: 1;
  width: 100%;
}

:deep(
  .n-collapse .n-collapse-item .n-collapse-item__content-wrapper .n-collapse-item__content-inner
) {
  padding-top: 0 !important;
}

/* 拖拽相关样式 */
.sortable-ghost {
  opacity: 1;
  background: transparent;
  border: 2px dashed #667eea;
  border-radius: 6px;
}

/* 选中状态下的拖拽占位符样式 - 与未选中状态保持一致 */
.sortable-ghost.active {
  background: transparent;
  border: 2px dashed #667eea;
}

.sortable-ghost .group-icon,
.sortable-ghost .group-content {
  opacity: 0;
}

.sortable-ghost.active .group-icon,
.sortable-ghost.active .group-content {
  opacity: 0;
}

.group-item.sortable-chosen {
  cursor: grabbing;
}

.groups-list > div,
.category-list > div,
.archived-list > div {
  transition: transform 0.2s ease-out;
}
</style>