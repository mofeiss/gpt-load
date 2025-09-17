<script setup lang="ts">
import type { Category } from "@/types/models";
import { categoriesApi } from "@/api/categories";
import { NDropdown, useDialog, useMessage } from "naive-ui";
import { computed, ref } from "vue";

interface Props {
  category: Category;
  show: boolean;
  x: number;
  y: number;
}

interface Emits {
  (e: "update:show", show: boolean): void;
  (e: "edit", category: Category): void;
  (e: "category-updated"): void;
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

const menuOptions = computed(() => [
  {
    label: "重命名分类",
    key: "rename",
  },
  {
    label: "",
    key: "divider",
    type: "divider",
  },
  {
    label: "解散分类",
    key: "delete",
    style: {
      color: "red",
    },
  },
]);

function closeDropdown() {
  showDropdown.value = false;
}

async function handleMenuSelect(key: string) {
  closeDropdown();

  switch (key) {
    case "rename":
      emit("edit", props.category);
      break;
    case "delete":
      dialog.warning({
        title: "确认解散分类",
        content: `确定要解散分类 "${props.category.name}" 吗？分类下的所有节点将移动到归档中。`,
        positiveText: "确定解散",
        negativeText: "取消",
        onPositiveClick: async () => {
          try {
            isProcessing.value = true;
            await categoriesApi.deleteCategory(props.category.id);
            message.success("分类解散成功");
            emit("category-updated");
          } catch (error) {
            console.error("解散分类失败:", error);
            message.error("解散分类失败");
          } finally {
            isProcessing.value = false;
          }
        },
      });
      break;
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
