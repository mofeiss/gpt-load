<script setup lang="ts">
import type { Group } from "@/types/models";
import { keysApi } from "@/api/keys";
import { NDropdown, useDialog, useMessage } from "naive-ui";
import { computed, ref } from "vue";

interface Props {
  group: Group;
  show: boolean;
  x: number;
  y: number;
}

interface Emits {
  (e: "update:show", show: boolean): void;
  (e: "archived", group: Group): void;
  (e: "unarchived", group: Group): void;
  (e: "group-updated", group: Group): void;
  (e: "delete", group: Group): void;
  (e: "edit", group: Group): void;
  (e: "copy", group: Group): void;
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

const dialog = useDialog();
const message = useMessage();

// 加载状态
const isProcessing = ref(false);

const showDropdown = computed({
  get: () => props.show,
  set: value => emit("update:show", value),
});

const dropdownX = computed(() => props.x);
const dropdownY = computed(() => props.y);

const menuOptions = computed(() => {
  const options: Array<{
    label: string;
    key: string;
    icon?: () => string;
    style?: Record<string, string>;
    type?: string;
  }> = [
    {
      label: "编辑节点",
      key: "edit",
    },
    {
      label: "复制节点",
      key: "copy",
    },
    {
      label: "刷新",
      key: "refresh",
    },
  ];

  if (props.group.archived) {
    options.push({
      label: "取消归档",
      key: "unarchive",
    });
  } else {
    options.push({
      label: "归档",
      key: "archive",
    });
  }

  options.push(
    {
      label: "",
      key: "divider",
      type: "divider",
    },
    {
      label: "删除节点",
      key: "delete",
      style: {
        color: "red",
      },
    }
  );

  return options;
});

function closeDropdown() {
  showDropdown.value = false;
}

async function handleMenuSelect(key: string) {
  closeDropdown();

  switch (key) {
    case "archive":
      await archiveGroup();
      break;
    case "unarchive":
      await unarchiveGroup();
      break;
    case "delete":
      // 通过事件冒泡到父组件处理删除
      dialog.warning({
        title: "确认删除",
        content: `确定要删除节点 "${props.group.display_name || props.group.name}" 吗？此操作不可撤销。`,
        positiveText: "确定删除",
        negativeText: "取消",
        onPositiveClick: async () => {
          try {
            isProcessing.value = true;
            if (!props.group.id) {
              throw new Error("节点ID不能为空");
            }
            await keysApi.deleteGroup(props.group.id);
            message.success("节点删除成功");
            // 通过事件通知父组件
            emit("delete", props.group);
            // 删除成功后刷新页面
            setTimeout(() => {
              window.location.reload();
            }, 1000);
          } catch (error) {
            console.error("删除节点失败:", error);
            message.error("删除节点失败");
          } finally {
            isProcessing.value = false;
          }
        },
      });
      break;
    case "refresh":
      // 刷新事件通过父组件处理
      emit("group-updated", props.group);
      break;
    case "copy":
      emit("copy", props.group);
      break;
    case "edit":
      emit("edit", props.group);
      break;
  }
}

async function archiveGroup() {
  if (isProcessing.value) {
    return;
  }

  try {
    isProcessing.value = true;
    if (!props.group.id) {
      throw new Error("节点ID不能为空");
    }
    const updatedGroup = await keysApi.archiveGroup(props.group.id);
    message.success("节点归档成功");
    emit("archived", updatedGroup);
  } catch (error) {
    console.error("归档失败:", error);
    message.error("归档失败");
  } finally {
    isProcessing.value = false;
  }
}

async function unarchiveGroup() {
  if (isProcessing.value) {
    return;
  }

  try {
    isProcessing.value = true;
    if (!props.group.id) {
      throw new Error("节点ID不能为空");
    }
    const updatedGroup = await keysApi.unarchiveGroup(props.group.id);
    message.success("节点取消归档成功");
    emit("unarchived", updatedGroup);
  } catch (error) {
    console.error("取消归档失败:", error);
    message.error("取消归档失败");
  } finally {
    isProcessing.value = false;
  }
}
</script>

<template>
  <n-dropdown
    :options="menuOptions"
    :show="showDropdown"
    :x="dropdownX"
    :y="dropdownY"
    placement="bottom-start"
    @clickoutside="closeDropdown"
    @select="handleMenuSelect"
  />
</template>

<style scoped></style>
