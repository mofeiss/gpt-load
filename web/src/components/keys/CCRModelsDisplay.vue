<script setup lang="ts">
import { copy } from "@/utils/clipboard";
import { getGroupDisplayName } from "@/utils/display";
import type { Group } from "@/types/models";
import { CopyOutline } from "@vicons/ionicons5";
import { NTag, NTooltip, useMessage } from "naive-ui";

interface Props {
  group: Group | null;
}

const props = defineProps<Props>();
const message = useMessage();

async function copyModelText(model: string) {
  if (!props.group) {
    return;
  }

  const text = `${props.group.name},${model}`;
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
  <div v-if="group?.ccr_models && group.ccr_models.length > 0" class="ccr-models-display">
    <div class="models-container">
      <n-tooltip v-for="model in group.ccr_models" :key="model" trigger="hover">
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
        点击复制 "{{ getGroupDisplayName(group) }},{{ model }}"
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
