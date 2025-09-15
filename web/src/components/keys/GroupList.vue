<script setup lang="ts">
import type { Group } from "@/types/models";
import { getGroupDisplayName } from "@/utils/display";
import { Add, Search } from "@vicons/ionicons5";
import { NButton, NCard, NEmpty, NInput, NSpin, NTag, NCollapse, NCollapseItem } from "naive-ui";
import { ref, watch, onMounted } from "vue";
import GroupFormModal from "./GroupFormModal.vue";
import GroupContextMenu from "./GroupContextMenu.vue";
import GroupCopyModal from "./GroupCopyModal.vue";
import { VueDraggableNext } from "vue-draggable-next";
import { log, setupGlobalLogExporter } from "@/utils/debug-logger";

// --- START: Persistence Logic ---
const ARCHIVED_EXPANDED_STORAGE_KEY = "gpt-load-archived-expanded";
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

// --- NEW DRAGGABLE STATE MANAGEMENT ---
const localActiveGroups = ref<Group[]>([]);
const localArchivedGroups = ref<Group[]>([]);

// Watch for prop changes to update local state
watch(
  () => props.groups,
  newGroups => {
    log(
      "Props changed, updating local draggable lists",
      newGroups.map(g => ({ id: g.id, name: g.name, archived: g.archived }))
    );
    const filtered = newGroups.filter(group => {
      if (!searchText.value) {
        return true;
      }
      const search = searchText.value.toLowerCase();
      return (
        group.name.toLowerCase().includes(search) ||
        (group.display_name && group.display_name.toLowerCase().includes(search))
      );
    });
    localActiveGroups.value = filtered.filter(g => !g.archived);
    localArchivedGroups.value = filtered.filter(g => g.archived);
  },
  { immediate: true, deep: true }
);

// Watch for search text changes to update local state
watch(searchText, () => {
  const filtered = props.groups.filter(group => {
    if (!searchText.value) {
      return true;
    }
    const search = searchText.value.toLowerCase();
    return (
      group.name.toLowerCase().includes(search) ||
      (group.display_name && group.display_name.toLowerCase().includes(search))
    );
  });
  localActiveGroups.value = filtered.filter(g => !g.archived);
  localArchivedGroups.value = filtered.filter(g => g.archived);
});

// This function is now only called ONCE at the end of the drag
function handleDragEnd() {
  log("handleDragEnd triggered. Processing final state.");

  const active = localActiveGroups.value;
  const archived = localArchivedGroups.value;

  log("Final list state", {
    active: active.map(g => ({ id: g.id, name: g.name })),
    archived: archived.map(g => ({ id: g.id, name: g.name })),
  });

  const activeWithState = active.map((group, index) => ({
    ...group,
    archived: false,
    sort: index,
  }));
  log(
    "Calculated final active groups with new state",
    activeWithState.map(g => ({ id: g.id, name: g.name, archived: g.archived, sort: g.sort }))
  );

  const archivedWithState = archived.map((group, index) => ({
    ...group,
    archived: true,
    sort: active.length + index,
  }));
  log(
    "Calculated final archived groups with new state",
    archivedWithState.map(g => ({ id: g.id, name: g.name, archived: g.archived, sort: g.sort }))
  );

  const finalPayload = [...activeWithState, ...archivedWithState];
  log(
    "Emitting SINGLE 'groups-order-updated' with final payload",
    finalPayload.map(g => ({ id: g.id, name: g.name, archived: g.archived, sort: g.sort }))
  );
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

// 归档列表展开状态
const archivedExpanded = ref(false);
const archivedExpandedArray = ref<string[]>([]);

// 初始化
onMounted(() => {
  setupGlobalLogExporter();
  // --- START: Persistence Logic ---
  const savedState = localStorage.getItem(ARCHIVED_EXPANDED_STORAGE_KEY);
  if (savedState !== null) {
    archivedExpanded.value = JSON.parse(savedState);
  }
  // --- END: Persistence Logic ---
});

// 同步展开状态并持久化
watch(archivedExpanded, newValue => {
  archivedExpandedArray.value = newValue ? ["archived"] : [];
  // --- START: Persistence Logic ---
  localStorage.setItem(ARCHIVED_EXPANDED_STORAGE_KEY, JSON.stringify(newValue));
  // --- END: Persistence Logic ---
});

// 监听数组变化来更新展开状态
watch(archivedExpandedArray, newValue => {
  archivedExpanded.value = newValue.includes("archived");
});

function handleGroupClick(group: Group) {
  emit("group-select", group);
}

// 右键菜单处理
function handleContextMenu(event: MouseEvent, group: Group) {
  event.preventDefault();
  contextMenuData.value = {
    show: true,
    x: event.clientX,
    y: event.clientY,
    group,
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
    // 创建成功后，通知父组件刷新并切换到新创建的分组
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
  // 先选择该分组，然后通知父组件进入编辑模式
  emit("group-select", group);
  // 直接发出编辑事件，由父组件处理编辑模式切换
  emit("edit", group);
}

// 处理复制成功
function handleCopySuccess(newGroup: Group) {
  showCopyModal.value = false;
  selectedGroupForCopy.value = null;
  // 通知父组件刷新并切换到新创建的分组
  if (newGroup.id) {
    emit("refresh-and-select", newGroup.id);
  }
}
</script>

<template>
  <div class="group-list-container">
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
          <!-- 常驻分组容器 -->
          <div class="active-groups-container">
            <vue-draggable-next
              v-model="localActiveGroups"
              class="groups-list"
              group="groups"
              :animation="150"
              ghost-class="sortable-ghost"
              handle=".group-item"
              @end="handleDragEnd"
            >
              <div
                v-for="group in localActiveGroups"
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
              v-if="localActiveGroups.length === 0 && !loading"
              size="small"
              :description="searchText ? '未找到匹配的节点' : '暂无节点'"
              class="empty-container"
            />
          </div>

          <!-- 归档分组容器 -->
          <div
            v-if="localArchivedGroups.length > 0 || searchText"
            class="archived-groups-container"
          >
            <n-collapse v-model:expanded-names="archivedExpandedArray">
              <n-collapse-item name="archived" class="archived-collapse">
                <template #header>
                  <div class="archived-header">
                    <span class="archived-title">归档 ({{ localArchivedGroups.length }})</span>
                  </div>
                </template>
                <vue-draggable-next
                  v-model="localArchivedGroups"
                  class="archived-list"
                  group="groups"
                  :animation="150"
                  ghost-class="sortable-ghost"
                  handle=".group-item"
                  @end="handleDragEnd"
                >
                  <div
                    v-for="group in localArchivedGroups"
                    :key="group.id"
                    class="group-item archived-item"
                    :class="{ active: selectedGroup?.id === group.id }"
                    @click="handleGroupClick(group)"
                    @contextmenu="handleContextMenu($event, group)"
                  >
                    <div class="group-icon archived-icon">
                      <span v-if="group.channel_type === 'openai'">🤖</span>
                      <span v-else-if="group.channel_type === 'gemini'">💎</span>
                      <span v-else-if="group.channel_type === 'anthropic'">🧠</span>
                      <span v-else>🔧</span>
                    </div>
                    <div class="group-content archived-content">
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

    <!-- 右键菜单 -->
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

    <group-form-modal v-model:show="showGroupModal" @success="handleGroupCreated" />

    <!-- 复制分组模态框 -->
    <group-copy-modal
      v-model:show="showCopyModal"
      :source-group="selectedGroupForCopy"
      @success="handleCopySuccess"
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

.active-groups-container {
  display: flex;
  flex-direction: column;
}

.archived-groups-container {
  display: flex;
  flex-direction: column;
  border-top: 1px solid rgba(0, 0, 0, 0.06);
  padding-top: 12px;
}

.groups-list,
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
.archived-list::-webkit-scrollbar {
  display: none;
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
.archived-list > div {
  transition: transform 0.2s ease-out;
}
</style>
