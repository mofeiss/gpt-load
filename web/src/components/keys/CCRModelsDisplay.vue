<script setup lang="ts">
import type { Group } from "@/types/models";
import { copy } from "@/utils/clipboard";
import { getGroupDisplayName } from "@/utils/display";
import { CopyOutline } from "@vicons/ionicons5";
import { NTag, NTooltip, useMessage } from "naive-ui";
import { computed } from "vue";

interface Props {
  group: Group | null;
}

const props = defineProps<Props>();
const message = useMessage();

// 计算属性来获取 CCR 模型列表
const ccrModels = computed(() => {
  return props.group?.ccr_models || [];
});

async function copyModelText(model: string) {
  if (!props.group) {
    return;
  }

  const text = `/model ${props.group.name},${model}`;
  const success = await copy(text);

  if (success) {
    message.success(`已复制: ${text}`, {
      duration: 3000,
    });
  } else {
    message.error("复制失败", {
      duration: 3000,
    });
  }
}
</script>

<template>
  <div v-if="ccrModels.length > 0" class="ccr-models-display">
    <div class="models-container">
      <n-tooltip v-for="model in ccrModels" :key="model" trigger="hover">
        <template #trigger>
          <n-tag
            :bordered="false"
            round
            type="info"
            size="medium"
            class="model-tag"
            @click="copyModelText(model)"
          >
            <template #icon>
              <n-icon :component="CopyOutline" />
            </template>
            {{ model }}
          </n-tag>
        </template>
        点击复制模型切换命令 "/model {{ props.group ? getGroupDisplayName(props.group) : "" }},{{
          model
        }}"
      </n-tooltip>
    </div>
  </div>
  <div v-else class="ccr-models-empty">
    <!-- 无模型时不显示任何内容 -->
  </div>
</template>

<style scoped>
.ccr-models-display {
  padding: 12px 16px;
  background: rgba(99, 102, 241, 0.05);
  border-radius: 8px;
  margin: 0 16px;
}

.models-container {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.model-tag {
  cursor: pointer;
  transition: all 0.2s ease;
  user-select: none;
}

.model-tag:hover {
  transform: translateY(-1px);
  box-shadow: 0 2px 4px rgba(99, 102, 241, 0.3);
}

.ccr-models-empty {
  height: 0;
  margin: 0;
}
</style>
