<script setup lang="ts">
import { keysApi } from "@/api/keys";

// 为了符合 kebab-case 命名规范，使用 ccRModelsDisplay 作为组件名
import CCRSettingsCard from "@/components/keys/CCRSettingsCard.vue";
import GroupInfoCard from "@/components/keys/GroupInfoCard.vue";
import GroupList from "@/components/keys/GroupList.vue";
import KeyTable from "@/components/keys/KeyTable.vue";
import type { Group } from "@/types/models";
import { onMounted, ref } from "vue";
import { useRoute } from "vue-router";
import { useMessage } from "naive-ui";

const groups = ref<Group[]>([]);
const loading = ref(false);
const selectedGroup = ref<Group | null>(null);
const route = useRoute();
const message = useMessage();
const groupInfoCardRef = ref();

onMounted(async () => {
  await loadGroups();
  restoreSelectedGroup();
});

async function loadGroups() {
  try {
    loading.value = true;
    const fetchedGroups = await keysApi.getGroups();
    // 按 sort 字段和 id 排序
    groups.value = fetchedGroups.sort((a, b) => (a.sort ?? a.id) - (b.sort ?? b.id));
    if (groups.value.length > 0 && !selectedGroup.value && route.query.groupId) {
      const found = groups.value.find(g => String(g.id) === String(route.query.groupId));
      if (found) {
        selectedGroup.value = found;
      }
    }
  } finally {
    loading.value = false;
  }
}

function restoreSelectedGroup() {
  const savedGroupId = localStorage.getItem("lastSelectedGroupId");
  if (savedGroupId && groups.value.length > 0) {
    const savedGroup = groups.value.find(g => String(g.id) === savedGroupId);
    if (savedGroup && savedGroup.id !== selectedGroup.value?.id) {
      handleGroupSelect(savedGroup);
      return;
    }
  }

  if (groups.value.length > 0 && !selectedGroup.value) {
    const firstGroup = groups.value.find(g => !g.archived) || groups.value[0];
    handleGroupSelect(firstGroup);
  }
}

function handleGroupSelect(group: Group | null) {
  selectedGroup.value = group || null;

  if (group?.id) {
    localStorage.setItem("lastSelectedGroupId", String(group.id));
  } else {
    localStorage.removeItem("lastSelectedGroupId");
  }

  // 注释掉强制路由跳转，避免页面刷新时的跳转问题
  // if (String(group?.id) !== String(route.query.groupId)) {
  //   router.push({ name: "keys", query: { groupId: group?.id || "" } });
  // }
}

async function handleGroupRefresh() {
  const currentGroupId = selectedGroup.value?.id;
  await loadGroups();
  if (currentGroupId) {
    const refreshedGroup = groups.value.find(g => g.id === currentGroupId);
    if (refreshedGroup) {
      selectedGroup.value = refreshedGroup;
    } else {
      // The group may have been deleted, so we fall back to restoring selection logic
      restoreSelectedGroup();
    }
  } else {
    restoreSelectedGroup();
  }
}

async function handleGroupRefreshAndSelect(targetGroupId: number) {
  await loadGroups();
  localStorage.setItem("lastSelectedGroupId", String(targetGroupId));
  restoreSelectedGroup();
}

function handleGroupDelete(deletedGroup: Group) {
  groups.value = groups.value.filter(g => g.id !== deletedGroup.id);
  if (selectedGroup.value?.id === deletedGroup.id) {
    localStorage.removeItem("lastSelectedGroupId");
    selectedGroup.value = null;
    restoreSelectedGroup();
  }
}

async function handleGroupCopySuccess(newGroup: Group) {
  await loadGroups();
  localStorage.setItem("lastSelectedGroupId", String(newGroup.id));
  restoreSelectedGroup();
}

function handleGroupUpdated(updatedGroup: Group) {
  if (selectedGroup.value && selectedGroup.value.id === updatedGroup.id) {
    selectedGroup.value = updatedGroup;
  }
  const index = groups.value.findIndex(g => g.id === updatedGroup.id);
  if (index !== -1) {
    groups.value[index] = updatedGroup;
  }
}

function handleGroupArchived(archivedGroup: Group) {
  const index = groups.value.findIndex(g => g.id === archivedGroup.id);
  if (index !== -1) {
    groups.value[index] = archivedGroup;
  }
  if (selectedGroup.value?.id === archivedGroup.id) {
    selectedGroup.value = null;
    localStorage.removeItem("lastSelectedGroupId");
  }
}

function handleGroupUnarchived(unarchivedGroup: Group) {
  const index = groups.value.findIndex(g => g.id === unarchivedGroup.id);
  if (index !== -1) {
    groups.value[index] = unarchivedGroup;
  }
}

async function handleGroupsOrderUpdated(updatedGroups: Group[]) {
  // 1. 更新本地视图，立即响应
  groups.value = updatedGroups.sort((a, b) => (a.sort ?? a.id) - (b.sort ?? b.id));

  // 2. 提取需要发送到后端的数据
  const payload = updatedGroups.map(g => ({
    id: g.id,
    sort: g.sort,
    archived: g.archived,
    category_id: g.category_id, // 添加 category_id 字段，这是关键的分类关联数据
  }));

  // 3. 调用 API 更新
  try {
    await keysApi.updateGroupsOrder(payload);
    message.success("分组排序已保存");
  } catch (_error) {
    message.error("保存分组排序失败，请重试");
    // 如果失败，重新加载以恢复到之前的状态
    await loadGroups();
  }
}

function handleEditGroup(group: Group) {
  // 确保选中该分组
  selectedGroup.value = group;
  // 通过ref调用GroupInfoCard的编辑方法
  groupInfoCardRef.value?.handleEdit();
}
</script>

<template>
  <div class="keys-container">
    <div class="sidebar">
      <group-list
        :groups="groups"
        :selected-group="selectedGroup"
        :loading="loading"
        @group-select="handleGroupSelect"
        @refresh="handleGroupRefresh"
        @refresh-and-select="handleGroupRefreshAndSelect"
        @group-archived="handleGroupArchived"
        @group-unarchived="handleGroupUnarchived"
        @group-updated="handleGroupUpdated"
        @groups-order-updated="handleGroupsOrderUpdated"
        @edit="handleEditGroup"
      />
    </div>

    <!-- 右侧主内容区域 -->
    <div class="main-content">
      <!-- CCR 模型设置区域 -->
      <c-c-r-settings-card :group="selectedGroup" @refresh="handleGroupRefresh" />

      <!-- 分组信息卡片 -->
      <div class="group-info">
        <group-info-card
          ref="groupInfoCardRef"
          :group="selectedGroup"
          @refresh="handleGroupRefresh"
          @delete="handleGroupDelete"
          @copy-success="handleGroupCopySuccess"
          @group-updated="handleGroupUpdated"
        />
      </div>

      <!-- 密钥表格区域 -->
      <div class="key-table-section">
        <key-table :selected-group="selectedGroup" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.keys-container {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

.sidebar {
  width: 100%;
  flex-shrink: 0;
}

.main-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.group-info {
  flex-shrink: 0;
}

.key-table-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

@media (min-width: 768px) {
  .keys-container {
    flex-direction: row;
  }

  .sidebar {
    width: 240px;
    height: calc(100vh - 159px);
  }
}
</style>
