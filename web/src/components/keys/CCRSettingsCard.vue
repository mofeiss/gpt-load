<script setup lang="ts">
import { ref, computed, watch, onMounted, nextTick } from "vue";
import type { Group } from "@/types/models";
import { keysApi } from "@/api/keys";
import { useAuthKey } from "@/services/auth";
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
  NCheckbox,
  NSpin,
} from "naive-ui";
import {
  CodeSlash,
  TrashBinOutline,
  CloudUploadOutline,
  SyncOutline,
  DownloadOutline,
} from "@vicons/ionicons5";
import axios from "axios";

const props = defineProps<{ group: Group | null }>();
const emit = defineEmits(["refresh"]);

const message = useMessage();

// 获取全局认证密钥
const authKey = useAuthKey();

const isLoading = ref(false);
const showJsonModal = ref(false);
const jsonInput = ref("");
const selectedDefaultModel = ref<string | null>(null);
const newModelInput = ref("");
const jsonTextarea = ref<InstanceType<typeof NInput> | null>(null);

// 快速模型标签列表
const quickModels = [
  "claude-sonnet-4-20250514",
  "claude-3-5-haiku-20241022",
  "claude-4.1-opus",
  "gemini-2.5-pro",
  "gemini-2.5-flash",
  "glm-4.5",
  "glm-4.5-air",
  "claude-opus-4-1-20250805",
  "claude-opus-4-20250514",
  "claude-3-5-sonnet-20241022",
  "claude-3-7-sonnet-20250219",
  "claude-4-sonnet",
  "claude-4-sonnet-think",
];

// CCR 相关状态
interface CCRConfig {
  Providers: Array<Record<string, unknown>>;
  Router: {
    default: string;
    think: string;
    longContext: string;
    [key: string]: unknown;
  };
  [key: string]: unknown;
}

const ccrConfig = ref<CCRConfig | null>(null);
const ccrDetectionStatus = ref<string>(""); // 'add'/'exists'
const showUpdateButton = ref(false); // 是否显示更新按钮
const isDetecting = ref(false);
const setAsDefault = ref(false);
const isOperating = ref(false);

const ccrModels = computed(() => props.group?.ccr_models || []);

// 从当前JSON输入中提取模型列表，用于模态窗口显示
const currentModelsFromJson = computed(() => {
  return extractModelsFromJson(jsonInput.value);
});

// 获取默认JSON结构
function getDefaultCodeSnippet(): string {
  if (!props.group) {
    return "";
  }

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
  if (!jsonStr.trim()) {
    return [];
  }

  try {
    const parsed = JSON.parse(jsonStr);
    return Array.isArray(parsed.models) ? parsed.models : [];
  } catch {
    return [];
  }
}

// 创建OpenAI模板
function createOpenAITemplate(): string {
  const groupName = props.group?.name || "分组名";
  const apiKey = authKey.value || "全局key";
  return JSON.stringify(
    {
      name: groupName,
      api_base_url: `http://localhost:3001/proxy/${groupName}/v1/chat/completions`,
      api_key: apiKey,
      models: ["claude-sonnet-4-20250514"],
      transformer: {
        use: [
          [
            "maxtoken",
            {
              max_tokens: 65535,
            },
          ],
        ],
        "claude-sonnet-4-20250514": {
          use: ["reasoning"],
        },
      },
    },
    null,
    2
  );
}

// 创建Anthropic模板
function createAnthropicTemplate(): string {
  const groupName = props.group?.name || "分组名";
  const apiKey = authKey.value || "全局key";
  return JSON.stringify(
    {
      name: groupName,
      api_base_url: `http://localhost:3001/proxy/${groupName}/v1/messages?beta=true`,
      api_key: apiKey,
      models: ["claude-sonnet-4-20250514"],
      transformer: {
        use: ["Anthropic"],
      },
    },
    null,
    2
  );
}

// 创建Gemini模板
function createGeminiTemplate(): string {
  const groupName = props.group?.name || "分组名";
  const apiKey = authKey.value || "全局key";
  return JSON.stringify(
    {
      name: groupName,
      api_base_url: `http://localhost:3001/proxy/${groupName}/v1beta/models/`,
      api_key: apiKey,
      models: ["gemini-2.5-pro", "gemini-2.5-flash"],
      transformer: {
        use: ["gemini"],
      },
    },
    null,
    2
  );
}

// 创建默认JSON配置（根据渠道类型选择对应模板）
async function createDefaultJsonConfig() {
  if (!props.group || typeof props.group.id !== "number") {
    message.error("未选择有效的分组");
    return;
  }

  const groupId = props.group.id;
  const channelType = props.group.channel_type;

  // 根据渠道类型选择对应的模板
  let defaultConfig: string;
  switch (channelType) {
    case "openai":
      defaultConfig = createOpenAITemplate();
      break;
    case "anthropic":
      defaultConfig = createAnthropicTemplate();
      break;
    case "gemini":
      defaultConfig = createGeminiTemplate();
      break;
    default:
      defaultConfig = createOpenAITemplate(); // 默认使用 OpenAI 模板
      break;
  }

  isLoading.value = true;
  try {
    await keysApi.updateGroup(groupId, { code_snippet: defaultConfig });
    message.success(`已创建${channelType.toUpperCase()}模板配置`);

    // 设置JSON内容并打开模态窗口
    jsonInput.value = defaultConfig;
    showJsonModal.value = true;

    // 等待模态窗口渲染后聚焦到编辑框
    await nextTick();
    if (jsonTextarea.value) {
      jsonTextarea.value.focus();
    }

    setTimeout(() => {
      emit("refresh");
    }, 300);
  } catch (error) {
    message.error("创建默认配置失败");
    console.error(error);
  } finally {
    isLoading.value = false;
  }
}

// 设置模板到JSON输入框
function setTemplate(templateType: "openai" | "anthropic" | "gemini") {
  switch (templateType) {
    case "openai":
      jsonInput.value = createOpenAITemplate();
      break;
    case "anthropic":
      jsonInput.value = createAnthropicTemplate();
      break;
    case "gemini":
      jsonInput.value = createGeminiTemplate();
      break;
  }
  message.success(`已加载${templateType.toUpperCase()}模板`);
}

// 清空JSON输入
function clearJsonInput() {
  jsonInput.value = "";
  message.success("已清空输入内容");
}

// 从快速标签添加模型
function addQuickModelFromTag(modelName: string) {
  try {
    let jsonObj;
    try {
      jsonObj = JSON.parse(jsonInput.value);
    } catch {
      jsonObj = {
        name: props.group?.name || "",
        api_base_url: `http://localhost:3001/proxy/${props.group?.name || ""}/v1/chat/completions`,
        api_key: "your-api-key-here",
        models: [],
      };
    }

    if (!Array.isArray(jsonObj.models)) {
      jsonObj.models = [];
    }

    if (jsonObj.models.includes(modelName)) {
      message.warning("模型已存在");
      return;
    }

    jsonObj.models.push(modelName);

    if (shouldAddReasoning()) {
      if (!jsonObj.transformer) {
        jsonObj.transformer = {
          use: [
            [
              "maxtoken",
              {
                max_tokens: 65535,
              },
            ],
          ],
        };
      }

      jsonObj.transformer[modelName] = {
        use: ["reasoning"],
      };
    }

    jsonInput.value = JSON.stringify(jsonObj, null, 2);

    const successMessage = shouldAddReasoning()
      ? `已添加模型: ${modelName}（包含reasoning配置）`
      : `已添加模型: ${modelName}`;
    message.success(successMessage);
  } catch (error) {
    message.error("添加模型失败，请检查JSON格式");
    console.error(error);
  }
}

async function openJsonModal() {
  if (!props.group) {
    return;
  }
  // 直接使用数据库中的 code_snippet，如果为空就是空，不生成默认内容
  jsonInput.value = props.group.code_snippet || "";
  showJsonModal.value = true;
}

function closeJsonModal() {
  showJsonModal.value = false;
}

// CCR 配置检测
async function detectCCRStatus() {
  if (!props.group || !props.group.code_snippet) {
    ccrDetectionStatus.value = "";
    showUpdateButton.value = false;
    return;
  }

  isDetecting.value = true;
  try {
    const response = await axios.get("http://127.0.0.1:3456/api/config", {
      headers: {
        "X-API-Key": "1968121800",
      },
    });
    ccrConfig.value = response.data;

    const providers = response.data.Providers || [];
    const currentGroup = JSON.parse(props.group.code_snippet);
    const existingProvider = providers.find(
      (p: Record<string, unknown>) => p.name === currentGroup.name
    );

    if (!existingProvider) {
      ccrDetectionStatus.value = "add";
      showUpdateButton.value = false;
    } else {
      ccrDetectionStatus.value = "exists";
      // 比较数据是否一致，决定是否显示更新按钮
      const existingDataStr = JSON.stringify(existingProvider);
      const currentDataStr = JSON.stringify(currentGroup);
      showUpdateButton.value = existingDataStr !== currentDataStr;
    }
  } catch (error) {
    console.error("检测 CCR 状态失败:", error);
    message.error("检测 CCR 状态失败");
    ccrDetectionStatus.value = "";
    showUpdateButton.value = false;
  } finally {
    isDetecting.value = false;
  }
}

// 添加到CCR
async function addToCCR() {
  if (!props.group || !props.group.code_snippet || !selectedDefaultModel.value) {
    message.warning("请先选择模型");
    return;
  }

  isOperating.value = true;
  try {
    const currentGroup = JSON.parse(props.group.code_snippet);
    const newConfig = { ...ccrConfig.value } as CCRConfig;

    if (!newConfig.Providers) {
      newConfig.Providers = [];
    }

    newConfig.Providers.push(currentGroup);

    if (setAsDefault.value && newConfig.Router) {
      const routerKey = `${currentGroup.name},${selectedDefaultModel.value}`;
      newConfig.Router.default = routerKey;
      newConfig.Router.think = routerKey;
      newConfig.Router.longContext = routerKey;
    }

    await axios.post("http://127.0.0.1:3456/api/config", newConfig, {
      headers: {
        "Content-Type": "application/json",
        "X-API-Key": "1968121800",
      },
    });

    await axios.post(
      "http://127.0.0.1:3456/api/restart",
      {},
      {
        headers: {
          "Content-Type": "application/json",
          "X-API-Key": "1968121800",
        },
      }
    );

    message.success("成功添加到 CCR 并重启服务");
    await detectCCRStatus();
  } catch (_error) {
    console.error("添加到 CCR 失败:", _error);
    message.error("添加到 CCR 失败");
  } finally {
    isOperating.value = false;
  }
}

// 更新CCR数据
async function updateCCR() {
  if (!props.group || !props.group.code_snippet || !selectedDefaultModel.value) {
    message.warning("请先选择模型");
    return;
  }

  isOperating.value = true;
  try {
    const currentGroup = JSON.parse(props.group.code_snippet);
    const newConfig = { ...ccrConfig.value } as CCRConfig;

    if (newConfig.Providers) {
      const providerIndex = newConfig.Providers.findIndex(
        (p: Record<string, unknown>) => p.name === currentGroup.name
      );
      if (providerIndex !== -1) {
        newConfig.Providers[providerIndex] = currentGroup;
      }
    }

    if (setAsDefault.value && newConfig.Router) {
      const routerKey = `${currentGroup.name},${selectedDefaultModel.value}`;
      newConfig.Router.default = routerKey;
      newConfig.Router.think = routerKey;
      newConfig.Router.longContext = routerKey;
    }

    await axios.post("http://127.0.0.1:3456/api/config", newConfig, {
      headers: {
        "Content-Type": "application/json",
        "X-API-Key": "1968121800",
      },
    });

    await axios.post(
      "http://127.0.0.1:3456/api/restart",
      {},
      {
        headers: {
          "Content-Type": "application/json",
          "X-API-Key": "1968121800",
        },
      }
    );

    message.success("成功更新 CCR 数据并重启服务");
    await detectCCRStatus();
  } catch (_error) {
    console.error("更新 CCR 数据失败:", _error);
    message.error("更新 CCR 数据失败");
  } finally {
    isOperating.value = false;
  }
}

// 从CCR移除
async function removeFromCCR() {
  if (!props.group || !props.group.code_snippet) {
    return;
  }

  isOperating.value = true;
  try {
    const currentGroup = JSON.parse(props.group.code_snippet);
    const newConfig = { ...ccrConfig.value } as CCRConfig;

    if (newConfig.Providers) {
      newConfig.Providers = newConfig.Providers.filter(
        (p: Record<string, unknown>) => p.name !== currentGroup.name
      );
    }

    await axios.post("http://127.0.0.1:3456/api/config", newConfig, {
      headers: {
        "Content-Type": "application/json",
        "X-API-Key": "1968121800",
      },
    });

    await axios.post(
      "http://127.0.0.1:3456/api/restart",
      {},
      {
        headers: {
          "Content-Type": "application/json",
          "X-API-Key": "1968121800",
        },
      }
    );

    message.success("成功从 CCR 移除并重启服务");
    await detectCCRStatus();
  } catch (_error) {
    console.error("从 CCR 移除失败:", _error);
    message.error("从 CCR 移除失败");
  } finally {
    isOperating.value = false;
  }
}

async function saveJsonChanges() {
  if (!props.group || typeof props.group.id !== "number") {
    message.error("未选择有效的分组");
    return;
  }

  const groupId = props.group.id;

  // 验证JSON格式（允许空内容）
  if (jsonInput.value.trim() !== "") {
    try {
      JSON.parse(jsonInput.value);
    } catch (_error) {
      message.error("JSON格式错误，请检查语法");
      return;
    }
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
  } catch (_error) {
    message.error("更新失败, 请重试");
    console.error(_error);
  } finally {
    isLoading.value = false;
  }
}

// 判断是否需要添加reasoning配置（仅OpenAI兼容端点需要）
function shouldAddReasoning(): boolean {
  return props.group?.channel_type === "openai";
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
      // 如果解析失败，创建最小化的结构
      jsonObj = {
        name: props.group?.name || "",
        api_base_url: `http://localhost:3001/proxy/${props.group?.name || ""}/v1/chat/completions`,
        api_key: "your-api-key-here",
        models: [],
      };
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

    // 添加新模型到models数组
    jsonObj.models.push(newModel);

    // 只有OpenAI兼容端点才需要添加reasoning配置
    if (shouldAddReasoning()) {
      // 确保transformer对象存在
      if (!jsonObj.transformer) {
        jsonObj.transformer = {
          use: [
            [
              "maxtoken",
              {
                max_tokens: 65535,
              },
            ],
          ],
        };
      }

      // 为新模型添加reasoning配置
      jsonObj.transformer[newModel] = {
        use: ["reasoning"],
      };
    }

    // 更新JSON字符串
    jsonInput.value = JSON.stringify(jsonObj, null, 2);
    newModelInput.value = "";

    const successMessage = shouldAddReasoning()
      ? "模型已添加到配置中（包含reasoning配置）"
      : "模型已添加到配置中";
    message.success(successMessage);
  } catch (_error) {
    message.error("添加模型失败，请检查JSON格式");
    console.error(_error);
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

    // 从models数组移除
    jsonObj.models.splice(modelIndex, 1);

    // 只有OpenAI兼容端点才需要清理transformer配置
    if (shouldAddReasoning() && jsonObj.transformer && jsonObj.transformer[modelToRemove]) {
      delete jsonObj.transformer[modelToRemove];
    }

    // 更新JSON字符串
    jsonInput.value = JSON.stringify(jsonObj, null, 2);

    const successMessage = shouldAddReasoning()
      ? `已删除模型: ${modelToRemove}（包含transformer配置）`
      : `已删除模型: ${modelToRemove}`;
    message.success(successMessage);
  } catch (_error) {
    message.error("删除模型失败，请检查JSON格式");
    console.error(_error);
  }
}

// 从CCR拉取配置
async function pullFromCCR() {
  if (!props.group) {
    message.warning("未选择分组");
    return;
  }

  if (typeof props.group.id !== "number") {
    message.error("分组ID无效");
    return;
  }

  isOperating.value = true;
  try {
    // 重新获取最新的CCR配置
    const response = await axios.get("http://127.0.0.1:3456/api/config", {
      headers: {
        "X-API-Key": "1968121800",
      },
    });

    const latestConfig = response.data;
    const providers = latestConfig.Providers || [];
    const currentGroup = JSON.parse(props.group.code_snippet || "{}");
    const existingProvider = providers.find(
      (p: Record<string, unknown>) => p.name === currentGroup.name
    );

    if (!existingProvider) {
      message.warning("CCR中不存在此分组配置");
      return;
    }

    // 将CCR配置覆盖到JSON编辑器
    const newConfigJson = JSON.stringify(existingProvider, null, 2);
    jsonInput.value = newConfigJson;

    // 自动保存到数据库的code_snippet字段
    const groupId = props.group.id;
    await keysApi.updateGroup(groupId, { code_snippet: newConfigJson });

    message.success("已从CCR拉取最新配置数据并更新到片段");

    // 更新本地缓存的配置
    ccrConfig.value = latestConfig;

    // 通知父组件刷新数据
    setTimeout(() => {
      emit("refresh");
    }, 300);
  } catch (_error) {
    console.error("从CCR拉取配置失败:", _error);
    message.error("从CCR拉取配置失败");
  } finally {
    isOperating.value = false;
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

// 监听分组变化，自动更新选中的模型
watch(
  () => props.group,
  newGroup => {
    if (newGroup && ccrModels.value.length > 0) {
      selectedDefaultModel.value = ccrModels.value[0];
    } else {
      selectedDefaultModel.value = null;
    }
    detectCCRStatus();
  },
  { immediate: true }
);

// 组件挂载时检测状态
onMounted(() => {
  detectCCRStatus();
});
</script>

<template>
  <div class="ccr-settings-card-wrapper">
    <n-card v-if="group" title="自定义模型 (CCR)" :bordered="false" size="small">
      <div class="display-mode">
        <!-- 配置区域 -->
        <div class="config-section">
          <n-select
            v-model:value="selectedDefaultModel"
            placeholder="选择模型"
            size="small"
            style="width: 200px"
            :options="ccrModels.map(model => ({ label: model, value: model }))"
            :disabled="ccrModels.length === 0"
          />

          <n-checkbox
            v-model:checked="setAsDefault"
            :disabled="!selectedDefaultModel || ccrDetectionStatus === ''"
          >
            设为默认
          </n-checkbox>
        </div>

        <!-- 操作区域 -->
        <div class="action-section">
          <n-spin :show="isDetecting" size="small">
            <div class="detection-buttons">
              <n-button
                v-if="ccrDetectionStatus === 'add'"
                type="primary"
                size="small"
                @click="addToCCR"
                :loading="isOperating"
                :disabled="!selectedDefaultModel"
              >
                <template #icon>
                  <n-icon :component="CloudUploadOutline" />
                </template>
                添加到CCR
              </n-button>

              <n-button
                v-if="ccrDetectionStatus === 'exists'"
                type="info"
                size="small"
                @click="updateCCR"
                :loading="isOperating"
                :disabled="!selectedDefaultModel"
              >
                <template #icon>
                  <n-icon :component="SyncOutline" />
                </template>
                保存到CCR
              </n-button>

              <n-button
                v-if="ccrDetectionStatus === 'exists'"
                type="info"
                size="small"
                @click="pullFromCCR"
                :loading="isOperating"
              >
                <template #icon>
                  <n-icon :component="DownloadOutline" />
                </template>
                从CCR拉取
              </n-button>

              <n-button
                v-if="ccrDetectionStatus === 'exists'"
                type="error"
                size="small"
                @click="removeFromCCR"
                :loading="isOperating"
              >
                <template #icon>
                  <n-icon :component="TrashBinOutline" />
                </template>
                从CCR移除
              </n-button>
            </div>
          </n-spin>
        </div>

        <!-- 工具区域 -->
        <div class="tools-section">
          <!-- 当没有配置片段时显示创建默认JSON按钮 -->
          <n-button
            v-if="!group?.code_snippet || group.code_snippet.trim() === ''"
            type="primary"
            size="small"
            @click="createDefaultJsonConfig"
            :loading="isLoading"
            style="margin-right: 8px"
          >
            创建默认JSON
          </n-button>

          <n-button type="info" ghost size="small" @click="openJsonModal">
            <template #icon>
              <n-icon :component="CodeSlash" />
            </template>
            JSON
          </n-button>
        </div>
      </div>

      <!-- 模型标签展示区域 -->
      <div class="models-display-section" v-if="ccrModels.length > 0">
        <div class="models-label">可用模型:</div>
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
        </n-space>
      </div>

      <div v-else class="no-models-section">
        <span class="no-models-text">未设置自定义模型</span>
      </div>
    </n-card>

    <!-- JSON 编辑模态窗口 -->
    <n-modal
      v-model:show="showJsonModal"
      preset="card"
      title="编辑 CCR 模型配置"
      style="width: 600px"
    >
      <!-- 模板操作按钮 -->
      <div class="template-buttons-section">
        <n-space :size="8">
          <n-button size="small" @click="setTemplate('openai')">创建OpenAI模板</n-button>
          <n-button size="small" @click="setTemplate('anthropic')">创建Anthropic模板</n-button>
          <n-button size="small" @click="setTemplate('gemini')">创建Gemini模板</n-button>
          <n-button size="small" type="warning" @click="clearJsonInput">清空输入</n-button>
        </n-space>
      </div>

      <div class="json-editor">
        <n-input
          ref="jsonTextarea"
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

        <!-- 快速模型标签区域 -->
        <div class="quick-models-section">
          <div class="quick-models-label">快速添加模型:</div>
          <n-space :size="8" class="quick-models-tags">
            <n-tag
              v-for="model in quickModels"
              :key="model"
              type="primary"
              round
              :bordered="false"
              class="quick-model-tag"
              @click="addQuickModelFromTag(model)"
            >
              {{ model }}
            </n-tag>
          </n-space>
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
  gap: 16px;
  flex-wrap: wrap;
}

.config-section {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

.action-section {
  display: flex;
  align-items: center;
  min-height: 32px;
}

.detection-buttons {
  display: flex;
  align-items: center;
  gap: 8px;
}

.tools-section {
  display: flex;
  align-items: center;
  margin-left: auto;
}

.models-display-section {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid #f0f0f0;
}

.models-label {
  font-size: 13px;
  color: #666;
  margin-bottom: 8px;
  font-weight: 500;
}

.no-models-section {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid #f0f0f0;
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

/* 模板操作按钮区域 */
.template-buttons-section {
  margin-bottom: 16px;
  padding-bottom: 16px;
  border-bottom: 1px solid #e8e8e8;
}

/* 快速模型标签区域 */
.quick-models-section {
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid #e8e8e8;
}

.quick-models-label {
  font-size: 14px;
  font-weight: 500;
  margin-bottom: 8px;
  color: #333;
}

.quick-models-tags {
  flex-wrap: wrap;
}

.quick-model-tag {
  cursor: pointer;
  transition: all 0.2s ease;
  user-select: none;
}

.quick-model-tag:hover {
  transform: translateY(-1px);
  box-shadow: 0 2px 4px rgba(24, 160, 88, 0.3);
}

.quick-model-tag:active {
  transform: translateY(0);
}
</style>
