<script setup lang="ts">
import type { Category } from "@/types/models";
import { categoriesApi } from "@/api/categories";
import { NModal, NForm, NFormItem, NInput, NButton, useMessage } from "naive-ui";
import { ref, watch, computed } from "vue";

interface Props {
  show: boolean;
  category?: Category | null;
}

interface Emits {
  (e: "update:show", show: boolean): void;
  (e: "success", category: Category): void;
}

const props = withDefaults(defineProps<Props>(), {
  category: null,
});

const emit = defineEmits<Emits>();

const message = useMessage();

const showModal = computed({
  get: () => props.show,
  set: value => emit("update:show", value),
});

const isEdit = computed(() => !!props.category?.id);
const title = computed(() => (isEdit.value ? "编辑分类" : "新建分类"));

// 表单数据
const formData = ref({
  name: "",
});

// 加载状态
const loading = ref(false);

// 监听 category prop 变化，更新表单数据
watch(
  () => props.category,
  newCategory => {
    if (newCategory) {
      formData.value.name = newCategory.name;
    } else {
      formData.value.name = "";
    }
  },
  { immediate: true }
);

// 重置表单
function resetForm() {
  formData.value.name = "";
}

// 关闭模态框
function handleClose() {
  showModal.value = false;
  setTimeout(() => {
    resetForm();
  }, 200);
}

// 提交表单
async function handleSubmit() {
  if (!formData.value.name.trim()) {
    message.error("分类名称不能为空");
    return;
  }

  if (formData.value.name.length > 50) {
    message.error("分类名称长度不能超过50个字符");
    return;
  }

  try {
    loading.value = true;
    let result: Category;

    if (isEdit.value && props.category) {
      // 编辑分类
      result = await categoriesApi.updateCategory(props.category.id, {
        name: formData.value.name.trim(),
      });
      message.success("分类更新成功");
    } else {
      // 新建分类
      result = await categoriesApi.createCategory({
        name: formData.value.name.trim(),
      });
      message.success("分类创建成功");
    }

    emit("success", result);
    handleClose();
  } catch (error) {
    console.error("操作分类失败:", error);
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <n-modal v-model:show="showModal" preset="card" :title="title" style="width: 400px">
    <n-form ref="formRef" :model="formData" label-placement="left" label-width="80px">
      <n-form-item label="分类名称" path="name">
        <n-input
          v-model:value="formData.name"
          placeholder="请输入分类名称"
          maxlength="50"
          show-count
          @keyup.enter="handleSubmit"
        />
      </n-form-item>
    </n-form>

    <template #footer>
      <div class="modal-footer">
        <n-button @click="handleClose">取消</n-button>
        <n-button type="primary" :loading="loading" @click="handleSubmit">确定</n-button>
      </div>
    </template>
  </n-modal>
</template>

<style scoped>
.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}
</style>
