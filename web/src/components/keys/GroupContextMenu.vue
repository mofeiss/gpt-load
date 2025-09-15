<script setup lang="ts">
import type { Group, Category } from "@/types/models";
import { keysApi } from "@/api/keys";
import { categoriesApi } from "@/api/categories";
import { NDropdown, useDialog, useMessage } from "naive-ui";
import { computed, ref, onMounted, watch } from "vue";

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

// 分类数据
const categories = ref<Category[]>([]);

const showDropdown = computed({
  get: () => props.show,
  set: value => emit("update:show", value),
});

const dropdownX = computed(() => props.x);
const dropdownY = computed(() => props.y);

// 实时加载分类数据 - 当菜单显示时重新加载
const refreshCategories = async () => {
  try {
    categories.value = await categoriesApi.getCategories();
  } catch (error) {
    console.error("加载分类失败:", error);
  }
};

// 监听菜单显示状态，显示时刷新分类数据
watch(() => props.show, async (newShow) => {
  if (newShow) {
    await refreshCategories();
  }
});

// 加载分类数据
onMounted(async () => {
  try {
    categories.value = await categoriesApi.getCategories();
  } catch (error) {
    console.error("加载分类失败:", error);
  }
});

const menuOptions = computed(() => {
  const options: Array<{
    label: string;
    key: string;
    icon?: () => string;
    style?: Record<string, string>;
    type?: string;
    children?: Array<{
      label: string;
      key: string;
      style?: Record<string, string>;
    }>;
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

  // 添加移动到分类的子菜单
  if (categories.value.length > 0) {
    // 过滤掉 archived 分类，避免重复显示
    const nonArchivedCategories = categories.value.filter(category => category.name !== "archived");

    const moveToOptions = nonArchivedCategories.map(category => ({
      label: category.name,
      key: `move-to-category-${category.id}`,
    }));

    // 如果当前已分类，添加"移动到未分类"选项
    if (props.group.category_id) {
      moveToOptions.unshift({
        label: "未分类",
        key: "move-to-uncategorized",
      });
    }

    // 只有在有可移动的分类时才添加"移动到"子菜单
    if (moveToOptions.length > 0) {
      options.push({
        label: "移动到",
        key: "move-to",
        children: moveToOptions,
      });
    }
  }

  // 添加归档/取消归档选项
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

  if (key.startsWith("move-to-category-")) {
    const categoryId = parseInt(key.replace("move-to-category-", ""));
    await moveToCategory(categoryId);
    return;
  }

  switch (key) {
    case "archive":
      await archiveGroup();
      break;
    case "unarchive":
      await unarchiveGroup();
      break;
    case "move-to-uncategorized":
      await moveToCategory(null);
      break;
    case "delete":
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
            emit("delete", props.group);
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

async function moveToCategory(categoryId: number | null) {
  if (isProcessing.value) {
    return;
  }

  try {
    isProcessing.value = true;
    if (!props.group.id) {
      throw new Error("节点ID不能为空");
    }

    // 构建更新数据
    const updateData = {
      ...props.group,
      category_id: categoryId,
      archived: false, // 移动到分类时取消归档状态
    };

    const updatedGroups = [updateData];
    await keysApi.updateGroupsOrder(updatedGroups);

    const categoryName = categoryId
      ? categories.value.find(c => c.id === categoryId)?.name || "分类"
      : "未分类";

    message.success(`已移动到${categoryName}`);
    emit("group-updated", updateData);
  } catch (error) {
    console.error("移动节点失败:", error);
    message.error("移动节点失败");
  } finally {
    isProcessing.value = false;
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

    // 找到名为 "archived" 的分类
    const archivedCategory = categories.value.find(cat => cat.name === "archived");

    if (archivedCategory) {
      // 如果存在 archived 分类，移动到该分类
      const updateData = {
        ...props.group,
        category_id: null, // 归档的组仍然使用 archived 字段而不是 category_id
        archived: true,
      };

      const updatedGroups = [updateData];
      await keysApi.updateGroupsOrder(updatedGroups);
      message.success("节点已归档");
      emit("archived", updateData);
    } else {
      // 如果没有 archived 分类，使用原有的归档 API
      const updatedGroup = await keysApi.archiveGroup(props.group.id);
      message.success("节点归档成功");
      emit("archived", updatedGroup);
    }
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

    // 取消归档就是移动到未分类
    const updateData = {
      ...props.group,
      category_id: null,
      archived: false,
    };

    const updatedGroups = [updateData];
    await keysApi.updateGroupsOrder(updatedGroups);
    message.success("节点取消归档成功");
    emit("unarchived", updateData);
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
