<script setup lang="ts">
import { logApi } from "@/api/logs";
import type { LogFilter, RequestLog } from "@/types/models";
import { copy } from "@/utils/clipboard";
import { maskKey } from "@/utils/display";
import { isLikelyGzipData, tryGzipDecode } from "@/utils/gzip";
import {
  AlertCircleOutline,
  CopyOutline,
  DocumentTextOutline,
  DownloadOutline,
  EyeOffOutline,
  EyeOutline,
  RefreshOutline,
  Search,
  TrashOutline,
} from "@vicons/ionicons5";
import {
  NButton,
  NCard,
  NCheckbox,
  NCollapse,
  NCollapseItem,
  NDataTable,
  NDatePicker,
  NEllipsis,
  NIcon,
  NInput,
  NModal,
  NPopconfirm,
  NSelect,
  NSpace,
  NSpin,
  NTag,
  useMessage,
} from "naive-ui";
import { computed, h, onMounted, onUnmounted, reactive, ref, watch } from "vue";
import StreamContentDisplay from "./StreamContentDisplay.vue";

// Message instance
const message = useMessage();

interface LogRow extends RequestLog {
  is_key_visible: boolean;
}

// Data
const loading = ref(false);
const logs = ref<LogRow[]>([]);
const currentPage = ref(1);
const pageSize = ref(100);
const total = ref(0);
const totalPages = computed(() => Math.ceil(total.value / pageSize.value));

// 批量选择相关
const selectedLogIds = ref<string[]>([]);
const selectAll = ref(false);

// 自动刷新相关
const autoRefresh = ref(false);
const refreshInterval = ref<number | null>(null);

// Modal for viewing request/response details
const showDetailModal = ref(false);
const selectedLog = ref<LogRow | null>(null);

// 请求信息折叠控制
const isRequestInfoCollapsed = ref(false);
const expandedNames = computed({
  get: () => (isRequestInfoCollapsed.value ? [] : ["request-info"]),
  set: (names: Array<string | number>) => {
    isRequestInfoCollapsed.value = !names.includes("request-info");
    saveRequestInfoCollapseState();
  },
});

// 从localStorage恢复折叠状态
const loadRequestInfoCollapseState = () => {
  const stored = localStorage.getItem("requestInfoCollapsed");
  if (stored !== null) {
    isRequestInfoCollapsed.value = JSON.parse(stored);
  }
};

// 保存折叠状态到localStorage
const saveRequestInfoCollapseState = () => {
  localStorage.setItem("requestInfoCollapsed", JSON.stringify(isRequestInfoCollapsed.value));
};

// 处理checkbox状态变化
const handleCheckboxChange = (checked: boolean) => {
  isRequestInfoCollapsed.value = checked;
  saveRequestInfoCollapseState();
};

// Filters
const filters = reactive({
  group_name: "",
  key_value: "",
  model: "",
  is_success: ref(null),
  status_code: "",
  source_ip: "",
  error_contains: "",
  start_time: null as number | null,
  end_time: null as number | null,
  request_type: ref(null),
});

const successOptions = [
  { label: "成功", value: "true" },
  { label: "失败", value: "false" },
];

const requestTypeOptions = [
  { label: "重试请求", value: "retry" },
  { label: "最终请求", value: "final" },
];

// Fetch data
const loadLogs = async () => {
  loading.value = true;
  try {
    const params: LogFilter = {
      page: currentPage.value,
      page_size: pageSize.value,
      group_name: filters.group_name || undefined,
      key_value: filters.key_value || undefined,
      model: filters.model || undefined,
      is_success:
        filters.is_success === "" || filters.is_success === null
          ? undefined
          : filters.is_success === "true",
      status_code: filters.status_code ? parseInt(filters.status_code, 10) : undefined,
      source_ip: filters.source_ip || undefined,
      error_contains: filters.error_contains || undefined,
      start_time: filters.start_time ? new Date(filters.start_time).toISOString() : undefined,
      end_time: filters.end_time ? new Date(filters.end_time).toISOString() : undefined,
      request_type: filters.request_type || undefined,
    };

    const res = await logApi.getLogs(params);
    if (res.code === 0 && res.data) {
      logs.value = res.data.items.map(log => ({ ...log, is_key_visible: false }));
      total.value = res.data.pagination.total_items;
      // 清空选择状态
      selectedLogIds.value = [];
      selectAll.value = false;
    } else {
      logs.value = [];
      total.value = 0;
      window.$message.error(res.message || "加载日志失败", {
        keepAliveOnHover: true,
        duration: 3000,
        closable: true,
      });
    }
  } catch (_error) {
    window.$message.error("加载日志请求失败", {
      duration: 3000,
    });
  } finally {
    loading.value = false;
  }
};

const formatDateTime = (timestamp: string) => {
  if (!timestamp) {
    return "-";
  }
  const date = new Date(timestamp);
  return date.toLocaleString("zh-CN", { hour12: false }).replace(/\//g, "-");
};

const toggleKeyVisibility = (row: LogRow) => {
  row.is_key_visible = !row.is_key_visible;
};

const viewLogDetails = (row: LogRow) => {
  console.log("【前端插桩】点击查看详情，原始数据:", row);
  console.log("【前端插桩】流式响应状态:", row.is_stream);
  console.log("【前端插桩】响应体长度:", row.response_body?.length || 0);
  console.log("【前端插桩】stream_content 数据:", row.stream_content);

  selectedLog.value = row;
  showDetailModal.value = true;
};

const formatJsonString = (jsonStr: string, tryDecompress = true) => {
  if (!jsonStr) {
    return "";
  }

  let processedStr = jsonStr;

  // 如果启用解压且看起来像 gzip 数据，则尝试解压
  if (tryDecompress && isLikelyGzipData(jsonStr)) {
    processedStr = tryGzipDecode(jsonStr);
  }

  try {
    return JSON.stringify(JSON.parse(processedStr), null, 2);
  } catch {
    return processedStr;
  }
};

// 复制功能
const copyContent = async (content: string, type: string) => {
  const success = await copy(content);
  if (success) {
    message.success(`${type}已复制到剪贴板`);
  } else {
    message.error(`复制${type}失败`);
  }
};

// Columns definition
const createColumns = () => [
  {
    type: "selection" as const,
    width: 50,
  },
  {
    title: "时间",
    key: "timestamp",
    width: 160,
    render: (row: LogRow) => formatDateTime(row.timestamp),
  },
  {
    title: "状态",
    key: "is_success",
    width: 50,
    render: (row: LogRow) =>
      h(
        NTag,
        { type: row.is_success ? "success" : "error", size: "small", round: true },
        { default: () => (row.is_success ? "成功" : "失败") }
      ),
  },
  {
    title: "请求类型",
    key: "request_type",
    width: 90,
    render: (row: LogRow) => {
      return h(
        NTag,
        { type: row.request_type === "retry" ? "warning" : "default", size: "small", round: true },
        { default: () => (row.request_type === "retry" ? "重试请求" : "最终请求") }
      );
    },
  },
  {
    title: "响应类型",
    key: "is_stream",
    width: 80,
    render: (row: LogRow) =>
      h(
        NTag,
        { type: row.is_stream ? "info" : "default", size: "small", round: true },
        { default: () => (row.is_stream ? "流式" : "非流") }
      ),
  },
  { title: "状态码", key: "status_code", width: 60 },
  { title: "耗时(ms)", key: "duration_ms", width: 80 },
  { title: "分组", key: "group_name", width: 120 },
  { title: "模型", key: "model", width: 200 },
  {
    title: "Key",
    key: "key_value",
    width: 200,
    render: (row: LogRow) =>
      h(NSpace, { align: "center", wrap: false }, () => [
        h(
          NEllipsis,
          { style: "max-width: 150px" },
          { default: () => (row.is_key_visible ? row.key_value : maskKey(row.key_value || "")) }
        ),
        h(
          NButton,
          { size: "tiny", text: true, onClick: () => toggleKeyVisibility(row) },
          {
            icon: () =>
              h(NIcon, null, { default: () => h(row.is_key_visible ? EyeOffOutline : EyeOutline) }),
          }
        ),
      ]),
  },
  { title: "源IP", key: "source_ip", width: 140 },
  {
    title: "操作",
    key: "actions",
    width: 100,
    fixed: "right" as const,
    render: (row: LogRow) =>
      h(
        NButton,
        {
          size: "small",
          type: "primary",
          ghost: true,
          onClick: () => viewLogDetails(row),
        },
        {
          icon: () => h(NIcon, null, { default: () => h(DocumentTextOutline) }),
          default: () => "详情",
        }
      ),
  },
];

const columns = createColumns();

// Lifecycle and Watchers
onMounted(() => {
  // 从 localStorage 恢复自动刷新状态
  const savedAutoRefresh = localStorage.getItem("autoRefreshEnabled");
  if (savedAutoRefresh === "true") {
    autoRefresh.value = true;
    refreshInterval.value = window.setInterval(() => {
      loadLogs();
    }, 1000);
  }

  // 恢复请求信息折叠状态
  loadRequestInfoCollapseState();

  loadLogs();
});
onUnmounted(() => {
  if (refreshInterval.value) {
    clearInterval(refreshInterval.value);
  }
});
watch([currentPage, pageSize], loadLogs);

const handleSearch = () => {
  currentPage.value = 1;
  loadLogs();
};

const resetFilters = () => {
  filters.group_name = "";
  filters.key_value = "";
  filters.model = "";
  filters.is_success = null;
  filters.status_code = "";
  filters.source_ip = "";
  filters.error_contains = "";
  filters.start_time = null;
  filters.end_time = null;
  filters.request_type = null;
  handleSearch();
};

const exportLogs = () => {
  const params: Omit<LogFilter, "page" | "page_size"> = {
    group_name: filters.group_name || undefined,
    key_value: filters.key_value || undefined,
    model: filters.model || undefined,
    is_success: filters.is_success === "" ? undefined : filters.is_success === "true",
    status_code: filters.status_code ? parseInt(filters.status_code, 10) : undefined,
    source_ip: filters.source_ip || undefined,
    error_contains: filters.error_contains || undefined,
    start_time: filters.start_time ? new Date(filters.start_time).toISOString() : undefined,
    end_time: filters.end_time ? new Date(filters.end_time).toISOString() : undefined,
    request_type: filters.request_type || undefined,
  };
  logApi.exportLogs(params);
};

const deleteLogs = async () => {
  if (selectedLogIds.value.length === 0) {
    message.warning("请先选择要删除的日志");
    return;
  }

  try {
    const res = await logApi.deleteLogs(selectedLogIds.value);
    if (res.code === 0) {
      message.success(`成功删除 ${selectedLogIds.value.length} 条日志`);
      selectedLogIds.value = [];
      selectAll.value = false;
      loadLogs();
    } else {
      message.error(res.message || "删除日志失败");
    }
  } catch (_error) {
    message.error("删除日志请求失败");
  }
};

const clearLogs = async () => {
  try {
    const res = await logApi.clearLogs();
    if (res.code === 0) {
      message.success("成功清空所有日志");
      selectedLogIds.value = [];
      selectAll.value = false;
      loadLogs();
    } else {
      message.error(res.message || "清空日志失败");
    }
  } catch (_error) {
    message.error("清空日志请求失败");
  }
};

// 自动刷新控制函数
const toggleAutoRefresh = (checked: boolean) => {
  autoRefresh.value = checked;

  // 保存自动刷新状态到 localStorage
  localStorage.setItem("autoRefreshEnabled", checked ? "true" : "false");

  if (checked) {
    refreshInterval.value = window.setInterval(() => {
      loadLogs();
    }, 1000);
  } else {
    if (refreshInterval.value) {
      clearInterval(refreshInterval.value);
      refreshInterval.value = null;
    }
  }
};

function changePage(page: number) {
  currentPage.value = page;
}

function changePageSize(size: number) {
  pageSize.value = size;
  currentPage.value = 1;
}

// 批量选择相关函数
const handleSelectionChange = (rowKeys: (string | number)[]) => {
  selectedLogIds.value = rowKeys as string[];
  selectAll.value = rowKeys.length === logs.value.length && logs.value.length > 0;
};

const selectedCount = computed(() => selectedLogIds.value.length);
</script>

<template>
  <div class="log-table-container">
    <n-space vertical>
      <!-- 工具栏 -->
      <div class="toolbar">
        <div class="filter-section">
          <div class="filter-row">
            <div class="filter-grid">
              <div class="filter-item">
                <n-select
                  v-model:value="filters.is_success"
                  :options="successOptions"
                  size="small"
                  placeholder="状态"
                  clearable
                  @update:value="handleSearch"
                />
              </div>
              <div class="filter-item">
                <n-input
                  v-model:value="filters.status_code"
                  placeholder="状态码"
                  size="small"
                  clearable
                  @keyup.enter="handleSearch"
                />
              </div>
              <div class="filter-item">
                <n-input
                  v-model:value="filters.group_name"
                  placeholder="分组名"
                  size="small"
                  clearable
                  @keyup.enter="handleSearch"
                />
              </div>
              <div class="filter-item">
                <n-input
                  v-model:value="filters.model"
                  placeholder="模型"
                  size="small"
                  clearable
                  @keyup.enter="handleSearch"
                />
              </div>
              <div class="filter-item">
                <n-input
                  v-model:value="filters.key_value"
                  placeholder="密钥"
                  size="small"
                  clearable
                  @keyup.enter="handleSearch"
                />
              </div>
              <div class="filter-item">
                <n-select
                  v-model:value="filters.request_type"
                  :options="requestTypeOptions"
                  size="small"
                  clearable
                  placeholder="请求类型"
                  @update:value="handleSearch"
                />
              </div>
              <div class="filter-item">
                <n-date-picker
                  v-model:value="filters.start_time"
                  type="datetime"
                  clearable
                  size="small"
                  placeholder="开始时间"
                />
              </div>
              <div class="filter-item">
                <n-date-picker
                  v-model:value="filters.end_time"
                  type="datetime"
                  clearable
                  size="small"
                  placeholder="结束时间"
                />
              </div>
              <div class="filter-item">
                <n-input
                  v-model:value="filters.error_contains"
                  placeholder="错误信息"
                  size="small"
                  clearable
                  @keyup.enter="handleSearch"
                />
              </div>
              <div class="filter-actions">
                <n-button ghost size="small" :disabled="loading" @click="handleSearch">
                  <template #icon>
                    <n-icon :component="Search" />
                  </template>
                  搜索
                </n-button>
                <n-button size="small" @click="resetFilters">重置</n-button>
                <n-button ghost size="small" @click="exportLogs">
                  <template #icon>
                    <n-icon :component="DownloadOutline" />
                  </template>
                  导出密钥
                </n-button>
                <n-popconfirm @positive-click="deleteLogs">
                  <template #trigger>
                    <n-button size="small" type="error" :disabled="selectedCount === 0">
                      <template #icon>
                        <n-icon :component="TrashOutline" />
                      </template>
                      删除选中日志 {{ selectedCount > 0 ? `(${selectedCount})` : "" }}
                    </n-button>
                  </template>
                  确定要删除选中的 {{ selectedCount }} 条日志吗？此操作不可撤销。
                </n-popconfirm>
                <n-popconfirm @positive-click="clearLogs">
                  <template #trigger>
                    <n-button size="small" type="warning">
                      <template #icon>
                        <n-icon :component="AlertCircleOutline" />
                      </template>
                      清空所有日志
                    </n-button>
                  </template>
                  确定要清空所有日志吗？此操作不可撤销，请谨慎操作。
                </n-popconfirm>
                <div class="auto-refresh-control">
                  <n-checkbox v-model:checked="autoRefresh" @update:checked="toggleAutoRefresh">
                    自动刷新
                  </n-checkbox>
                  <n-button size="tiny" type="info" ghost @click="loadLogs">
                    <template #icon>
                      <n-icon :component="RefreshOutline" />
                    </template>
                    刷新
                  </n-button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
      <div class="table-main">
        <!-- 表格 -->
        <div class="table-container">
          <n-spin :show="loading">
            <n-data-table
              :columns="columns"
              :data="logs"
              :bordered="false"
              remote
              size="small"
              :scroll-x="1230"
              :row-key="(row: LogRow) => row.id"
              :checked-row-keys="selectedLogIds"
              @update:checked-row-keys="handleSelectionChange"
            />
          </n-spin>
        </div>

        <!-- 分页 -->
        <div class="pagination-container">
          <div class="pagination-info">
            <span>共 {{ total }} 条记录</span>
            <n-select
              v-model:value="pageSize"
              :options="[
                { label: '15条/页', value: 15 },
                { label: '30条/页', value: 30 },
                { label: '50条/页', value: 50 },
                { label: '100条/页', value: 100 },
              ]"
              size="small"
              style="width: 100px; margin-left: 12px"
              @update:value="changePageSize"
            />
          </div>
          <div class="pagination-controls">
            <n-button
              size="small"
              :disabled="currentPage <= 1"
              @click="changePage(currentPage - 1)"
            >
              上一页
            </n-button>
            <span class="page-info">第 {{ currentPage }} 页，共 {{ totalPages }} 页</span>
            <n-button
              size="small"
              :disabled="currentPage >= totalPages"
              @click="changePage(currentPage + 1)"
            >
              下一页
            </n-button>
          </div>
        </div>
      </div>
    </n-space>

    <!-- 详情模态框 -->
    <n-modal
      v-model:show="showDetailModal"
      preset="card"
      style="width: 1200px"
      title="请求详情"
      :show-close="false"
      :mask-closable="true"
    >
      <template #header-extra>
        <n-space align="center" size="small">
          <n-checkbox
            v-model:checked="isRequestInfoCollapsed"
            @update:checked="handleCheckboxChange"
            size="small"
          >
            收缩请求信息
          </n-checkbox>
        </n-space>
      </template>
      <div v-if="selectedLog" class="modal-content-container">
        <div class="modal-content-flex">
          <!-- 头部信息区域(固定高度) -->
          <div class="modal-header-section">
            <!-- 基本信息 -->
            <n-card
              title="基本信息"
              size="small"
              :header-style="{ padding: '8px 12px', fontSize: '13px' }"
            >
              <div class="detail-grid-compact">
                <div class="detail-item-compact">
                  <span class="detail-label-compact">时间:</span>
                  <span class="detail-value-compact">
                    {{ formatDateTime(selectedLog.timestamp) }}
                  </span>
                </div>
                <div class="detail-item-compact">
                  <span class="detail-label-compact">状态:</span>
                  <n-tag :type="selectedLog.is_success ? 'success' : 'error'" size="small">
                    {{ selectedLog.is_success ? "成功" : "失败" }} - {{ selectedLog.status_code }}
                  </n-tag>
                </div>
                <div class="detail-item-compact">
                  <span class="detail-label-compact">耗时:</span>
                  <span class="detail-value-compact">{{ selectedLog.duration_ms }}ms</span>
                </div>
                <div class="detail-item-compact">
                  <span class="detail-label-compact">分组:</span>
                  <span class="detail-value-compact">{{ selectedLog.group_name }}</span>
                </div>
                <div class="detail-item-compact">
                  <span class="detail-label-compact">模型:</span>
                  <span class="detail-value-compact">{{ selectedLog.model }}</span>
                </div>
                <div class="detail-item-compact">
                  <span class="detail-label-compact">请求类型:</span>
                  <n-tag v-if="selectedLog.request_type === 'retry'" type="warning" size="small">
                    重试
                  </n-tag>
                  <n-tag v-else type="default" size="small">最终</n-tag>
                </div>
                <div class="detail-item-compact">
                  <span class="detail-label-compact">响应类型:</span>
                  <n-tag :type="selectedLog.is_stream ? 'info' : 'default'" size="small">
                    {{ selectedLog.is_stream ? "流式" : "非流" }}
                  </n-tag>
                </div>
                <div class="detail-item-compact">
                  <span class="detail-label-compact">源IP:</span>
                  <span class="detail-value-compact">{{ selectedLog.source_ip || "-" }}</span>
                </div>
                <div class="detail-item-compact key-item">
                  <span class="detail-label-compact">密钥:</span>
                  <div class="key-display-compact">
                    <span class="key-value-compact">
                      {{
                        selectedLog.is_key_visible
                          ? selectedLog.key_value || "-"
                          : maskKey(selectedLog.key_value || "")
                      }}
                    </span>
                    <div class="key-actions-compact">
                      <n-button size="tiny" text @click="toggleKeyVisibility(selectedLog)">
                        <template #icon>
                          <n-icon
                            :component="selectedLog.is_key_visible ? EyeOffOutline : EyeOutline"
                          />
                        </template>
                      </n-button>
                      <n-button
                        v-if="selectedLog.key_value"
                        size="tiny"
                        text
                        @click="copyContent(selectedLog.key_value, 'API Key')"
                      >
                        <template #icon>
                          <n-icon :component="CopyOutline" />
                        </template>
                      </n-button>
                    </div>
                  </div>
                </div>
              </div>
            </n-card>

            <!-- 请求信息 (折叠组件) -->
            <n-collapse v-model:expanded-names="expandedNames">
              <n-collapse-item title="请求信息" name="request-info">
                <div class="compact-fields">
                  <div
                    class="compact-field-row"
                    v-if="selectedLog.request_path || selectedLog.upstream_addr"
                  >
                    <div class="compact-field-half" v-if="selectedLog.request_path">
                      <div class="compact-field-header">
                        <span class="compact-field-title">请求路径</span>
                        <n-button
                          size="tiny"
                          text
                          @click="copyContent(selectedLog.request_path, '请求路径')"
                        >
                          <template #icon>
                            <n-icon :component="CopyOutline" />
                          </template>
                        </n-button>
                      </div>
                      <div class="compact-field-content">
                        {{ selectedLog.request_path }}
                      </div>
                    </div>

                    <div class="compact-field-half" v-if="selectedLog.upstream_addr">
                      <div class="compact-field-header">
                        <span class="compact-field-title">上游地址</span>
                        <n-button
                          size="tiny"
                          text
                          @click="copyContent(selectedLog.upstream_addr, '上游地址')"
                        >
                          <template #icon>
                            <n-icon :component="CopyOutline" />
                          </template>
                        </n-button>
                      </div>
                      <div class="compact-field-content">
                        {{ selectedLog.upstream_addr }}
                      </div>
                    </div>
                  </div>

                  <div class="compact-field" v-if="selectedLog.user_agent">
                    <div class="compact-field-header">
                      <span class="compact-field-title">User Agent</span>
                      <n-button
                        size="tiny"
                        text
                        @click="copyContent(selectedLog.user_agent, 'User Agent')"
                      >
                        <template #icon>
                          <n-icon :component="CopyOutline" />
                        </template>
                      </n-button>
                    </div>
                    <div class="compact-field-content">
                      {{ selectedLog.user_agent }}
                    </div>
                  </div>

                  <div class="compact-field" v-if="selectedLog.request_body">
                    <div class="compact-field-header">
                      <span class="compact-field-title">请求内容</span>
                      <n-button
                        size="tiny"
                        text
                        @click="copyContent(formatJsonString(selectedLog.request_body), '请求内容')"
                      >
                        <template #icon>
                          <n-icon :component="CopyOutline" />
                        </template>
                      </n-button>
                    </div>
                    <div class="compact-field-content compact-field-content-large">
                      {{ formatJsonString(selectedLog.request_body) }}
                    </div>
                  </div>
                </div>
              </n-collapse-item>
            </n-collapse>
          </div>

          <!-- 响应信息区域(可伸缩) -->
          <div class="modal-response-section">
            <n-card
              v-if="selectedLog.response_body"
              title="响应信息"
              size="small"
              :header-style="{ padding: '8px 12px', fontSize: '13px' }"
            >
              <!-- 流式响应：双列布局 -->
              <stream-content-display
                v-if="selectedLog.is_stream"
                :stream-content="selectedLog.stream_content"
                :raw-content="selectedLog.response_body"
              />

              <!-- 非流式响应：单列显示原文 -->
              <div v-else class="compact-fields">
                <div class="compact-field">
                  <div class="compact-field-header">
                    <span class="compact-field-title">响应内容</span>
                    <n-button
                      size="tiny"
                      text
                      @click="
                        copyContent(formatJsonString(selectedLog.response_body, true), '响应内容')
                      "
                    >
                      <template #icon>
                        <n-icon :component="CopyOutline" />
                      </template>
                    </n-button>
                  </div>
                  <div class="compact-field-content compact-field-content-large">
                    {{ formatJsonString(selectedLog.response_body, true) }}
                  </div>
                </div>
              </div>
            </n-card>

            <!-- 错误信息 -->
            <n-card
              v-if="selectedLog.error_message"
              title="错误信息"
              size="small"
              :header-style="{ padding: '8px 12px', fontSize: '13px' }"
            >
              <template #header-extra>
                <n-button
                  size="tiny"
                  text
                  ghost
                  @click="copyContent(selectedLog.error_message, '错误信息')"
                >
                  <template #icon>
                    <n-icon :component="CopyOutline" />
                  </template>
                </n-button>
              </template>
              <div class="compact-field compact-field-error">
                <div class="compact-field-content">
                  {{ selectedLog.error_message }}
                </div>
              </div>
            </n-card>
          </div>
        </div>
      </div>
    </n-modal>
  </div>
</template>

<style scoped>
/* 模态窗口布局优化 */
.modal-content-container {
  width: 100%;
  display: flex;
  flex-direction: column;
}

.modal-content-flex {
  display: flex;
  flex-direction: column;
  width: 100%;
}

/* 头部信息区域 - 固定高度，不伸缩 */
.modal-header-section {
  flex-shrink: 0;
  margin-bottom: 12px;
}

/* 响应信息区域 - 自动占据剩余空间 */
.modal-response-section {
  flex-grow: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden; /* 确保内容不会超出模态窗口 */
}
.log-table-container {
  /* background: white; */
  /* border-radius: 8px; */
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.05);
  display: flex;
  flex-direction: column;
  /* height: 100%; */
}
.toolbar {
  background: white;
  border-radius: 8px;
  padding: 16px;
  border-bottom: 1px solid #f0f0f0;
}

.filter-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.filter-row {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end; /* Aligns buttons with the bottom of the filter items */
  gap: 16px;
}

.filter-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  flex: 1 1 auto; /* Let it take available space and wrap */
}

.filter-item {
  flex: 1 1 180px; /* Each item will have a base width of 180px and can grow */
  min-width: 180px; /* Prevent from becoming too narrow */
}

.filter-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.auto-refresh-control {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: auto;
  padding-left: 16px;
  border-left: 1px solid #e5e7eb;
}

@media (max-width: 768px) {
  .pagination-container {
    flex-direction: column;
    gap: 12px;
  }
}

@media (max-width: 480px) {
  .filter-actions {
    width: 100%;
    flex-direction: column;
    align-items: stretch;
  }
  .filter-actions .n-button {
    width: 100%;
  }
  .auto-refresh-control {
    margin-left: 0;
    padding-left: 0;
    border-left: none;
    border-top: 1px solid #e5e7eb;
    padding-top: 8px;
    margin-top: 8px;
    justify-content: center;
  }
}

.table-main {
  background: white;
  border-radius: 8px;
  overflow: hidden;
}
.table-container {
  /* background: white;
  border-radius: 8px; */
  flex: 1;
  overflow: auto;
  position: relative;
}
.empty-container {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
}
.pagination-container {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px;
  border-top: 1px solid #f0f0f0;
}
.pagination-info {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 13px;
  color: #666;
}
.pagination-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}
.page-info {
  font-size: 13px;
  color: #666;
}

.detail-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
}

.detail-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.detail-label {
  font-weight: 500;
  color: #666;
  min-width: 70px;
  flex-shrink: 0;
}

/* 紧凑布局样式 */
.detail-grid-compact {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 4px 10px;
  font-size: 12px;
}

.detail-item-compact {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 1px 0;
  line-height: 1.2;
}

.detail-label-compact {
  font-weight: 500;
  color: #666;
  min-width: 55px;
  flex-shrink: 0;
  font-size: 11px;
}

.detail-value-compact {
  font-size: 11px;
}

.key-item {
  grid-column: 1 / -1;
}

.key-display-compact {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  flex: 1;
  min-width: 0;
}

.key-value-compact {
  font-family: monospace;
  font-size: 11px;
  color: #495057;
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 3px;
  padding: 4px 6px;
  flex: 1;
  min-width: 0;
  word-break: break-all;
  line-height: 1.3;
  max-height: 60px;
  overflow-y: auto;
}

.key-actions-compact {
  display: flex;
  gap: 2px;
  flex-shrink: 0;
  line-height: 24px;
  height: 24px;
}

.compact-fields {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.compact-field {
  border: 1px solid #e9ecef;
  border-radius: 3px;
  padding: 6px;
  background: #f8f9fa;
}

.compact-field-error {
  border: 1px solid #f5c6cb;
  background: #f8d7da;
}

.compact-field-error .compact-field-content {
  color: #721c24;
}

.compact-field-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 3px;
}

.compact-field-title {
  font-weight: 500;
  color: #495057;
  font-size: 11px;
}

.compact-field-content {
  font-family: monospace;
  font-size: 10px;
  line-height: 1.3;
  word-break: break-all;
  white-space: pre-wrap;
  color: #6c757d;
  max-height: 100px;
  overflow-y: auto;
}

.compact-field-content-large {
  max-height: 200px;
}

.compact-field-row {
  display: flex;
  gap: 6px;
}

.compact-field-half {
  flex: 1;
  border: 1px solid #e9ecef;
  border-radius: 3px;
  padding: 6px;
  background: #f8f9fa;
}

.detail-field {
  margin-bottom: 8px;
}

.field-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.field-title {
  font-weight: 500;
  color: #666;
  font-size: 14px;
}

.field-actions {
  display: flex;
  gap: 8px;
}

.field-content {
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 6px;
  padding: 12px;
  font-family: monospace;
  font-size: 13px;
  line-height: 1.5;
  word-break: break-all;
  color: #495057;
}

.key-display {
  display: flex;
  align-items: center;
  gap: 8px;
}

.key-value {
  font-family: monospace;
  font-size: 12px;
  color: #856404;
  background: #fff3cd;
  border: 1px solid #ffeaa7;
  border-radius: 4px;
  padding: 4px 8px;
}

.key-actions {
  display: flex;
  gap: 4px;
}

.empty-content {
  text-align: center;
  color: #6c757d;
  padding: 24px;
  background: #f8f9fa;
  border-radius: 6px;
  font-style: italic;
}

.code-block {
  max-height: 400px;
  overflow-y: auto;
  border-radius: 6px;
}

.error-block {
  max-height: 200px;
}
</style>
