<script setup lang="ts">
import { ref, computed, nextTick } from "vue";
import type { Group } from "@/types/models";
import { keysApi } from "@/api/keys";
import { NCard, NButton, NSpace, NTag, NInput, useMessage, NTooltip } from "naive-ui";

const props = defineProps<{ group: Group | null }>();
const emit = defineEmits(["refresh"]);

const message = useMessage();

const isEditing = ref(false);
const editInput = ref("");
const isLoading = ref(false);
const inputRef = ref<HTMLInputElement | null>(null);

const ccrModels = computed(() => props.group?.ccr_models || []);

async function enterEditMode() {
  if (!props.group) {
    return;
  }
  editInput.value = ccrModels.value.join(",");
  isEditing.value = true;
  await nextTick();
  inputRef.value?.focus();
}

function cancelEdit() {
  isEditing.value = false;
}

async function saveChanges() {
  if (!props.group || typeof props.group.id !== 'number') {
    message.error("未选择有效的分组");
    return;
  }

  const groupId = props.group.id;

  isLoading.value = true;
  try {
    await keysApi.updateGroupCCRModels(groupId, editInput.value);
    message.success("自定义模型已更新");
    isEditing.value = false;
    setTimeout(() => {
      emit("refresh");
    }, 300);
  } catch (error) {
    message.error("更新失败, 请重试");
    console.error(error);
  } finally {
    isLoading.value = false;
  }
}

function copyToClipboard(text: string) {
  navigator.clipboard
    .writeText(text)
    .then(() => {
      message.success("已复制到剪切板");
    })
    .catch(() => {
      message.error("复制失败");
    });
}
</script>

<template>
  <div class="ccr-settings-card-wrapper">
    <n-card v-if="group" title="自定义模型 (CCR)" :bordered="false" size="small">
      <div v-if="!isEditing" class="display-mode">
        <n-button type="primary" ghost size="small" @click="enterEditMode">编辑</n-button>
        <n-space class="tags-space" :size="8">
          <n-tooltip v-for="model in ccrModels" :key="model" trigger="hover">
            <template #trigger>
              <n-tag
                type="info"
                round
                :bordered="false"
                style="cursor: pointer"
                @click="copyToClipboard(`/model ${group?.name || ''},${model}`)"
              >
                {{ model }}
              </n-tag>
            </template>
            点击复制模型切换命令
          </n-tooltip>
          <span v-if="ccrModels.length === 0" class="no-models-text">未设置自定义模型</span>
        </n-space>
      </div>
      <div v-else class="edit-mode">
        <n-button type="primary" size="small" @click="saveChanges" :loading="isLoading">
          确定
        </n-button>
        <n-input
          ref="inputRef"
          v-model:value="editInput"
          placeholder="输入模型, 以英文逗号 , 分隔"
          @blur="cancelEdit"
          @keydown.enter.prevent="saveChanges"
        />
      </div>
    </n-card>
  </div>
</template>

<style scoped>
.ccr-settings-card-wrapper {
  flex-shrink: 0;
}

.display-mode,
.edit-mode {
  display: flex;
  align-items: center;
  gap: 16px;
}

.tags-space {
  flex-wrap: wrap;
}

.no-models-text {
  color: #999;
  font-size: 13px;
}
</style>
