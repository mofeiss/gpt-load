<script setup lang="ts">
import { keysApi } from "@/api/keys";
import type { Group } from "@/types/models";
import { appState } from "@/utils/app-state";
import { getGroupDisplayName } from "@/utils/display";
import { CloseOutline, CopyOutline } from "@vicons/ionicons5";
import {
  NButton,
  NCard,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NModal,
  NRadio,
  NRadioGroup,
  NCheckbox,
  useMessage,
} from "naive-ui";
import { computed, ref, watchEffect } from "vue";

interface Props {
  show: boolean;
  sourceGroup: Group | null;
}

interface Emits {
  (e: "update:show", value: boolean): void;
  (e: "success", group: Group): void;
}

interface CopyFormData {
  newGroupName: string;
  copyKeys: "all" | "valid_only" | "none";
  copyDescription: boolean;
  copyCodeSnippet: boolean;
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

const message = useMessage();
const loading = ref(false);

const formData = ref<CopyFormData>({
  newGroupName: "",
  copyKeys: "all",
  copyDescription: true,
  copyCodeSnippet: true,
});

const modalVisible = computed({
  get: () => props.show,
  set: (value: boolean) => emit("update:show", value),
});

// Watch for show prop changes to reset form
watchEffect(() => {
  if (props.show) {
    resetForm();
  }
});

function resetForm() {
  const baseName = props.sourceGroup?.name || "";
  formData.value = {
    newGroupName: `${baseName}_copy`,
    copyKeys: "all",
    copyDescription: true,
    copyCodeSnippet: true,
  };
}

async function handleCopy() {
  if (!props.sourceGroup?.id) {
    message.error("源分组不存在");
    return;
  }

  if (!formData.value.newGroupName.trim()) {
    message.error("请输入分组名称");
    return;
  }

  loading.value = true;
  try {
    const copyData = {
      new_group_name: formData.value.newGroupName.trim(),
      copy_keys: formData.value.copyKeys,
      copy_description: formData.value.copyDescription,
      copy_code_snippet: formData.value.copyCodeSnippet,
    };
    const result = await keysApi.copyGroup(props.sourceGroup.id, copyData);

    // Show appropriate success message based on copy strategy
    if (formData.value.copyKeys !== "none") {
      message.success(
        `复制成功！已创建新分组 "${result.group.display_name || result.group.name}"，密钥正在后台导入，请稍后查看进度`
      );
      // Trigger task polling to show import progress
      appState.taskPollingTrigger++;
    } else {
      message.success(`复制成功！已创建新分组 "${result.group.display_name || result.group.name}"`);
    }

    emit("success", result.group);
    modalVisible.value = false;
  } catch (error) {
    console.error(error);
    message.error("复制分组失败，请稍后重试");
  } finally {
    loading.value = false;
  }
}

function handleCancel() {
  modalVisible.value = false;
}
</script>

<template>
  <n-modal :show="modalVisible" @update:show="handleCancel" class="group-copy-modal">
    <n-card
      class="group-copy-card"
      :title="`复制分组 - ${sourceGroup ? getGroupDisplayName(sourceGroup) : ''}`"
      :bordered="false"
      size="huge"
      role="dialog"
      aria-modal="true"
    >
      <template #header-extra>
        <n-button quaternary circle @click="handleCancel">
          <template #icon>
            <n-icon :component="CloseOutline" />
          </template>
        </n-button>
      </template>

      <div class="modal-content">
        <n-form
          :model="formData"
          label-placement="left"
          label-width="100px"
          class="group-copy-form"
        >
          <!-- 分组名称输入 -->
          <n-form-item label="新分组名称" required>
            <n-input
              v-model:value="formData.newGroupName"
              placeholder="请输入新分组名称"
              :maxlength="30"
              show-count
            />
          </n-form-item>

          <!-- 密钥复制选项 -->
          <n-form-item label="密钥处理">
            <n-radio-group v-model:value="formData.copyKeys" name="copyKeys">
              <div class="radio-options">
                <n-radio value="all" class="radio-option">复制所有密钥</n-radio>
                <n-radio value="valid_only" class="radio-option">仅复制有效密钥</n-radio>
                <n-radio value="none" class="radio-option">不复制密钥</n-radio>
              </div>
            </n-radio-group>
          </n-form-item>

          <!-- 内容复制选项 -->
          <n-form-item label="内容选项">
            <div class="checkbox-options">
              <n-checkbox v-model:checked="formData.copyDescription" class="checkbox-option">
                复制描述卡片
              </n-checkbox>
              <n-checkbox v-model:checked="formData.copyCodeSnippet" class="checkbox-option">
                复制代码片段
              </n-checkbox>
            </div>
          </n-form-item>
        </n-form>
      </div>

      <template #footer>
        <div class="modal-actions">
          <n-button @click="handleCancel" :disabled="loading">取消</n-button>
          <n-button type="primary" @click="handleCopy" :loading="loading">
            <template #icon>
              <n-icon :component="CopyOutline" />
            </template>
            确认复制
          </n-button>
        </div>
      </template>
    </n-card>
  </n-modal>
</template>

<style scoped>
.group-copy-modal {
  width: 450px;
  max-width: 90vw;
  --n-color: rgba(255, 255, 255, 0.95);
}

.modal-content {
  padding: 0;
}

.copy-options {
  margin-bottom: 16px;
}

.radio-options {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.radio-option {
  margin: 0;
}

.checkbox-options {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.checkbox-option {
  margin: 0;
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

/* 增强表单样式 - 与GroupFormModal保持一致 */
:deep(.n-form-item-label) {
  font-weight: 500;
  color: #374151;
}

:deep(.n-button) {
  --n-border-radius: 8px;
}

:deep(.n-card-header) {
  border-bottom: 1px solid rgba(239, 239, 245, 0.8);
  padding: 10px 20px;
}

:deep(.n-card__content) {
  padding: 16px 20px;
}

:deep(.n-card__footer) {
  border-top: 1px solid rgba(239, 239, 245, 0.8);
  padding: 10px 15px;
}

:deep(.n-form-item-feedback-wrapper) {
  min-height: 10px;
}
</style>
