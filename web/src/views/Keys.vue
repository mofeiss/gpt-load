<script setup lang="ts">
import { keysApi } from "@/api/keys";
import GroupInfoCard from "@/components/keys/GroupInfoCard.vue";
import GroupList from "@/components/keys/GroupList.vue";
import KeyTable from "@/components/keys/KeyTable.vue";
import type { Group } from "@/types/models";
import { onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";

const groups = ref<Group[]>([]);
const loading = ref(false);
const selectedGroup = ref<Group | null>(null);
const router = useRouter();
const route = useRoute();

onMounted(async () => {
  await loadGroups();
  restoreSelectedGroup();
});

async function loadGroups() {
  try {
    loading.value = true;
    groups.value = await keysApi.getGroups();
    // 只处理 URL 参数中的分组 ID，不自动选择第一个分组
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
  // 从 localStorage 读取保存的分组 ID 并自动切换
  const savedGroupId = localStorage.getItem("lastSelectedGroupId");
  if (savedGroupId && groups.value.length > 0) {
    const savedGroup = groups.value.find(g => String(g.id) === savedGroupId);
    if (savedGroup && savedGroup.id !== selectedGroup.value?.id) {
      handleGroupSelect(savedGroup);
      return;
    }
  }

  // 如果没有保存的分组，或者保存的分组不存在，则选择第一个分组
  if (groups.value.length > 0 && !selectedGroup.value) {
    handleGroupSelect(groups.value[0]);
  }
}

function handleGroupSelect(group: Group | null) {
  selectedGroup.value = group || null;

  // 保存选中的分组到 localStorage
  if (group?.id) {
    localStorage.setItem("lastSelectedGroupId", String(group.id));
  } else {
    localStorage.removeItem("lastSelectedGroupId");
  }

  if (String(group?.id) !== String(route.query.groupId)) {
    router.push({ name: "keys", query: { groupId: group?.id || "" } });
  }
}

async function handleGroupRefresh() {
  await loadGroups();
  restoreSelectedGroup();
}

async function handleGroupRefreshAndSelect(targetGroupId: number) {
  await loadGroups();
  // 临时设置选中的分组 ID 到 localStorage，确保 restoreSelectedGroup 能正确处理
  localStorage.setItem("lastSelectedGroupId", String(targetGroupId));
  restoreSelectedGroup();
}

function handleGroupDelete(deletedGroup: Group) {
  // 从分组列表中移除已删除的分组
  groups.value = groups.value.filter(g => g.id !== deletedGroup.id);

  // 如果删除的是当前选中的分组，则移除 localStorage 中的记录并重新选择分组
  if (selectedGroup.value?.id === deletedGroup.id) {
    localStorage.removeItem("lastSelectedGroupId");
    selectedGroup.value = null;
    restoreSelectedGroup();
  }
}

async function handleGroupCopySuccess(newGroup: Group) {
  // 重新加载分组列表以包含新创建的分组
  await loadGroups();
  // 自动切换到新创建的分组
  localStorage.setItem("lastSelectedGroupId", String(newGroup.id));
  restoreSelectedGroup();
}

function handleGroupUpdated(updatedGroup: Group) {
  // 更新当前选中的分组数据
  if (selectedGroup.value && selectedGroup.value.id === updatedGroup.id) {
    selectedGroup.value = updatedGroup;
  }

  // 更新分组列表中的对应分组
  const index = groups.value.findIndex(g => g.id === updatedGroup.id);
  if (index !== -1) {
    groups.value[index] = updatedGroup;
  }
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
      />
    </div>

    <!-- 右侧主内容区域，占80% -->
    <div class="main-content">
      <!-- 分组信息卡片，更紧凑 -->
      <div class="group-info">
        <group-info-card
          :group="selectedGroup"
          @refresh="handleGroupRefresh"
          @delete="handleGroupDelete"
          @copy-success="handleGroupCopySuccess"
          @group-updated="handleGroupUpdated"
        />
      </div>

      <!-- 密钥表格区域，占主要空间 -->
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
