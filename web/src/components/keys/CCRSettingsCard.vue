<script setup lang="ts">
import { ref, computed } from "vue";
import type { Group } from "@/types/models";
import { keysApi } from "@/api/keys";
import {
  NCard,
  NButton,
  NSpace,
  NTag,
  NInput,
  useMessage,
  NTooltip,
  NModal,
  NSelect,
  NIcon,
} from "naive-ui";
import { CheckmarkOutline, CodeSlash } from "@vicons/ionicons5";

const props = defineProps<{ group: Group | null }>();
const emit = defineEmits(["refresh"]);

const message = useMessage();

const isLoading = ref(false);
const showJsonModal = ref(false);
const jsonInput = ref("");
const selectedDefaultModel = ref<string | null>(null);
const newModelInput = ref("");

const ccrModels = computed(() => props.group?.ccr_models || []);

// 从当前JSON输入中提取模型列表，用于模态窗口显示
const currentModelsFromJson = computed(() => {
  return extractModelsFromJson(jsonInput.value);
});

// 获取默认JSON结构
function getDefaultCodeSnippet(): string {
  if (!props.group) return "";

  return JSON.stringify(
    {
      name: props.group.name,
      api_base_url: `http://localhost:3001/proxy/${props.group.name}/v1/chat/completions`,
      api_key: "your-api-key-here",
      models: [],
      transformer: {
        use: [
          [
            "maxtoken",
            {
              max_tokens: 65535,
            },
          ],
        ],
      },
    },
    null,
    2
  );
}

// 从JSON中提取模型列表
function extractModelsFromJson(jsonStr: string): string[] {
  if (!jsonStr.trim()) return [];

  try {
    const parsed = JSON.parse(jsonStr);
    return Array.isArray(parsed.models) ? parsed.models : [];
  } catch {
    return [];
  }
}

async function openJsonModal() {
  if (!props.group) {
    return;
  }
  // 显示完整的片段JSON，而不是模型字符串
  jsonInput.value = props.group.code_snippet || getDefaultCodeSnippet();
  showJsonModal.value = true;
}

function closeJsonModal() {
  showJsonModal.value = false;
}

// 应用到CCR - TODO: 实现具体逻辑
function applyToCCR() {
  // TODO: 实现应用到CCR的逻辑
  message.info("应用到CCR功能待实现");
}

// 设为CCR默认模型 - TODO: 实现具体逻辑
function setDefaultModel() {
  if (!selectedDefaultModel.value) {
    message.warning("请先选择一个模型");
    return;
  }
  // TODO: 实现设为默认模型的逻辑
  message.info(`设为CCR默认模型功能待实现: ${selectedDefaultModel.value}`);
}

async function saveJsonChanges() {
  if (!props.group || typeof props.group.id !== "number") {
    message.error("未选择有效的分组");
    return;
  }

  const groupId = props.group.id;

  // 验证JSON格式
  try {
    JSON.parse(jsonInput.value);
  } catch (error) {
    message.error("JSON格式错误，请检查语法");
    return;
  }

  isLoading.value = true;
  try {
    // 使用通用的分组更新接口，更新code_snippet字段
    await keysApi.updateGroup(groupId, { code_snippet: jsonInput.value });
    message.success("代码片段已更新");
    showJsonModal.value = false;
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

// 快速添加模型
function addQuickModel() {
  if (!newModelInput.value.trim()) {
    message.warning("请输入模型名称");
    return;
  }

  try {
    // 解析现有JSON
    let jsonObj;
    try {
      jsonObj = JSON.parse(jsonInput.value);
    } catch {
      // 如果解析失败，使用默认结构
      jsonObj = JSON.parse(getDefaultCodeSnippet());
    }

    // 确保models数组存在
    if (!Array.isArray(jsonObj.models)) {
      jsonObj.models = [];
    }

    const newModel = newModelInput.value.trim();
    if (jsonObj.models.includes(newModel)) {
      message.warning("模型已存在");
      return;
    }

    // 添加新模型
    jsonObj.models.push(newModel);

    // 更新JSON字符串
    jsonInput.value = JSON.stringify(jsonObj, null, 2);
    newModelInput.value = "";
    message.success("模型已添加到配置中");
  } catch (error) {
    message.error("添加模型失败，请检查JSON格式");
    console.error(error);
  }
}

// 从JSON中删除指定模型
function removeModelFromJson(modelToRemove: string) {
  try {
    let jsonObj;
    try {
      jsonObj = JSON.parse(jsonInput.value);
    } catch {
      message.error("JSON格式错误，无法删除模型");
      return;
    }

    // 确保models数组存在
    if (!Array.isArray(jsonObj.models)) {
      message.warning("配置中没有模型数组");
      return;
    }

    // 删除指定模型
    const modelIndex = jsonObj.models.indexOf(modelToRemove);
    if (modelIndex === -1) {
      message.warning("模型不存在");
      return;
    }

    jsonObj.models.splice(modelIndex, 1);

    // 更新JSON字符串
    jsonInput.value = JSON.stringify(jsonObj, null, 2);
    message.success(`已删除模型: ${modelToRemove}`);
  } catch (error) {
    message.error("删除模型失败，请检查JSON格式");
    console.error(error);
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
      <div class="display-mode">
        <n-button
          type="primary"
          size="small"
          @click="applyToCCR"
          :disabled="ccrModels.length === 0"
        >
          <template #icon>
            <n-icon :component="CheckmarkOutline" />
          </template>
          应用到ccr
        </n-button>

        <n-select
          v-model:value="selectedDefaultModel"
          placeholder="选择模型"
          size="small"
          style="width: 120px"
          :options="ccrModels.map(model => ({ label: model, value: model }))"
          :disabled="ccrModels.length === 0"
        />
        <n-button
          type="primary"
          ghost
          size="small"
          @click="setDefaultModel"
          :disabled="!selectedDefaultModel"
        >
          设为ccr默认模型
        </n-button>

        <n-button type="info" ghost size="small" @click="openJsonModal">
          <template #icon>
            <n-icon :component="CodeSlash" />
          </template>
          JSON
        </n-button>

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
    </n-card>

    <!-- JSON 编辑模态窗口 -->
    <n-modal
      v-model:show="showJsonModal"
      preset="card"
      title="编辑 CCR 模型配置"
      style="width: 600px"
    >
      <div class="json-editor">
        <n-input
          v-model:value="jsonInput"
          type="textarea"
          placeholder='请输入完整的JSON配置，例如:&#10;{&#10;  "name": "group-name",&#10;  "api_base_url": "http://localhost:3001/proxy/group-name/v1/chat/completions",&#10;  "api_key": "your-api-key",&#10;  "models": ["gpt-4", "claude-3"],&#10;  "transformer": { ... }&#10;}'
          :rows="8"
          style="margin-bottom: 16px"
        />

        <!-- 快速添加模型 -->
        <div class="quick-add-section">
          <div class="quick-add-input">
            <n-input
              v-model:value="newModelInput"
              placeholder="输入模型名称"
              @keydown.enter.prevent="addQuickModel"
              style="flex: 1; margin-right: 8px"
            />
            <n-button type="primary" @click="addQuickModel">加入</n-button>
          </div>
        </div>

        <!-- 已添加的模型展示 -->
        <div class="current-models" v-if="currentModelsFromJson.length > 0">
          <div class="models-label">已添加的模型:</div>
          <n-space :size="8">
            <n-tag
              v-for="model in currentModelsFromJson"
              :key="model"
              type="info"
              round
              :bordered="false"
              closable
              @close="removeModelFromJson(model)"
              class="model-tag-deletable"
            >
              {{ model }}
            </n-tag>
          </n-space>
        </div>
      </div>

      <template #footer>
        <div style="display: flex; justify-content: flex-end; gap: 8px">
          <n-button @click="closeJsonModal">取消</n-button>
          <n-button type="primary" @click="saveJsonChanges" :loading="isLoading">保存</n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.ccr-settings-card-wrapper {
  flex-shrink: 0;
}

.display-mode {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.tags-space {
  flex-wrap: wrap;
}

.no-models-text {
  color: #999;
  font-size: 13px;
}

.json-editor {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.quick-add-section {
  border-top: 1px solid #e8e8e8;
  padding-top: 16px;
}

.quick-add-input {
  display: flex;
  align-items: center;
  margin-bottom: 12px;
}

.current-models {
  margin-top: 16px;
}

.models-label {
  font-size: 14px;
  font-weight: 500;
  margin-bottom: 8px;
  color: #333;
}

.model-tag-deletable {
  cursor: pointer;
  transition: all 0.2s ease;
}

.model-tag-deletable:hover {
  transform: translateY(-1px);
  box-shadow: 0 2px 4px rgba(99, 102, 241, 0.3);
}
</style>
