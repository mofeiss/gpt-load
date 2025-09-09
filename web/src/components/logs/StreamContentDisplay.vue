<script setup lang="ts">
import type { StreamContent } from "@/types/models";
import { copy } from "@/utils/clipboard";
import { CopyOutline } from "@vicons/ionicons5";
import { NButton, NCard, NGrid, NIcon, useMessage } from "naive-ui";

interface Props {
  streamContent: StreamContent | undefined;
  rawContent: string;
}

const props = defineProps<Props>();
const message = useMessage();

// 复制功能
const copyContent = async (content: string, type: string) => {
  const success = await copy(content);
  if (success) {
    message.success(`${type}已复制到剪贴板`);
  } else {
    message.error(`复制${type}失败`);
  }
};

// 格式化流式内容为 Markdown 纯文本
const formatStreamContentAsMarkdown = (content: StreamContent) => {
  let result = "";

  if (content.thinking_chain) {
    result += `**思维链**\n\`\`\`\n${content.thinking_chain}\n\`\`\`\n\n`;
  }

  if (content.text_messages) {
    result += `**文本消息**\n\`\`\`\n${content.text_messages}\n\`\`\`\n\n`;
  }

  if (content.tool_calls) {
    result += `**工具调用**\n\`\`\`\n${content.tool_calls}\n\`\`\`\n`;
  }

  return result.trim();
};
</script>

<template>
  <div v-if="props.streamContent || props.rawContent" class="stream-display">
    <n-grid :cols="2" :x-gap="12">
      <!-- 左侧：原文内容 -->
      <n-card title="原文内容" size="small" class="raw-content-card">
        <template #header-extra>
          <n-button size="tiny" text @click="copyContent(props.rawContent, '原文内容')">
            <template #icon>
              <n-icon :component="CopyOutline" />
            </template>
          </n-button>
        </template>
        <div class="content-display raw-content">
          {{ props.rawContent }}
        </div>
      </n-card>

      <!-- 右侧：解析后内容 -->
      <n-card title="解析后内容" size="small" class="parsed-content-card">
        <template #header-extra>
          <n-button
            v-if="props.streamContent"
            size="tiny"
            text
            @click="copyContent(formatStreamContentAsMarkdown(props.streamContent), '解析后内容')"
          >
            <template #icon>
              <n-icon :component="CopyOutline" />
            </template>
          </n-button>
        </template>
        <div class="content-display parsed-content">
          <div v-if="props.streamContent">
            {{ formatStreamContentAsMarkdown(props.streamContent) }}
          </div>
          <div v-else class="no-parsed-content">暂无解析内容</div>
        </div>
      </n-card>
    </n-grid>
  </div>
</template>

<style scoped>
.stream-display {
  width: 100%;
}

.raw-content-card,
.parsed-content-card {
  height: 400px;
}

.content-display {
  height: 320px;
  overflow-y: auto;
  font-family: "Courier New", Consolas, monospace;
  font-size: 12px;
  line-height: 1.4;
  word-break: break-all;
  white-space: pre-wrap;
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 4px;
  padding: 12px;
}

.raw-content {
  color: #495057;
}

.parsed-content {
  color: #212529;
}

.no-parsed-content {
  text-align: center;
  color: #6c757d;
  font-style: italic;
  padding: 20px;
}

/* 滚动条样式 */
.content-display::-webkit-scrollbar {
  width: 8px;
}

.content-display::-webkit-scrollbar-track {
  background: #f1f1f1;
  border-radius: 4px;
}

.content-display::-webkit-scrollbar-thumb {
  background: #c1c1c1;
  border-radius: 4px;
}

.content-display::-webkit-scrollbar-thumb:hover {
  background: #a8a8a8;
}
</style>
