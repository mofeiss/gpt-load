<script setup lang="ts">
import { keysApi } from "@/api/keys";
import type { Group, GroupConfigOption, GroupStatsResponse } from "@/types/models";
import { appState } from "@/utils/app-state";
import { copy } from "@/utils/clipboard";
import { getGroupDisplayName, maskProxyKeys } from "@/utils/display";
import {
  convertCCRConfigToChannel,
  isValidCCRConfig,
  getChannelDisplayName,
} from "@/utils/ccr-config-converter";
import { CopyOutline, EyeOffOutline, EyeOutline, Pencil, Refresh, Trash } from "@vicons/ionicons5";
import {
  NButton,
  NCard,
  NForm,
  NFormItem,
  NGrid,
  NGridItem,
  NIcon,
  NInput,
  NTabPane,
  NTabs,
  NTag,
  NTooltip,
  useDialog,
  useMessage,
} from "naive-ui";
import { computed, h, nextTick, onMounted, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { useAuthKey } from "@/services/auth";
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

const route = useRoute();

// 获取全局认证密钥
const authKey = useAuthKey();

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
const editModeFormRefs = ref<{
  basic: any;
  upstream: any;
  advanced: any;
}>({
  basic: null,
  upstream: null,
  advanced: null,
});

// 渠道切换相关状态
const channelSwitchLoading = ref(false);

// 渠道类型循环顺序
const channelTypeCycle = ["openai", "anthropic", "gemini"] as const;

// 内联编辑相关状态
const isEditingDescription = ref(false);
const editingDescription = ref("");
const descriptionLoading = ref(false);

// 详细信息页面编辑模式状态
const isEditMode = ref(false);
const editModeLoading = ref(false);
const detailsActiveTab = ref("basic"); // 详细信息主tabs的当前标签

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

    // 准备更新数据
    const updateData: any = {
      channel_type: newChannelType,
      validation_endpoint: getChannelEndpoint(newChannelType),
    };

    // 如果存在有效的 CCR 配置，则进行格式转换
    if (props.group.code_snippet && isValidCCRConfig(props.group.code_snippet)) {
      try {
        const convertedConfig = convertCCRConfigToChannel(
          props.group.code_snippet,
          newChannelType,
          props.group.name,
          authKey.value || "全局key"
        );
        updateData.code_snippet = convertedConfig;

        message.success(
          `渠道类型已切换至 ${getChannelDisplayName(newChannelType)}，CCR 配置已自动转换`
        );
      } catch (error) {
        console.error("CCR 配置转换失败:", error);
        message.warning(
          `渠道类型已切换至 ${getChannelDisplayName(newChannelType)}，但 CCR 配置转换失败，请手动调整`
        );
      }
    } else {
      message.success(`渠道类型已切换至 ${getChannelDisplayName(newChannelType)}`);
    }

    // 调用 API 更新分组
    const updatedGroup = await keysApi.updateGroup(props.group.id, updateData);

    // 通知父组件更新数据
    emit("group-updated", updatedGroup);
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
  // 检查路由参数，如果有 tab=settings，则切换到设置标签页
  if (route.query.tab === "settings") {
    activeTab.value = "settings";
  }
});

watch(
  () => props.group,
  (newGroup, oldGroup) => {
    // 只有在组实际发生变化时才完全重置页面
    if (!oldGroup || !newGroup || oldGroup.id !== newGroup.id) {
      // 如果当前处于编辑模式，不要重置标签页状态
      // 这是为了避免在通过右键菜单编辑其他分组时，标签页被意外重置
      const shouldPreserveTabState = isEditMode.value;

      // 重置除标签页外的其他状态
      showCopyModal.value = false;
      isEditingDescription.value = false;
      editingDescription.value = "";

      // 如果不在编辑模式，才重置标签页相关状态
      if (!shouldPreserveTabState) {
        isEditMode.value = false;
        detailsActiveTab.value = "basic";
        activeTab.value = "description";
      }
    }
    loadStats();
  }
);

// 监听路由参数变化
watch(
  () => route.query.tab,
  newTab => {
    if (newTab === "settings") {
      activeTab.value = "settings";
    }
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
  activeTab.value = "details";
  isEditMode.value = true;
}

function handleCopy() {
  showCopyModal.value = true;
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

// 进入编辑模式
function enterEditMode() {
  isEditMode.value = true;
}

// 退出编辑模式
function exitEditMode() {
  isEditMode.value = false;
}

// 切换编辑模式
function toggleEditMode() {
  if (isEditMode.value) {
    exitEditMode();
  } else {
    enterEditMode();
  }
}

// 处理编辑模式下的设置更新
function handleEditModeGroupUpdated(newGroup: Group) {
  if (newGroup) {
    emit("updated", newGroup);
    // 重新加载当前分组的统计数据
    loadStats();
    // 退出编辑模式
    exitEditMode();
  }
}

// 编辑模式提交函数
async function handleEditModeSubmit() {
  const currentFormRef =
    editModeFormRefs.value[detailsActiveTab.value as keyof typeof editModeFormRefs.value];
  if (!currentFormRef) {
    message.error("表单组件未加载");
    return;
  }

  try {
    editModeLoading.value = true;
    // 调用当前标签页对应的表单的提交方法
    await currentFormRef.handleSubmit();
  } finally {
    editModeLoading.value = false;
  }
}

// 暴露编辑方法供父组件调用
defineExpose({
  handleEdit,
});

const isShow = ref(false);
</script>

<template>
  <div class="group-info-container">
    <n-card :bordered="false" class="group-info-card">
      <template #header>
        <div class="card-header">
          <!-- 第 1 列：显示名称和按钮 -->
          <div class="header-column-1">
            <div class="column-row-1">
              <h3 class="group-title">
                {{ group ? group.display_name || group.name : "请选择分组" }}
              </h3>
            </div>
            <div class="column-row-2">
              <!-- 应用到ccr按钮已移至CCRSettingsCard组件 -->
              <span class="group-button-placeholder">&nbsp;</span>
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

      <!-- <n-divider style="margin: 0; margin-bottom: 12px" /> -->
      <!-- 统计摘要区 -->
      <div class="stats-summary" v-show="isShow">
        <n-spin :show="loading" size="small">
          <div class="stats-row">
            <!-- 密钥数量统计 -->
            <div class="stat-item">
              <span class="stat-label">密钥数量：{{ stats?.key_stats?.total_keys ?? 0 }}</span>
              <div class="stat-value">
                <n-tooltip trigger="hover">
                  <template #trigger>
                    <span class="stat-number stat-success">
                      {{ stats?.key_stats?.active_keys ?? 0 }}
                    </span>
                  </template>
                  有效密钥数
                </n-tooltip>
                <span class="stat-separator">/</span>
                <n-tooltip trigger="hover">
                  <template #trigger>
                    <span class="stat-number stat-error">
                      {{ stats?.key_stats?.invalid_keys ?? 0 }}
                    </span>
                  </template>
                  无效密钥数
                </n-tooltip>
              </div>
            </div>

            <!-- 1小时请求统计 -->
            <div class="stat-item">
              <span class="stat-label">
                1小时请求：{{ formatNumber(stats?.hourly_stats?.total_requests ?? 0) }}
              </span>
              <div class="stat-value">
                <n-tooltip trigger="hover">
                  <template #trigger>
                    <span class="stat-number stat-error">
                      {{ formatNumber(stats?.hourly_stats?.failed_requests ?? 0) }}
                    </span>
                  </template>
                  近1小时失败请求
                </n-tooltip>
                <span class="stat-separator">/</span>
                <n-tooltip trigger="hover">
                  <template #trigger>
                    <span class="stat-number stat-error">
                      {{ formatPercentage(stats?.hourly_stats?.failure_rate ?? 0) }}
                    </span>
                  </template>
                  近1小时失败率
                </n-tooltip>
              </div>
            </div>

            <!-- 24小时请求统计 -->
            <div class="stat-item">
              <span class="stat-label">
                24小时请求：{{ formatNumber(stats?.daily_stats?.total_requests ?? 0) }}
              </span>
              <div class="stat-value">
                <n-tooltip trigger="hover">
                  <template #trigger>
                    <span class="stat-number stat-error">
                      {{ formatNumber(stats?.daily_stats?.failed_requests ?? 0) }}
                    </span>
                  </template>
                  近24小时失败请求
                </n-tooltip>
                <span class="stat-separator">/</span>
                <n-tooltip trigger="hover">
                  <template #trigger>
                    <span class="stat-number stat-error">
                      {{ formatPercentage(stats?.daily_stats?.failure_rate ?? 0) }}
                    </span>
                  </template>
                  近24小时失败率
                </n-tooltip>
              </div>
            </div>

            <!-- 近7天请求统计 -->
            <div class="stat-item">
              <span class="stat-label">
                近7天请求：{{ formatNumber(stats?.weekly_stats?.total_requests ?? 0) }}
              </span>
              <div class="stat-value">
                <n-tooltip trigger="hover">
                  <template #trigger>
                    <span class="stat-number stat-error">
                      {{ formatNumber(stats?.weekly_stats?.failed_requests ?? 0) }}
                    </span>
                  </template>
                  近7天失败请求
                </n-tooltip>
                <span class="stat-separator">/</span>
                <n-tooltip trigger="hover">
                  <template #trigger>
                    <span class="stat-number stat-error">
                      {{ formatPercentage(stats?.weekly_stats?.failure_rate ?? 0) }}
                    </span>
                  </template>
                  近7天失败率
                </n-tooltip>
              </div>
            </div>
          </div>
        </n-spin>
      </div>
      <!-- <n-divider style="margin: 0" /> -->

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

          <!-- 详细信息标签页 (索引 1) -->
          <n-tab-pane name="details" tab="详细信息">
            <div class="details-content">
              <div class="details-layout">
                <!-- 左侧按钮区域 -->
                <div class="buttons-area">
                  <!-- 模式切换按钮 -->
                  <n-button
                    type="primary"
                    @click="toggleEditMode"
                    size="small"
                    class="mode-toggle-btn"
                  >
                    {{ isEditMode ? "查看模式" : "编辑模式" }}
                  </n-button>

                  <!-- 保存按钮 -->
                  <n-button
                    type="primary"
                    @click="handleEditModeSubmit"
                    :loading="editModeLoading"
                    :disabled="!isEditMode"
                    size="small"
                    class="save-btn"
                  >
                    保存
                  </n-button>

                  <!-- 取消按钮 -->
                  <n-button
                    @click="exitEditMode"
                    :disabled="!isEditMode"
                    size="small"
                    class="cancel-btn"
                  >
                    取消
                  </n-button>
                </div>

                <!-- 右侧内容区域 -->
                <div class="content-area">
                  <n-tabs v-model:value="detailsActiveTab" type="line" animated>
                    <!-- 基础信息 Tab -->
                    <n-tab-pane name="basic" tab="基础信息">
                      <!-- 查看模式 -->
                      <div v-if="!isEditMode" class="tab-content">
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
                                  <n-button-group
                                    size="small"
                                    class="key-actions"
                                    v-if="group?.proxy_keys"
                                  >
                                    <n-tooltip trigger="hover">
                                      <template #trigger>
                                        <n-button
                                          quaternary
                                          circle
                                          @click="showProxyKeys = !showProxyKeys"
                                        >
                                          <template #icon>
                                            <n-icon
                                              :component="
                                                showProxyKeys ? EyeOffOutline : EyeOutline
                                              "
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

                      <!-- 编辑模式 -->
                      <div v-else class="tab-content">
                        <group-settings-form
                          :group="group"
                          section="basic"
                          @updated="handleEditModeGroupUpdated"
                          :ref="el => (editModeFormRefs.basic = el)"
                        />
                      </div>
                    </n-tab-pane>

                    <!-- 上游地址 Tab -->
                    <n-tab-pane name="upstream" tab="上游地址">
                      <!-- 查看模式 -->
                      <div v-if="!isEditMode" class="tab-content">
                        <n-form label-placement="left">
                          <n-form-item
                            v-for="(upstream, index) in group?.upstreams ?? []"
                            :key="index"
                            class="upstream-item"
                            :label="`上游 ${index + 1}:`"
                          >
                            <span class="upstream-weight">
                              <n-tag size="small" type="info">权重: {{ upstream.weight }}</n-tag>
                            </span>
                            <n-input
                              class="upstream-url"
                              :value="upstream.url"
                              readonly
                              size="small"
                            />
                          </n-form-item>
                        </n-form>
                      </div>

                      <!-- 编辑模式 -->
                      <div v-else class="tab-content">
                        <group-settings-form
                          :group="group"
                          section="upstream"
                          @updated="handleEditModeGroupUpdated"
                          :ref="el => (editModeFormRefs.upstream = el)"
                        />
                      </div>
                    </n-tab-pane>

                    <!-- 高级配置 Tab -->
                    <n-tab-pane name="advanced" tab="高级配置">
                      <!-- 查看模式 -->
                      <div v-if="!isEditMode" class="tab-content">
                        <div v-if="hasAdvancedConfig">
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
                                    <div class="tooltip-title">
                                      {{ getConfigDisplayName(key) }}
                                    </div>
                                    <div class="tooltip-description">
                                      {{ getConfigDescription(key) }}
                                    </div>
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
                                  <n-tag
                                    :type="rule.action === 'remove' ? 'error' : 'default'"
                                    size="small"
                                  >
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
                        <div v-else class="no-advanced-config">
                          <p>暂无高级配置</p>
                        </div>
                      </div>

                      <!-- 编辑模式 -->
                      <div v-else class="tab-content">
                        <group-settings-form
                          :group="group"
                          section="advanced"
                          @updated="handleEditModeGroupUpdated"
                          :ref="el => (editModeFormRefs.advanced = el)"
                        />
                      </div>
                    </n-tab-pane>
                  </n-tabs>
                </div>
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
.delete-btn,
.apply-to-ccr-btn {
  opacity: 0.8;
  transition: opacity 0.2s ease;
  flex-shrink: 0;
}

.channel-switch-btn:hover,
.copy-btn:hover,
.edit-btn:hover,
.delete-btn:hover,
.apply-to-ccr-btn:hover {
  opacity: 1;
}

.apply-to-ccr-btn {
  font-size: 0.85rem;
  padding: 4px 12px;
  min-width: 0;
}

/* 按钮占位符样式 */
.group-button-placeholder {
  font-size: 0.9rem;
  color: transparent;
  font-weight: 500;
  visibility: hidden;
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
  padding: 12px 16px;
  background: rgba(102, 126, 234, 0.02);
  border-radius: var(--border-radius-sm);
  border: 1px solid rgba(102, 126, 234, 0.1);
}

.stats-row {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
}

.stat-item {
  flex: 1;
  min-width: 180px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.stat-label {
  font-size: 0.8rem;
  color: #64748b;
  font-weight: 500;
  white-space: nowrap;
}

.stat-value {
  display: flex;
  align-items: center;
  gap: 6px;
}

.stat-number {
  font-size: 1.1rem;
  font-weight: 600;
  font-family: monospace;
}

.stat-success {
  color: #059669;
}

.stat-error {
  color: #dc2626;
}

.stat-separator {
  color: #9ca3af;
  font-weight: 500;
  font-size: 0.9rem;
}

/* 移动端响应式 */
@media (max-width: 768px) {
  .stats-row {
    flex-direction: column;
    gap: 12px;
    align-items: stretch;
  }

  .stat-item {
    min-width: 0;
    flex: none;
  }
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
  margin-top: -10px;
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

/* 详细信息标签页查看模式下的表单紧凑样式 */
.tab-content :deep(.n-form-item) {
  margin-bottom: 3px;
}

.tab-content :deep(.n-form-item-feedback-wrapper) {
  min-height: 0;
}

/* 分组描述区域样式 */
.group-description-section {
  margin: 12px 0;
  padding: 12px 16px;
  background: rgba(102, 126, 234, 0.05);
  border-radius: var(--border-radius-sm);
  border-left: 4px solid rgba(102, 126, 234, 0.3);
  max-height: 300px; /* 限制最大高度为400px */
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
  height: 300px; /* 固定高度为 300px */
}

/* 描述编辑框样式 */
.description-textarea {
  height: 275px; /* 固定高度为 275px */
}

.description-textarea :deep(.n-input__textarea-el) {
  height: 275px !important; /* 固定高度为 275px */
  min-height: 275px !important;
  resize: none !important;
  overflow-y: auto !important;
}

/* 片段内容专用样式 */
.code-snippet-content {
  font-family: "Monaco", "Menlo", "Ubuntu Mono", monospace !important;
  font-size: 0.85rem;
  line-height: 1.4;
}

.code-snippet-textarea :deep(.n-input__textarea-el) {
  font-family: "Monaco", "Menlo", "Ubuntu Mono", monospace !important;
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

/* 查看/编辑模式切换相关样式 */
.mode-switch-buttons {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid #e5e7eb;
}

.view-mode .detail-section:first-child {
  margin-top: 0;
}

.edit-mode .edit-content {
  margin-top: 0;
}

/* 编辑模式下的内容样式 */
.edit-content {
  width: 100%;
}

/* 编辑模式下隐藏 GroupSettingsTabs 原本的保存按钮 */
.edit-content :deep(.top-right-save-button) {
  display: none !important;
}

/* 新的详细信息布局样式 */
.details-layout {
  display: flex;
  gap: 20px;
  height: 100%;
}

.buttons-area {
  flex: 0 0 120px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding-right: 20px;
  border-right: 1px solid #e5e7eb;
}

.mode-toggle-btn {
  width: 100%;
}

.save-btn,
.cancel-btn {
  width: 100%;
}

.content-area {
  flex: 1;
  min-width: 0;
}

.tab-content {
  padding: 4px 0;
}

.no-advanced-config {
  text-align: center;
  color: #9ca3af;
  padding: 40px 20px;
  font-size: 0.9rem;
}

/* 移动端适配 */
@media (max-width: 768px) {
  .details-layout {
    flex-direction: column;
    gap: 16px;
  }

  .buttons-area {
    flex: none;
    flex-direction: row;
    padding-right: 0;
    padding-bottom: 16px;
    border-right: none;
    border-bottom: 1px solid #e5e7eb;
  }

  .mode-toggle-btn,
  .save-btn,
  .cancel-btn {
    width: auto;
    flex: 1;
  }
}
</style>
