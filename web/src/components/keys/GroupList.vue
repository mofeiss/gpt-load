<script setup lang="ts">
import type { Group } from "@/types/models";
import { getGroupDisplayName } from "@/utils/display";
import { Add, Search } from "@vicons/ionicons5";
import { NButton, NCard, NEmpty, NInput, NSpin, NTag, NCollapse, NCollapseItem } from "naive-ui";
import { computed, ref, watch } from "vue";
import GroupFormModal from "./GroupFormModal.vue";
import GroupContextMenu from "./GroupContextMenu.vue";

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
}

const props = withDefaults(defineProps<Props>(), {
  loading: false,
});

const emit = defineEmits<Emits>();

const searchText = ref("");
const showGroupModal = ref(false);

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

// 同步展开状态
watch(archivedExpanded, newValue => {
  archivedExpandedArray.value = newValue ? ["archived"] : [];
});

// 监听数组变化来更新展开状态
watch(archivedExpandedArray, newValue => {
  archivedExpanded.value = newValue.includes("archived");
});

// 过滤后的分组列表
const filteredGroups = computed(() => {
  if (!searchText.value) {
    return props.groups;
  }
  const search = searchText.value.toLowerCase();
  return props.groups.filter(
    group =>
      group.name.toLowerCase().includes(search) ||
      (group.display_name && group.display_name.toLowerCase().includes(search))
  );
});

// 常驻分组（未归档）
const activeGroups = computed(() => {
  return filteredGroups.value.filter(group => !group.archived);
});

// 归档分组
const archivedGroups = computed(() => {
  return filteredGroups.value.filter(group => group.archived);
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
</script>

<template>
  <div class="group-list-container">
    <n-card class="group-list-card modern-card" :bordered="false" size="small">
      <!-- 搜索框 -->
      <div class="search-section">
        <n-input v-model:value="searchText" placeholder="搜索分组名称..." size="small" clearable>
          <template #prefix>
            <n-icon :component="Search" />
          </template>
        </n-input>
      </div>

      <!-- 分组列表 -->
      <div class="groups-section">
        <n-spin :show="loading" size="small">
          <!-- 常驻分组 -->
          <div v-if="activeGroups.length === 0 && !loading" class="empty-container">
            <n-empty size="small" :description="searchText ? '未找到匹配的分组' : '暂无分组'" />
          </div>
          <div v-else class="groups-list">
            <div
              v-for="group in activeGroups"
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
          </div>

          <!-- 归档分组 -->
          <div v-if="archivedGroups.length > 0" class="archived-section">
            <n-collapse v-model:expanded-names="archivedExpandedArray">
              <n-collapse-item name="archived" class="archived-collapse">
                <template #header>
                  <div class="archived-header">
                    <span class="archived-title">归档分组 ({{ archivedGroups.length }})</span>
                  </div>
                </template>

                <div class="archived-list">
                  <div
                    v-for="group in archivedGroups"
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
                </div>
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
          创建分组
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
    />

    <group-form-modal v-model:show="showGroupModal" @success="handleGroupCreated" />
  </div>
</template>

<style scoped>
:deep(.n-card__content) {
  height: 100%;
}

.groups-section::-webkit-scrollbar {
  width: 1px;
  height: 1px;
}

.group-list-container {
  height: 100%;
}

.group-list-card {
  height: 100%;
  display: flex;
  flex-direction: column;
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
  height: calc(100% - 82px);
  overflow: auto;
}

.empty-container {
  padding: 20px 0;
}

.groups-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 100%;
  overflow-y: auto;
}

.group-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s ease;
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

/* 滚动条样式 */
.groups-list::-webkit-scrollbar {
  width: 4px;
}

.groups-list::-webkit-scrollbar-track {
  background: transparent;
}

.groups-list::-webkit-scrollbar-thumb {
  background: rgba(0, 0, 0, 0.2);
  border-radius: 2px;
}

.groups-list::-webkit-scrollbar-thumb:hover {
  background: rgba(0, 0, 0, 0.3);
}

/* 归档分组样式 */
.archived-section {
  margin-top: 12px;
  border-top: 1px solid rgba(0, 0, 0, 0.06);
  padding-top: 12px;
}

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

.archived-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  max-height: 200px;
  overflow-y: auto;
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

:deep(.archived-collapse .n-collapse-item__content-inner) {
  padding-top: 8px;
}

/* 归档列表滚动条 */
.archived-list::-webkit-scrollbar {
  width: 3px;
}

.archived-list::-webkit-scrollbar-track {
  background: transparent;
}

.archived-list::-webkit-scrollbar-thumb {
  background: rgba(0, 0, 0, 0.1);
  border-radius: 2px;
}

.archived-list::-webkit-scrollbar-thumb:hover {
  background: rgba(0, 0, 0, 0.2);
}
</style>
