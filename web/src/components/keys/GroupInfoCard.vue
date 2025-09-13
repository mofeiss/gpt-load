<script setup lang="ts">
import { keysApi } from "@/api/keys";
import { settingsApi } from "@/api/settings";
import ProxyKeysInput from "@/components/common/ProxyKeysInput.vue";
import type { Group, GroupConfigOption, GroupStatsResponse, UpstreamInfo } from "@/types/models";
import { appState } from "@/utils/app-state";
import { copy } from "@/utils/clipboard";
import { getGroupDisplayName, maskProxyKeys } from "@/utils/display";
import { Add, Close, CopyOutline, EyeOffOutline, EyeOutline, HelpCircleOutline, Pencil, Refresh, Remove, Trash } from "@vicons/ionicons5";
import {
  NButton,
  NCard,
  NCollapse,
  NCollapseItem,
  NForm,
  NFormItem,
  NGradientText,
  NGrid,
  NGridItem,
  NIcon,
  NInput,
  NInputNumber,
  NSelect,
  NStatistic,
  NSwitch,
  NTabs,
  NTabPane,
  NTag,
  NTooltip,
  useDialog,
  useMessage,
  type FormRules,
} from "naive-ui";
import { computed, h, nextTick, onMounted, ref, watch } from "vue";
import GroupFormModal from "./GroupFormModal.vue";
import GroupCopyModal from "./GroupCopyModal.vue";
import GroupSettingsForm from "./GroupSettingsForm.vue";

interface Props {
  group: Group | null;
}

interface Emits {
  (e: "refresh", value: Group): void;
  (e: "delete", value: Group): void;
  (e: "copy-success", group: Group): void;
  (e: "group-updated", group: Group): void;
  (e: "updated", group: Group): void;
}

const props = defineProps<Props>();

const emit = defineEmits<Emits>();

const stats = ref<GroupStatsResponse | null>(null);
const loading = ref(false);
const dialog = useDialog();
const message = useMessage();
// const showEditModal = ref(false); // 不再需要模态框
const showCopyModal = ref(false);
const delLoading = ref(false);
const confirmInput = ref("");
const configOptions = ref<GroupConfigOption[]>([]);
const showProxyKeys = ref(false);
const descriptionInput = ref<HTMLInputElement | null>(null);
const activeTab = ref("description");

// 渠道切换相关状态
const channelSwitchLoading = ref(false);

// 渠道类型循环顺序
const channelTypeCycle = ["openai", "anthropic", "gemini"] as const;

// 内联编辑相关状态
const isEditingDescription = ref(false);
const editingDescription = ref("");
const descriptionLoading = ref(false);

// 片段编辑相关状态（完全复制描述的模式）
const isEditingCodeSnippet = ref(false);
const editingCodeSnippet = ref("");
const codeSnippetLoading = ref(false);

const proxyKeysDisplay = computed(() => {
  if (!props.group?.proxy_keys) {
    return "-";
  }
  if (showProxyKeys.value) {
    return props.group.proxy_keys.replace(/,/g, "\n");
  }
  return maskProxyKeys(props.group.proxy_keys);
});

// 获取当前渠道类型的接口路径
const getChannelEndpoint = (channelType: string) => {
  switch (channelType) {
    case "openai":
      return "/v1/chat/completions";
    case "anthropic":
      return "/v1/messages?beta=true";
    case "gemini":
      return "/v1beta/models/";
    default:
      return "";
  }
};

// 生成完整 URL
const fullUpstreamUrl = computed(() => {
  if (!props.group?.upstreams || props.group.upstreams.length === 0) {
    return "";
  }
  const upstream = props.group.upstreams[0];
  const endpoint = props.group.validation_endpoint || getChannelEndpoint(props.group.channel_type);
  return `${upstream.url}${endpoint}`;
});

const hasAdvancedConfig = computed(() => {
  return (
    (props.group?.config && Object.keys(props.group.config).length > 0) ||
    props.group?.param_overrides ||
    (props.group?.header_rules && props.group.header_rules.length > 0)
  );
});

// 渠道类型切换函数
async function switchChannelType() {
  if (!props.group || !props.group.id || channelSwitchLoading.value) {
    return;
  }

  const currentChannelType = props.group.channel_type;
  const currentIndex = channelTypeCycle.indexOf(
    currentChannelType as "openai" | "anthropic" | "gemini"
  );

  if (currentIndex === -1) {
    return;
  }

  const nextIndex = (currentIndex + 1) % channelTypeCycle.length;
  const newChannelType = channelTypeCycle[nextIndex];

  try {
    channelSwitchLoading.value = true;

    // 调用 API 更新分组渠道类型
    const updatedGroup = await keysApi.updateGroup(props.group.id, {
      channel_type: newChannelType,
      validation_endpoint: getChannelEndpoint(newChannelType),
    });

    // 通知父组件更新数据
    emit("group-updated", updatedGroup);

    //message.success(`渠道类型已切换至 ${newChannelType}`);
  } catch (error) {
    console.error("切换渠道类型失败:", error);
    message.error("切换渠道类型失败，请稍后重试");
  } finally {
    channelSwitchLoading.value = false;
  }
}

async function copyProxyKeys() {
  if (!props.group?.proxy_keys) {
    return;
  }
  const keysToCopy = props.group.proxy_keys.replace(/,/g, "\n");
  const success = await copy(keysToCopy);
  if (success) {
    window.$message.success("代理密钥已复制到剪贴板", {
      duration: 3000,
    });
  } else {
    window.$message.error("复制失败", {
      duration: 3000,
    });
  }
}

onMounted(() => {
  loadStats();
  loadConfigOptions();
});

watch(
  () => props.group,
  () => {
    resetPage();
    loadStats();
  }
);

// 监听任务完成事件，自动刷新当前分组数据
watch(
  () => appState.groupDataRefreshTrigger,
  () => {
    // 检查是否需要刷新当前分组的数据
    if (appState.lastCompletedTask && props.group) {
      // 通过分组名称匹配
      const isCurrentGroup = appState.lastCompletedTask.groupName === props.group.name;

      const shouldRefresh =
        appState.lastCompletedTask.taskType === "KEY_VALIDATION" ||
        appState.lastCompletedTask.taskType === "KEY_IMPORT" ||
        appState.lastCompletedTask.taskType === "KEY_DELETE";

      if (isCurrentGroup && shouldRefresh) {
        // 刷新当前分组的统计数据
        loadStats();
      }
    }
  }
);

// 监听同步操作完成事件，自动刷新当前分组数据
watch(
  () => appState.syncOperationTrigger,
  () => {
    // 检查是否需要刷新当前分组的数据
    if (appState.lastSyncOperation && props.group) {
      // 通过分组名称匹配
      const isCurrentGroup = appState.lastSyncOperation.groupName === props.group.name;

      if (isCurrentGroup) {
        // 刷新当前分组的统计数据
        loadStats();
      }
    }
  }
);

async function loadStats() {
  if (!props.group?.id) {
    stats.value = null;
    return;
  }

  try {
    loading.value = true;
    if (props.group?.id) {
      stats.value = await keysApi.getGroupStats(props.group.id);
    }
  } finally {
    loading.value = false;
  }
}

async function loadConfigOptions() {
  try {
    const options = await keysApi.getGroupConfigOptions();
    configOptions.value = options || [];
  } catch (error) {
    console.error("获取配置选项失败:", error);
  }
}

function getConfigDisplayName(key: string): string {
  const option = configOptions.value.find(opt => opt.key === key);
  return option?.name || key;
}

function getConfigDescription(key: string): string {
  const option = configOptions.value.find(opt => opt.key === key);
  return option?.description || "暂无说明";
}

function handleEdit() {
  activeTab.value = "settings";
}

function handleCopy() {
  showCopyModal.value = true;
}

function handleGroupEdited(newGroup: Group) {
  // showEditModal.value = false; // 不再需要模态框
  if (newGroup) {
    emit("refresh", newGroup);
  }
}

// 处理编辑分组后的更新（带刷新）
function handleGroupUpdated(newGroup: Group) {
  // showEditModal.value = false; // 不再需要模态框
  if (newGroup) {
    emit("refresh", newGroup);
    // 重新加载当前分组的统计数据
    loadStats();
  }
}

// 处理设置表单的更新
function handleGroupUpdatedFromSettings(newGroup: Group) {
  if (newGroup) {
    emit("updated", newGroup);
    // 重新加载当前分组的统计数据
    loadStats();
  }
}

function handleGroupCopied(newGroup: Group) {
  showCopyModal.value = false;
  if (newGroup) {
    emit("copy-success", newGroup);
  }
}

async function handleDelete() {
  if (!props.group || delLoading.value) {
    return;
  }

  dialog.warning({
    title: "删除分组",
    content: `确定要删除分组 "${getGroupDisplayName(
      props.group
    )}" 吗？此操作将删除分组及其下所有密钥，且不可恢复。`,
    positiveText: "确定",
    negativeText: "取消",
    onPositiveClick: () => {
      confirmInput.value = ""; // Reset before opening second dialog
      dialog.create({
        title: "请输入分组名称以确认删除",
        content: () => {
          return h("div", {}, [
            h("p", {}, [
              "这是一个非常危险的操作。为防止误操作，请输入分组名称 ",
              h("strong", { style: { color: "#d03050" } }, props.group?.name),
              " 以确认删除。",
            ]),
            h(NInput, {
              value: confirmInput.value,
              "onUpdate:value": (v: string) => {
                confirmInput.value = v;
              },
              placeholder: "请输入分组名称",
            }),
          ]);
        },
        positiveText: "确认删除",
        negativeText: "取消",
        onPositiveClick: async () => {
          if (confirmInput.value !== props.group?.name) {
            window.$message.error("分组名称输入不正确", {
              duration: 3000,
            });
            return false; // Prevent dialog from closing
          }

          delLoading.value = true;
          try {
            if (props.group?.id) {
              await keysApi.deleteGroup(props.group.id);
              emit("delete", props.group);
              window.$message.success("分组已成功删除", {
                duration: 3000,
              });
            }
          } catch (error) {
            console.error("删除分组失败:", error);
            window.$message.error("删除分组失败，请稍后重试", {
              duration: 3000,
            });
          } finally {
            delLoading.value = false;
          }
        },
      });
    },
  });
}

function formatNumber(num: number): string {
  // if (num >= 1000000) {
  //   return `${(num / 1000000).toFixed(1)}M`;
  // }
  if (num >= 1000) {
    return `${(num / 1000).toFixed(1)}K`;
  }
  return num.toString();
}

function formatPercentage(num: number): string {
  if (num <= 0) {
    return "0";
  }
  return `${(num * 100).toFixed(1)}%`;
}

async function copyUrl(url: string) {
  if (!url) {
    return;
  }
  const success = await copy(url);
  if (success) {
    window.$message.success("地址已复制到剪贴板", {
      duration: 3000,
    });
  } else {
    window.$message.error("复制失败", {
      duration: 3000,
    });
  }
}

// 开始编辑描述
function startEditingDescription() {
  if (!props.group) {
    return;
  }
  isEditingDescription.value = true;
  editingDescription.value = props.group.description || "";

  // 等待DOM更新后自动聚焦到输入框
  nextTick(() => {
    if (descriptionInput.value) {
      descriptionInput.value.focus();
    }
  });
}


// 取消编辑描述
function cancelEditingDescription() {
  isEditingDescription.value = false;
  editingDescription.value = "";
}

// 保存描述编辑
async function saveDescription() {
  if (!props.group || descriptionLoading.value || props.group.id === undefined) {
    return;
  }

  try {
    descriptionLoading.value = true;

    // 调用API更新分组描述
    const updatedGroup = await keysApi.updateGroup(props.group.id, {
      description: editingDescription.value,
    });

    // 通知父组件更新数据
    emit("group-updated", updatedGroup);
    isEditingDescription.value = false;
    editingDescription.value = "";

    message.success("描述已更新");
  } catch (error) {
    console.error("更新描述失败:", error);
    message.error("更新描述失败，请稍后重试");
    // 保存失败时保持编辑状态，让用户可以继续编辑
    isEditingDescription.value = true;
  } finally {
    descriptionLoading.value = false;
  }
}

// 开始编辑片段（复制描述的逻辑）
function startEditingCodeSnippet() {
  if (!props.group) {
    return;
  }
  isEditingCodeSnippet.value = true;
  editingCodeSnippet.value = props.group.code_snippet || "";

  // 等待DOM更新后自动聚焦到输入框
  nextTick(() => {
    const codeSnippetInput = document.querySelector('.code-snippet-textarea .n-input__textarea-el') as HTMLElement;
    if (codeSnippetInput) {
      codeSnippetInput.focus();
    }
  });
}

// 取消编辑片段
function cancelEditingCodeSnippet() {
  isEditingCodeSnippet.value = false;
  editingCodeSnippet.value = "";
}

// 保存片段编辑（完全复制描述的逻辑）
async function saveCodeSnippet() {
  if (!props.group || codeSnippetLoading.value || props.group.id === undefined) {
    return;
  }

  try {
    codeSnippetLoading.value = true;

    // 调用API更新分组片段
    const updatedGroup = await keysApi.updateGroup(props.group.id, {
      code_snippet: editingCodeSnippet.value,
    });

    // 通知父组件更新数据
    emit("group-updated", updatedGroup);
    isEditingCodeSnippet.value = false;
    editingCodeSnippet.value = "";

    message.success("片段已更新");
  } catch (error) {
    console.error("更新片段失败:", error);
    message.error("更新片段失败，请稍后重试");
    // 保存失败时保持编辑状态，让用户可以继续编辑
    isEditingCodeSnippet.value = true;
  } finally {
    codeSnippetLoading.value = false;
  }
}

function resetPage() {
  // showEditModal.value = false; // 不再需要模态框
  showCopyModal.value = false;
  // 重置内联编辑状态
  isEditingDescription.value = false;
  editingDescription.value = "";
  // 重置片段编辑状态
  isEditingCodeSnippet.value = false;
  editingCodeSnippet.value = "";
  // 重置标签页到描述卡片
  activeTab.value = "description";
}

// 复制描述内容
async function copyDescription() {
  if (!props.group?.description) {
    return;
  }
  const success = await copy(props.group.description);
  if (success) {
    window.$message.success("描述已复制到剪贴板", {
      duration: 3000,
    });
  } else {
    window.$message.error("复制失败", {
      duration: 3000,
    });
  }
}

// 复制片段内容
async function copyCodeSnippet() {
  if (!props.group?.code_snippet) {
    return;
  }
  const success = await copy(props.group.code_snippet);
  if (success) {
    window.$message.success("片段已复制到剪贴板", {
      duration: 3000,
    });
  } else {
    window.$message.error("复制失败", {
      duration: 3000,
    });
  }
}
</script>

<template>
  <div class="group-info-container">
    <n-card :bordered="false" class="group-info-card">
      <template #header>
        <div class="card-header">
          <!-- 第 1 列：分组名称和显示名称 -->
          <div class="header-column-1">
            <div class="column-row-1">
              <h3 class="group-title">
                {{ group ? group.name : "请选择分组" }}
              </h3>
            </div>
            <div class="column-row-2">
              <span v-if="group && group.display_name" class="group-display-name">
                {{ group.display_name }}
              </span>
              <span v-else class="group-display-name-placeholder">&nbsp;</span>
            </div>
          </div>

          <!-- 第 2 列：代理网址和上游拼接地址 -->
          <div class="header-column-2">
            <div class="column-row-1">
              <n-tooltip trigger="hover" v-if="group && group.endpoint">
                <template #trigger>
                  <div class="group-url" @click="copyUrl(group.endpoint)">
                    {{ group.endpoint }}
                  </div>
                </template>
                点击复制
              </n-tooltip>
            </div>
            <div class="column-row-2" v-if="group && group.upstreams && group.upstreams.length > 0">
              <n-tooltip trigger="hover">
                <template #trigger>
                  <div class="upstream-endpoint-url" @click="copyUrl(fullUpstreamUrl)">
                    {{ fullUpstreamUrl }}
                  </div>
                </template>
                点击复制完整地址
              </n-tooltip>
            </div>
          </div>

          <!-- 第 3 列：刷新、复制、编辑、删除按钮 -->
          <div class="header-column-3">
            <div class="column-row-1">
              <n-tooltip
                trigger="hover"
                v-if="group && group.upstreams && group.upstreams.length > 0"
              >
                <template #trigger>
                  <n-button
                    quaternary
                    circle
                    size="small"
                    @click="switchChannelType"
                    :loading="channelSwitchLoading"
                    class="channel-switch-btn"
                  >
                    <template #icon>
                      <n-icon :component="Refresh" />
                    </template>
                  </n-button>
                </template>
                切换渠道类型 ({{ group.channel_type }})
              </n-tooltip>
              <n-button
                quaternary
                circle
                size="small"
                @click="handleCopy"
                title="复制分组"
                :disabled="!group"
                class="copy-btn"
              >
                <template #icon>
                  <n-icon :component="CopyOutline" />
                </template>
              </n-button>
            </div>
            <div class="column-row-2">
              <n-button
                quaternary
                circle
                size="small"
                @click="handleEdit"
                title="编辑分组"
                class="edit-btn"
              >
                <template #icon>
                  <n-icon :component="Pencil" />
                </template>
              </n-button>
              <n-button
                quaternary
                circle
                size="small"
                @click="handleDelete"
                title="删除分组"
                type="error"
                :disabled="!group"
                class="delete-btn"
              >
                <template #icon>
                  <n-icon :component="Trash" />
                </template>
              </n-button>
            </div>
          </div>
        </div>
      </template>

      <n-divider style="margin: 0; margin-bottom: 12px" />
      <!-- 统计摘要区 -->
      <div class="stats-summary">
        <n-spin :show="loading" size="small">
          <n-grid cols="2 s:4" :x-gap="12" :y-gap="12" responsive="screen">
            <n-grid-item span="1">
              <n-statistic :label="`密钥数量：${stats?.key_stats?.total_keys ?? 0}`">
                <n-tooltip trigger="hover">
                  <template #trigger>
                    <n-gradient-text type="success" size="20">
                      {{ stats?.key_stats?.active_keys ?? 0 }}
                    </n-gradient-text>
                  </template>
                  有效密钥数
                </n-tooltip>
                <n-divider vertical />
                <n-tooltip trigger="hover">
                  <template #trigger>
                    <n-gradient-text type="error" size="20">
                      {{ stats?.key_stats?.invalid_keys ?? 0 }}
                    </n-gradient-text>
                  </template>
                  无效密钥数
                </n-tooltip>
              </n-statistic>
            </n-grid-item>
            <n-grid-item span="1">
              <n-statistic
                :label="`1小时请求：${formatNumber(stats?.hourly_stats?.total_requests ?? 0)}`"
              >
                <n-tooltip trigger="hover">
                  <template #trigger>
                    <n-gradient-text type="error" size="20">
                      {{ formatNumber(stats?.hourly_stats?.failed_requests ?? 0) }}
                    </n-gradient-text>
                  </template>
                  近1小时失败请求
                </n-tooltip>
                <n-divider vertical />
                <n-tooltip trigger="hover">
                  <template #trigger>
                    <n-gradient-text type="error" size="20">
                      {{ formatPercentage(stats?.hourly_stats?.failure_rate ?? 0) }}
                    </n-gradient-text>
                  </template>
                  近1小时失败率
                </n-tooltip>
              </n-statistic>
            </n-grid-item>
            <n-grid-item span="1">
              <n-statistic
                :label="`24小时请求：${formatNumber(stats?.daily_stats?.total_requests ?? 0)}`"
              >
                <n-tooltip trigger="hover">
                  <template #trigger>
                    <n-gradient-text type="error" size="20">
                      {{ formatNumber(stats?.daily_stats?.failed_requests ?? 0) }}
                    </n-gradient-text>
                  </template>
                  近24小时失败请求
                </n-tooltip>
                <n-divider vertical />
                <n-tooltip trigger="hover">
                  <template #trigger>
                    <n-gradient-text type="error" size="20">
                      {{ formatPercentage(stats?.daily_stats?.failure_rate ?? 0) }}
                    </n-gradient-text>
                  </template>
                  近24小时失败率
                </n-tooltip>
              </n-statistic>
            </n-grid-item>
            <n-grid-item span="1">
              <n-statistic
                :label="`近7天请求：${formatNumber(stats?.weekly_stats?.total_requests ?? 0)}`"
              >
                <n-tooltip trigger="hover">
                  <template #trigger>
                    <n-gradient-text type="error" size="20">
                      {{ formatNumber(stats?.weekly_stats?.failed_requests ?? 0) }}
                    </n-gradient-text>
                  </template>
                  近7天失败请求
                </n-tooltip>
                <n-divider vertical />
                <n-tooltip trigger="hover">
                  <template #trigger>
                    <n-gradient-text type="error" size="20">
                      {{ formatPercentage(stats?.weekly_stats?.failure_rate ?? 0) }}
                    </n-gradient-text>
                  </template>
                  近7天失败率
                </n-tooltip>
              </n-statistic>
            </n-grid-item>
          </n-grid>
        </n-spin>
      </div>
      <n-divider style="margin: 0" />

      <!-- 标签页区域 -->
      <div class="tab-section">
        <n-tabs v-model:value="activeTab" type="line" animated>
          <!-- 描述卡片标签页 (索引 0) -->
          <n-tab-pane name="description" tab="描述卡片">
            <!-- 分组描述区域 -->
            <div class="group-description-section card-section">
              <!-- 固定复制按钮 -->
              <n-button
                v-if="group?.description && !isEditingDescription"
                quaternary
                circle
                size="small"
                @click="copyDescription"
                class="fixed-copy-btn"
                title="复制描述"
              >
                <template #icon>
                  <n-icon :component="CopyOutline" />
                </template>
              </n-button>
              <!-- 显示模式：点击可编辑 -->
              <div
                class="description-content"
                v-if="!isEditingDescription && group?.description"
                @click="startEditingDescription"
                :class="{ 'description-editable': !isEditingDescription }"
              >
                {{ group.description }}
              </div>

              <!-- 显示模式：没有描述时的占位符 -->
              <div
                class="description-content description-placeholder"
                v-if="!isEditingDescription && !group?.description"
                @click="startEditingDescription"
                :class="{ 'description-editable': !isEditingDescription }"
              >
                点击添加描述...
              </div>

              <!-- 编辑模式 -->
              <div class="description-edit-container" v-if="isEditingDescription">
                <n-input
                  v-model:value="editingDescription"
                  type="textarea"
                  placeholder="请输入分组描述..."
                  :loading="descriptionLoading"
                  @blur="saveDescription"
                  @keyup.enter.ctrl="saveDescription"
                  @keyup.esc="cancelEditingDescription"
                  ref="descriptionInput"
                  class="description-textarea"
                />
              </div>
            </div>
          </n-tab-pane>

          <!-- 片段标签页 (索引 1) -->
          <n-tab-pane name="code-snippet" tab="片段">
            <!-- 分组片段区域 -->
            <div class="group-description-section card-section">
              <!-- 固定复制按钮 -->
              <n-button
                v-if="group?.code_snippet && !isEditingCodeSnippet"
                quaternary
                circle
                size="small"
                @click="copyCodeSnippet"
                class="fixed-copy-btn"
                title="复制片段"
              >
                <template #icon>
                  <n-icon :component="CopyOutline" />
                </template>
              </n-button>
              <!-- 显示模式：点击可编辑 -->
              <div
                class="description-content code-snippet-content"
                v-if="!isEditingCodeSnippet && group?.code_snippet"
                @click="startEditingCodeSnippet"
                :class="{ 'description-editable': !isEditingCodeSnippet }"
              >
                {{ group.code_snippet }}
              </div>

              <!-- 显示模式：没有片段时的占位符 -->
              <div
                class="description-content description-placeholder"
                v-if="!isEditingCodeSnippet && !group?.code_snippet"
                @click="startEditingCodeSnippet"
                :class="{ 'description-editable': !isEditingCodeSnippet }"
              >
                点击添加代码片段...
              </div>

              <!-- 编辑模式 -->
              <div class="description-edit-container" v-if="isEditingCodeSnippet">
                <n-input
                  v-model:value="editingCodeSnippet"
                  type="textarea"
                  placeholder="请输入代码片段..."
                  :loading="codeSnippetLoading"
                  @blur="saveCodeSnippet"
                  @keyup.enter.ctrl="saveCodeSnippet"
                  @keyup.esc="cancelEditingCodeSnippet"
                  class="description-textarea code-snippet-textarea"
                />
              </div>
            </div>
          </n-tab-pane>

          <!-- 详细信息标签页 (索引 2) -->
          <n-tab-pane name="details" tab="详细信息">
            <div class="details-content">
              <div class="detail-section">
                <h4 class="section-title">基础信息</h4>
                <n-form label-placement="left" label-width="85px" label-align="right">
                  <n-grid cols="1 m:2">
                    <n-grid-item>
                      <n-form-item label="分组名称：">
                        {{ group?.name }}
                      </n-form-item>
                    </n-grid-item>
                    <n-grid-item>
                      <n-form-item label="显示名称：">
                        {{ group?.display_name }}
                      </n-form-item>
                    </n-grid-item>
                    <n-grid-item>
                      <n-form-item label="渠道类型：">
                        {{ group?.channel_type }}
                      </n-form-item>
                    </n-grid-item>
                    <n-grid-item>
                      <n-form-item label="排序：">
                        {{ group?.sort }}
                      </n-form-item>
                    </n-grid-item>
                    <n-grid-item>
                      <n-form-item label="测试模型：">
                        {{ group?.test_model }}
                      </n-form-item>
                    </n-grid-item>
                    <n-grid-item v-if="group?.channel_type !== 'gemini'">
                      <n-form-item label="测试路径：">
                        {{ group?.validation_endpoint }}
                      </n-form-item>
                    </n-grid-item>
                    <n-grid-item :span="2">
                      <n-form-item label="代理密钥：">
                        <div class="proxy-keys-content">
                          <span class="key-text">{{ proxyKeysDisplay }}</span>
                          <n-button-group size="small" class="key-actions" v-if="group?.proxy_keys">
                            <n-tooltip trigger="hover">
                              <template #trigger>
                                <n-button quaternary circle @click="showProxyKeys = !showProxyKeys">
                                  <template #icon>
                                    <n-icon
                                      :component="showProxyKeys ? EyeOffOutline : EyeOutline"
                                    />
                                  </template>
                                </n-button>
                              </template>
                              {{ showProxyKeys ? "隐藏密钥" : "显示密钥" }}
                            </n-tooltip>
                            <n-tooltip trigger="hover">
                              <template #trigger>
                                <n-button quaternary circle @click="copyProxyKeys">
                                  <template #icon>
                                    <n-icon :component="CopyOutline" />
                                  </template>
                                </n-button>
                              </template>
                              复制密钥
                            </n-tooltip>
                          </n-button-group>
                        </div>
                      </n-form-item>
                    </n-grid-item>
                  </n-grid>
                </n-form>
              </div>

              <div class="detail-section">
                <h4 class="section-title">上游地址</h4>
                <n-form label-placement="left" label-width="100px">
                  <n-form-item
                    v-for="(upstream, index) in group?.upstreams ?? []"
                    :key="index"
                    class="upstream-item"
                    :label="`上游 ${index + 1}:`"
                  >
                    <span class="upstream-weight">
                      <n-tag size="small" type="info">权重: {{ upstream.weight }}</n-tag>
                    </span>
                    <n-input class="upstream-url" :value="upstream.url" readonly size="small" />
                  </n-form-item>
                </n-form>
              </div>

              <div class="detail-section" v-if="hasAdvancedConfig">
                <h4 class="section-title">高级配置</h4>
                <n-form label-placement="left">
                  <n-form-item v-for="(value, key) in group?.config || {}" :key="key">
                    <template #label>
                      <n-tooltip trigger="hover" :delay="300" placement="top">
                        <template #trigger>
                          <span class="config-label">
                            {{ getConfigDisplayName(key) }}:
                            <n-icon size="14" class="config-help-icon">
                              <svg viewBox="0 0 24 24">
                                <path
                                  fill="currentColor"
                                  d="M12,2A10,10 0 0,0 2,12A10,10 0 0,0 12,22A10,10 0 0,0 22,12A10,10 0 0,0 12,2M12,17A1.5,1.5 0 0,1 10.5,15.5A1.5,1.5 0 0,1 12,14A1.5,1.5 0 0,1 13.5,15.5A1.5,1.5 0 0,1 12,17M12,10.5C10.07,10.5 8.5,8.93 8.5,7A3.5,3.5 0 0,1 12,3.5A3.5,3.5 0 0,1 15.5,7C15.5,8.93 13.93,10.5 12,10.5Z"
                                />
                              </svg>
                            </n-icon>
                          </span>
                        </template>
                        <div class="config-tooltip">
                          <div class="tooltip-title">{{ getConfigDisplayName(key) }}</div>
                          <div class="tooltip-description">{{ getConfigDescription(key) }}</div>
                          <div class="tooltip-key">配置键: {{ key }}</div>
                        </div>
                      </n-tooltip>
                    </template>
                    {{ value || "-" }}
                  </n-form-item>
                  <n-form-item
                    v-if="group?.header_rules && group.header_rules.length > 0"
                    label="自定义请求头:"
                    :span="2"
                  >
                    <div class="header-rules-display">
                      <div
                        v-for="(rule, index) in group.header_rules"
                        :key="index"
                        class="header-rule-item"
                      >
                        <n-tag :type="rule.action === 'remove' ? 'error' : 'default'" size="small">
                          {{ rule.key }}
                        </n-tag>
                        <span class="header-separator">:</span>
                        <span class="header-value" v-if="rule.action === 'set'">
                          {{ rule.value || "(空值)" }}
                        </span>
                        <span class="header-removed" v-else>删除</span>
                      </div>
                    </div>
                  </n-form-item>
                  <n-form-item v-if="group?.param_overrides" label="参数覆盖:" :span="2">
                    <pre class="config-json">{{
                      JSON.stringify(group?.param_overrides || "", null, 2)
                    }}</pre>
                  </n-form-item>
                </n-form>
              </div>
            </div>
          </n-tab-pane>

          <!-- 设置标签页 (索引 3) -->
          <n-tab-pane name="settings" tab="设置">
            <div class="settings-content">
              <div v-if="!group" class="no-group-message">
                <p>请先选择一个分组</p>
              </div>
              <div v-else class="settings-form-container">
                <group-settings-form
                  :group="group"
                  @updated="handleGroupUpdatedFromSettings"
                />
              </div>
            </div>
          </n-tab-pane>
        </n-tabs>
      </div>
    </n-card>

    <!-- 模态框已移除，设置功能已集成到tab中 -->
    <!-- <group-form-modal
      v-model:show="showEditModal"
      :group="group"
      @success="handleGroupEdited"
      @updated="handleGroupUpdated"
    /> -->
    <group-copy-modal
      v-model:show="showCopyModal"
      :source-group="group"
      @success="handleGroupCopied"
    />
  </div>
</template>

<style scoped>
.group-info-container {
  width: 100%;
}

:deep(.n-card-header) {
  padding: 12px 24px;
}

.group-info-card {
  background: rgba(255, 255, 255, 0.98);
  border-radius: var(--border-radius-lg);
  border: 1px solid rgba(255, 255, 255, 0.3);
  animation: fadeInUp 0.2s ease-out;
}

.card-header {
  display: grid;
  grid-template-columns: auto 1fr auto;
  grid-template-rows: auto auto;
  gap: 8px 16px;
  align-items: center;
  width: 100%;
}

.header-column-1 {
  justify-self: start;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.header-column-2 {
  justify-self: stretch;
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1;
  min-width: 0;
  width: 100%;
}

.header-column-3 {
  justify-self: end;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.column-row-1 {
  display: flex;
  align-items: center;
  gap: 8px;
}

.column-row-2 {
  display: flex;
  align-items: center;
  gap: 8px;
}

.group-title {
  font-size: 1.2rem;
  font-weight: 600;
  color: #1e293b;
  margin: 0;
}

.group-url {
  font-size: 0.8rem;
  color: #2563eb;
  font-family: monospace;
  background: rgba(37, 99, 235, 0.1);
  border-radius: 4px;
  padding: 2px 6px;
  cursor: pointer;
  transition: background-color 0.2s ease;
  width: 100%;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: block;
  box-sizing: border-box;
}

/* 上游地址和接口路径展示区域 */
.upstream-endpoint-section {
  padding: 0px 0;
  width: 100%;
}

.upstream-endpoint-url {
  font-size: 0.8rem;
  color: #059669;
  font-family: monospace;
  background: rgba(5, 150, 105, 0.1);
  border-radius: 4px;
  padding: 4px 8px;
  cursor: pointer;
  transition: background-color 0.2s ease;
  width: 100%;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: block;
  box-sizing: border-box;
}

.upstream-endpoint-url:hover {
  background: rgba(5, 150, 105, 0.2);
}

/* 分组显示名称样式 */
.group-display-name {
  font-size: 0.9rem;
  color: #64748b;
  font-weight: 500;
}

/* 分组显示名称占位符样式 */
.group-display-name-placeholder {
  font-size: 0.9rem;
  color: transparent;
  font-weight: 500;
  visibility: hidden;
}

/* 按钮样式 */
.channel-switch-btn,
.copy-btn,
.edit-btn,
.delete-btn {
  opacity: 0.8;
  transition: opacity 0.2s ease;
  flex-shrink: 0;
}

.channel-switch-btn:hover,
.copy-btn:hover,
.edit-btn:hover,
.delete-btn:hover {
  opacity: 1;
}

.group-url:hover {
  background: rgba(37, 99, 235, 0.2);
}

/* .group-meta {
  display: flex;
  align-items: center;
  gap: 8px;
} */

.group-id {
  font-size: 0.75rem;
  color: #64748b;
  opacity: 0.7;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.stats-summary {
  margin-bottom: 12px;
  text-align: center;
}

.status-cards-container:deep(.n-card) {
  max-width: 160px;
}

:deep(.status-card-failure .n-card-header__main) {
  color: #d03050;
}

.status-title {
  color: #64748b;
  font-size: 12px;
}

.tab-section {
  margin-top: 12px;
}

.details-section {
  margin-top: 12px;
}

.details-title {
  font-size: 1.1rem;
  font-weight: 600;
  color: #374151;
  margin: 0 0 16px 0;
  padding-bottom: 8px;
  border-bottom: 2px solid rgba(102, 126, 234, 0.2);
}

.details-content {
  margin-top: 12px;
}

.detail-section {
  margin-bottom: 24px;
}

.detail-section:last-child {
  margin-bottom: 0;
}

.section-title {
  font-size: 1rem;
  font-weight: 600;
  color: #374151;
  margin: 0 0 12px 0;
  padding-bottom: 8px;
  border-bottom: 2px solid rgba(102, 126, 234, 0.1);
}

.upstream-url {
  font-family: monospace;
  font-size: 0.9rem;
  color: #374151;
  margin-left: 5px;
}

.upstream-weight {
  min-width: 70px;
}

.config-json {
  background: rgba(102, 126, 234, 0.05);
  border-radius: var(--border-radius-sm);
  padding: 12px;
  font-size: 0.8rem;
  color: #374151;
  margin: 8px 0;
  overflow-x: auto;
}

@keyframes fadeInUp {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

:deep(.n-form-item-feedback-wrapper) {
  min-height: 0;
}

/* 分组描述区域样式 */
.group-description-section {
  margin: 12px 0;
  padding: 12px 16px;
  background: rgba(102, 126, 234, 0.05);
  border-radius: var(--border-radius-sm);
  border-left: 4px solid rgba(102, 126, 234, 0.3);
  max-height: 400px; /* 限制最大高度为400px */
  overflow-y: auto; /* 超出时显示滚动条 */
}

/* 卡片区域容器 */
.card-section {
  position: relative;
}

/* 固定复制按钮 */
.fixed-copy-btn {
  position: absolute;
  top: 8px;
  right: 8px;
  z-index: 10;
  background: rgba(255, 255, 255, 0.9);
  backdrop-filter: blur(4px);
  opacity: 0.7;
  transition: opacity 0.2s ease;
}

.fixed-copy-btn:hover {
  opacity: 1;
  background: rgba(255, 255, 255, 0.95);
}

/* 滚动条样式美化 */
.group-description-section::-webkit-scrollbar {
  width: 6px;
}

.group-description-section::-webkit-scrollbar-track {
  background: rgba(102, 126, 234, 0.08);
  border-radius: 3px;
}

.group-description-section::-webkit-scrollbar-thumb {
  background: rgba(102, 126, 234, 0.3);
  border-radius: 3px;
}

.group-description-section::-webkit-scrollbar-thumb:hover {
  background: rgba(102, 126, 234, 0.5);
}

/* 描述内容样式 */
.description-content {
  white-space: pre-wrap;
  word-wrap: break-word;
  line-height: 1.5;
  min-height: 20px;
  color: #374151;
  font-size: 0.9rem;
}

/* 描述可编辑状态样式 */
.description-editable {
  cursor: text;
  border-radius: var(--border-radius-sm);
  padding: 4px 8px;
  margin: -4px -8px;
  transition: background-color 0.2s ease;
}

.description-editable:hover {
  background-color: rgba(102, 126, 234, 0.08);
}

.description-editable:active {
  background-color: rgba(102, 126, 234, 0.12);
}

/* 描述占位符样式 */
.description-placeholder {
  color: #9ca3af;
  font-style: italic;
  font-size: 0.9rem;
}

/* 描述编辑容器样式 */
.description-edit-container {
  display: flex;
  flex-direction: column;
  height: 400px; /* 固定高度为 300px */
}

/* 描述编辑框样式 */
.description-textarea {
  height: 375px; /* 固定高度为 375px */
}

.description-textarea :deep(.n-input__textarea-el) {
  height: 375px !important; /* 固定高度为 375px */
  min-height: 375px !important;
  resize: none !important;
  overflow-y: auto !important;
}

/* 片段内容专用样式 */
.code-snippet-content {
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace !important;
  font-size: 0.85rem;
  line-height: 1.4;
}

.code-snippet-textarea :deep(.n-input__textarea-el) {
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace !important;
  font-size: 0.85rem;
  line-height: 1.4;
}

.proxy-keys-content {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  width: 100%;
  gap: 8px;
}

.key-text {
  flex-grow: 1;
  font-family: monospace;
  white-space: pre-wrap;
  word-break: break-all;
  line-height: 1.5;
  padding-top: 4px; /* Align with buttons */
  color: #374151;
}

.key-actions {
  flex-shrink: 0;
}

/* 配置项tooltip样式 */
.config-label {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  cursor: help;
}

.config-help-icon {
  color: #9ca3af;
  transition: color 0.2s ease;
}

.config-label:hover .config-help-icon {
  color: #6366f1;
}

.config-tooltip {
  max-width: 300px;
  padding: 8px 0;
}

.tooltip-title {
  font-weight: 600;
  color: #ffffff;
  margin-bottom: 4px;
  font-size: 0.9rem;
}

.tooltip-description {
  color: #e5e7eb;
  margin-bottom: 6px;
  line-height: 1.4;
  font-size: 0.85rem;
}

.tooltip-key {
  color: #d1d5db;
  font-size: 0.75rem;
  font-family: monospace;
  background: rgba(255, 255, 255, 0.15);
  padding: 2px 6px;
  border-radius: 4px;
  display: inline-block;
}

/* Header rules display styles */
.header-rules-display {
  display: flex;
  flex-direction: column;
  gap: 6px;
  background: rgba(102, 126, 234, 0.03);
  border-radius: var(--border-radius-sm);
  padding: 8px;
}

.header-rule-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.875rem;
}

.header-separator {
  color: #6b7280;
  font-weight: 500;
}

.header-value {
  color: #374151;
  font-family: monospace;
  background: rgba(59, 130, 246, 0.08);
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 0.8rem;
}

.header-removed {
  color: #dc2626;
  font-style: italic;
  font-size: 0.8rem;
}

/* 设置页面样式 */
.settings-content {
  padding: 0;
}

.no-group-message {
  text-align: center;
  color: #9ca3af;
  padding: 40px 20px;
  font-size: 0.9rem;
}

.settings-form-container {
  padding: 0;
  max-height: 70vh;
  overflow-y: auto;
}
</style>
