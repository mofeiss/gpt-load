<script setup lang="ts">
import { keysApi } from "@/api/keys";
import { settingsApi } from "@/api/settings";
import ProxyKeysInput from "@/components/common/ProxyKeysInput.vue";
import type { Group, GroupConfigOption, UpstreamInfo } from "@/types/models";
import { Add, HelpCircleOutline, Remove } from "@vicons/ionicons5";
import {
  NButton,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NInputNumber,
  NSelect,
  NSwitch,
  NTooltip,
  useMessage,
  type FormRules,
} from "naive-ui";
import { computed, reactive, ref, watch } from "vue";

interface Props {
  group?: Group | null;
}

interface Emits {
  (e: "success", value: Group): void;
  (e: "updated", value: Group): void;
}

// 配置项类型
interface ConfigItem {
  key: string;
  value: number | string | boolean;
}

// Header规则类型
interface HeaderRuleItem {
  key: string;
  value: string;
  action: "set" | "remove";
}

const props = withDefaults(defineProps<Props>(), {
  group: null,
});

const emit = defineEmits<Emits>();

const message = useMessage();
const loading = ref(false);
const formRef = ref();
const activeTab = ref("basic");

// Tab 配置
const tabs = [
  { key: "placeholder", label: "", disabled: true }, // 占位tab，用于保存按钮定位
  { key: "basic", label: "基础信息" },
  { key: "upstream", label: "上游地址" },
  { key: "advanced", label: "高级配置" },
];

// 表单数据接口
interface GroupFormData {
  name: string;
  display_name: string;
  description: string;
  upstreams: UpstreamInfo[];
  channel_type: "anthropic" | "gemini" | "openai";
  sort: number;
  test_model: string;
  validation_endpoint: string;
  param_overrides: string;
  config: Record<string, number | string | boolean>;
  configItems: ConfigItem[];
  header_rules: HeaderRuleItem[];
  proxy_keys: string;
  blacklist_threshold: number | null;
  force_http11?: boolean;
  code_snippet: string;
}

// 表单数据
const formData = reactive<GroupFormData>({
  name: "",
  display_name: "",
  description: "",
  upstreams: [
    {
      url: "",
      weight: 1,
    },
  ] as UpstreamInfo[],
  channel_type: "openai",
  sort: 1,
  test_model: "",
  validation_endpoint: "",
  param_overrides: "",
  config: {},
  configItems: [] as ConfigItem[],
  header_rules: [] as HeaderRuleItem[],
  proxy_keys: "",
  blacklist_threshold: null,
  code_snippet: "",
});

const channelTypeOptions = ref<{ label: string; value: string }[]>([]);
const configOptions = ref<GroupConfigOption[]>([]);
const channelTypesFetched = ref(false);
const configOptionsFetched = ref(false);
const globalBlacklistThreshold = ref<number>(3);

// 跟踪用户是否已手动修改过字段（仅在新增模式下使用）
const userModifiedFields = ref({
  test_model: false,
  upstream: false,
});

// 根据渠道类型动态生成占位符提示
const testModelPlaceholder = computed(() => {
  switch (formData.channel_type) {
    case "openai":
      return "gpt-4.1-nano";
    case "gemini":
      return "gemini-2.0-flash-lite";
    case "anthropic":
      return "claude-3-haiku-20240307";
    default:
      return "请输入模型名称";
  }
});

const upstreamPlaceholder = computed(() => {
  switch (formData.channel_type) {
    case "openai":
      return "https://api.openai.com";
    case "gemini":
      return "https://generativelanguage.googleapis.com";
    case "anthropic":
      return "https://api.anthropic.com";
    default:
      return "请输入上游地址";
  }
});

const validationEndpointPlaceholder = computed(() => {
  switch (formData.channel_type) {
    case "openai":
      return "/v1/chat/completions";
    case "anthropic":
      return "/v1/messages?beta=true";
    case "gemini":
      return "";
    default:
      return "请输入验证端点路径";
  }
});

// 表单验证规则
const rules: FormRules = {
  name: [
    {
      required: true,
      message: "请输入分组名称",
      trigger: ["blur", "input"],
    },
    {
      pattern: /^[a-z0-9_-]{3,30}$/,
      message: "只能包含小写字母、数字、中划线或下划线，长度3-30位",
      trigger: ["blur", "input"],
    },
  ],
  channel_type: [
    {
      required: true,
      message: "请选择渠道类型",
      trigger: ["blur", "change"],
    },
  ],
  test_model: [
    {
      required: true,
      message: "请输入测试模型",
      trigger: ["blur", "input"],
    },
  ],
  upstreams: [
    {
      type: "array",
      min: 1,
      message: "至少需要一个上游地址",
      trigger: ["blur", "change"],
    },
  ],
};

// 监听group变化
watch(
  () => props.group,
  group => {
    if (group) {
      if (!channelTypesFetched.value) {
        fetchChannelTypes();
      }
      if (!configOptionsFetched.value) {
        fetchGroupConfigOptions();
      }
      loadGroupData();
    }
  },
  { immediate: true }
);

// 监听渠道类型变化，在新增模式下智能更新默认值
watch(
  () => formData.channel_type,
  (_newChannelType, oldChannelType) => {
    if (!props.group && oldChannelType) {
      if (
        !userModifiedFields.value.test_model ||
        formData.test_model === getOldDefaultTestModel(oldChannelType)
      ) {
        formData.test_model = testModelPlaceholder.value;
        userModifiedFields.value.test_model = false;
      }

      if (
        formData.upstreams.length > 0 &&
        (!userModifiedFields.value.upstream ||
          formData.upstreams[0].url === getOldDefaultUpstream(oldChannelType))
      ) {
        formData.upstreams[0].url = upstreamPlaceholder.value;
        userModifiedFields.value.upstream = false;
      }
    }
  }
);

// 获取旧渠道类型的默认值
function getOldDefaultTestModel(channelType: string): string {
  switch (channelType) {
    case "openai":
      return "gpt-4.1-nano";
    case "gemini":
      return "gemini-2.0-flash-lite";
    case "anthropic":
      return "claude-3-haiku-20240307";
    default:
      return "";
  }
}

function getOldDefaultUpstream(channelType: string): string {
  switch (channelType) {
    case "openai":
      return "https://api.openai.com";
    case "gemini":
      return "https://generativelanguage.googleapis.com";
    case "anthropic":
      return "https://api.anthropic.com";
    default:
      return "";
  }
}

// 加载分组数据（编辑模式）
function loadGroupData() {
  if (!props.group) {
    return;
  }

  const configItems = Object.entries(props.group.config || {}).map(([key, value]) => {
    return {
      key,
      value,
    };
  });

  let blacklistThreshold = null;
  if (props.group.config && "blacklist_threshold" in props.group.config) {
    blacklistThreshold = props.group.config.blacklist_threshold as number;
  }

  Object.assign(formData, {
    name: props.group.name || "",
    display_name: props.group.display_name || "",
    description: props.group.description || "",
    upstreams: props.group.upstreams?.length
      ? [...props.group.upstreams]
      : [{ url: "", weight: 1 }],
    channel_type: props.group.channel_type || "openai",
    sort: props.group.sort || 1,
    test_model: props.group.test_model || "",
    validation_endpoint: props.group.validation_endpoint || "",
    param_overrides: JSON.stringify(props.group.param_overrides || {}, null, 2),
    config: {},
    configItems,
    header_rules: (props.group.header_rules || []).map((rule: HeaderRuleItem) => ({
      key: rule.key || "",
      value: rule.value || "",
      action: (rule.action as "set" | "remove") || "set",
    })),
    proxy_keys: props.group.proxy_keys || "",
    blacklist_threshold: blacklistThreshold,
    force_http11: props.group.force_http11 ?? false,
    code_snippet: props.group.code_snippet || "",
  });
}

async function fetchChannelTypes() {
  const options = (await settingsApi.getChannelTypes()) || [];
  channelTypeOptions.value =
    options?.map((type: string) => ({
      label: type,
      value: type,
    })) || [];
  channelTypesFetched.value = true;
}

// 添加上游地址
function addUpstream() {
  formData.upstreams.push({
    url: "",
    weight: 1,
  });
}

// 删除上游地址
function removeUpstream(index: number) {
  if (formData.upstreams.length > 1) {
    formData.upstreams.splice(index, 1);
  } else {
    message.warning("至少需要保留一个上游地址");
  }
}

async function fetchGroupConfigOptions() {
  const options = await keysApi.getGroupConfigOptions();
  configOptions.value = options || [];

  const blacklistOption = options.find(opt => opt.key === "blacklist_threshold");
  if (blacklistOption && typeof blacklistOption.default_value === "number") {
    globalBlacklistThreshold.value = blacklistOption.default_value;
  }

  configOptionsFetched.value = true;
}

// 添加配置项
function addConfigItem() {
  formData.configItems.push({
    key: "",
    value: "",
  });
}

// 删除配置项
function removeConfigItem(index: number) {
  formData.configItems.splice(index, 1);
}

// 添加Header规则
function addHeaderRule() {
  formData.header_rules.push({
    key: "",
    value: "",
    action: "set",
  });
}

// 删除Header规则
function removeHeaderRule(index: number) {
  formData.header_rules.splice(index, 1);
}

// 规范化Header Key到Canonical格式
function canonicalHeaderKey(key: string): string {
  if (!key) {
    return key;
  }
  return key
    .split("-")
    .map(part => part.charAt(0).toUpperCase() + part.slice(1).toLowerCase())
    .join("-");
}

// 验证Header Key唯一性
function validateHeaderKeyUniqueness(
  rules: HeaderRuleItem[],
  currentIndex: number,
  key: string
): boolean {
  if (!key.trim()) {
    return true;
  }

  const canonicalKey = canonicalHeaderKey(key.trim());
  return !rules.some(
    (rule, index) => index !== currentIndex && canonicalHeaderKey(rule.key.trim()) === canonicalKey
  );
}

// 当配置项的key改变时，设置默认值
function handleConfigKeyChange(index: number, key: string) {
  const option = configOptions.value.find(opt => opt.key === key);
  if (option) {
    formData.configItems[index].value = option.default_value;
  }
}

const getConfigOption = (key: string) => {
  return configOptions.value.find(opt => opt.key === key);
};

// 提交表单
async function handleSubmit() {
  if (loading.value || !props.group?.id) {
    return;
  }

  try {
    await formRef.value?.validate();

    loading.value = true;

    // 验证 JSON 格式
    let paramOverrides = {};
    if (formData.param_overrides) {
      try {
        paramOverrides = JSON.parse(formData.param_overrides);
      } catch {
        message.error("参数覆盖必须是有效的 JSON 格式");
        return;
      }
    }

    // 将configItems转换为config对象
    const config: Record<string, number | string | boolean> = {};
    formData.configItems.forEach((item: ConfigItem) => {
      if (item.key && item.key.trim()) {
        const option = configOptions.value.find(opt => opt.key === item.key);
        if (option && typeof option.default_value === "number" && typeof item.value === "string") {
          const numValue = Number(item.value);
          config[item.key] = isNaN(numValue) ? 0 : numValue;
        } else {
          config[item.key] = item.value;
        }
      }
    });

    // 处理黑名单阈值配置
    if (formData.blacklist_threshold !== null && formData.blacklist_threshold !== undefined) {
      config.blacklist_threshold = formData.blacklist_threshold;
    }

    // 构建提交数据
    const submitData = {
      name: formData.name,
      display_name: formData.display_name,
      description: formData.description,
      upstreams: formData.upstreams.filter((upstream: UpstreamInfo) => upstream.url.trim()),
      channel_type: formData.channel_type,
      sort: formData.sort,
      test_model: formData.test_model,
      validation_endpoint: formData.validation_endpoint,
      param_overrides: paramOverrides,
      config,
      header_rules: formData.header_rules
        .filter((rule: HeaderRuleItem) => rule.key.trim())
        .map((rule: HeaderRuleItem) => ({
          key: rule.key.trim(),
          value: rule.value,
          action: rule.action,
        })),
      proxy_keys: formData.proxy_keys,
      force_http11: formData.force_http11,
      code_snippet: formData.code_snippet,
    };

    const res = await keysApi.updateGroup(props.group.id, submitData);
    emit("updated", res);
    message.success("分组设置已更新");
    window.location.reload();
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <div class="group-settings-tabs">
    <div class="tabs-container">
      <!-- 左侧导航区域 -->
      <div class="tabs-nav-area">
        <!-- 保存按钮覆盖在占位tab上 -->
        <div class="top-right-save-button">
          <n-button type="primary" @click="handleSubmit" :loading="loading" size="small">
            保存设置
          </n-button>
        </div>

        <div class="nav-tabs">
          <div
            v-for="tab in tabs"
            :key="tab.key"
            class="nav-tab-item"
            :class="{
              'nav-tab-active': activeTab === tab.key,
              'nav-tab-disabled': tab.disabled,
              'nav-tab-placeholder': tab.key === 'placeholder',
            }"
            @click="!tab.disabled && (activeTab = tab.key)"
          >
            {{ tab.label }}
          </div>
        </div>
      </div>

      <!-- 右侧内容区域 -->
      <div class="tabs-content-area">
        <!-- 基础信息 Tab -->
        <div class="tab-content" v-show="activeTab === 'basic'">
          <n-form
            ref="formRef"
            :model="formData"
            :rules="rules"
            label-placement="left"
            label-width="120px"
            require-mark-placement="right-hanging"
            class="settings-form"
          >
            <!-- 分组名称和显示名称在同一行 -->
            <div class="form-row">
              <n-form-item label="分组名称" path="name" class="form-item-half">
                <template #label>
                  <div class="form-label-with-tooltip">
                    分组名称
                    <n-tooltip trigger="hover" placement="top">
                      <template #trigger>
                        <n-icon :component="HelpCircleOutline" class="help-icon" />
                      </template>
                      作为API路由的一部分，只能包含小写字母、数字、中划线或下划线，长度3-30位。例如：gemini、openai-2
                    </n-tooltip>
                  </div>
                </template>
                <n-input v-model:value="formData.name" placeholder="gemini" />
              </n-form-item>

              <n-form-item label="显示名称" path="display_name" class="form-item-half">
                <template #label>
                  <div class="form-label-with-tooltip">
                    显示名称
                    <n-tooltip trigger="hover" placement="top">
                      <template #trigger>
                        <n-icon :component="HelpCircleOutline" class="help-icon" />
                      </template>
                      用于在界面上显示的友好名称，可以包含中文和特殊字符。如果不填写，将使用分组名称作为显示名称
                    </n-tooltip>
                  </div>
                </template>
                <n-input v-model:value="formData.display_name" placeholder="Google Gemini" />
              </n-form-item>
            </div>

            <!-- 渠道类型和排序在同一行 -->
            <div class="form-row">
              <n-form-item label="渠道类型" path="channel_type" class="form-item-half">
                <template #label>
                  <div class="form-label-with-tooltip">
                    渠道类型
                    <n-tooltip trigger="hover" placement="top">
                      <template #trigger>
                        <n-icon :component="HelpCircleOutline" class="help-icon" />
                      </template>
                      选择API提供商类型，决定了请求格式和认证方式。支持OpenAI、Gemini、Anthropic等主流AI服务商
                    </n-tooltip>
                  </div>
                </template>
                <n-select
                  v-model:value="formData.channel_type"
                  :options="channelTypeOptions"
                  placeholder="请选择渠道类型"
                />
              </n-form-item>

              <n-form-item label="排序" path="sort" class="form-item-half">
                <template #label>
                  <div class="form-label-with-tooltip">
                    排序
                    <n-tooltip trigger="hover" placement="top">
                      <template #trigger>
                        <n-icon :component="HelpCircleOutline" class="help-icon" />
                      </template>
                      决定分组在列表中的显示顺序，数字越小越靠前。建议使用10、20、30这样的间隔数字，便于后续调整
                    </n-tooltip>
                  </div>
                </template>
                <n-input-number
                  v-model:value="formData.sort"
                  :min="0"
                  placeholder="排序值"
                  style="width: 100%"
                />
              </n-form-item>
            </div>

            <!-- 测试模型和测试路径在同一行 -->
            <div class="form-row">
              <n-form-item label="测试模型" path="test_model" class="form-item-half">
                <template #label>
                  <div class="form-label-with-tooltip">
                    测试模型
                    <n-tooltip trigger="hover" placement="top">
                      <template #trigger>
                        <n-icon :component="HelpCircleOutline" class="help-icon" />
                      </template>
                      用于验证API密钥有效性的模型名称。系统会使用这个模型发送测试请求来检查密钥是否可用，请尽量使用轻量快速的模型
                    </n-tooltip>
                  </div>
                </template>
                <n-input
                  v-model:value="formData.test_model"
                  :placeholder="testModelPlaceholder"
                  @input="() => !props.group && (userModifiedFields.test_model = true)"
                />
              </n-form-item>

              <n-form-item
                label="测试路径"
                path="validation_endpoint"
                class="form-item-half"
                v-if="formData.channel_type !== 'gemini'"
              >
                <template #label>
                  <div class="form-label-with-tooltip">
                    测试路径
                    <n-tooltip trigger="hover" placement="top">
                      <template #trigger>
                        <n-icon :component="HelpCircleOutline" class="help-icon" />
                      </template>
                      <div>
                        自定义用于验证密钥的API端点路径。如果不填写，将使用默认路径：
                        <br />
                        • OpenAI: /v1/chat/completions
                        <br />
                        • Anthropic: /v1/messages?beta=true
                        <br />
                        如需使用非标准路径，请在此填写完整的API路径
                      </div>
                    </n-tooltip>
                  </div>
                </template>
                <n-input
                  v-model:value="formData.validation_endpoint"
                  :placeholder="validationEndpointPlaceholder || '可选，自定义用于验证key的API路径'"
                />
              </n-form-item>

              <div v-else class="form-item-half" />
            </div>

            <!-- 黑名单阈值 -->
            <n-form-item label="黑名单阈值" path="blacklist_threshold">
              <template #label>
                <div class="form-label-with-tooltip">
                  黑名单阈值
                  <n-tooltip trigger="hover" placement="top">
                    <template #trigger>
                      <n-icon :component="HelpCircleOutline" class="help-icon" />
                    </template>
                    <div>
                      此分组下API密钥连续失败多少次后进入黑名单，0为不拉黑。
                      <br />
                      • 如果不设置，将使用全局设定（当前：{{ globalBlacklistThreshold }}）
                      <br />
                      • 如果设置了值，则忽略全局设定，使用此分组的专用配置
                      <br />
                      • 此配置对该分组下的所有API密钥均生效
                    </div>
                  </n-tooltip>
                </div>
              </template>
              <n-input-number
                v-model:value="formData.blacklist_threshold"
                :min="0"
                :placeholder="`全局设定：${globalBlacklistThreshold}`"
                clearable
                style="width: 200px"
              />
            </n-form-item>

            <!-- 代理密钥 -->
            <n-form-item label="代理密钥" path="proxy_keys">
              <template #label>
                <div class="form-label-with-tooltip">
                  代理密钥
                  <n-tooltip trigger="hover" placement="top">
                    <template #trigger>
                      <n-icon :component="HelpCircleOutline" class="help-icon" />
                    </template>
                    分组专用代理密钥，用于访问此分组的代理端点。多个密钥请用逗号分隔。
                  </n-tooltip>
                </div>
              </template>
              <proxy-keys-input
                v-model="formData.proxy_keys"
                placeholder="多个密钥请用英文逗号 , 分隔"
                size="medium"
              />
            </n-form-item>

            <!-- 强制HTTP/1.1 -->
            <n-form-item label="强制HTTP/1.1" path="force_http11">
              <template #label>
                <div class="form-label-with-tooltip">
                  强制HTTP/1.1
                  <n-tooltip trigger="hover" placement="top">
                    <template #trigger>
                      <n-icon :component="HelpCircleOutline" class="help-icon" />
                    </template>
                    如果上游服务不支持HTTP/2，可以尝试开启此选项，强制使用HTTP/1.1进行请求。
                  </n-tooltip>
                </div>
              </template>
              <n-switch v-model:value="formData.force_http11" />
            </n-form-item>
          </n-form>
        </div>

        <!-- 上游地址 Tab -->
        <div class="tab-content" v-show="activeTab === 'upstream'">
          <n-form
            :model="formData"
            :rules="rules"
            label-placement="left"
            label-width="120px"
            require-mark-placement="right-hanging"
            class="settings-form"
          >
            <n-form-item
              v-for="(upstream, index) in formData.upstreams"
              :key="index"
              :label="`上游 ${index + 1}`"
              :path="`upstreams[${index}].url`"
              :rule="{
                required: true,
                message: '',
                trigger: ['blur', 'input'],
              }"
            >
              <template #label>
                <div class="form-label-with-tooltip">
                  上游 {{ index + 1 }}
                  <n-tooltip trigger="hover" placement="top">
                    <template #trigger>
                      <n-icon :component="HelpCircleOutline" class="help-icon" />
                    </template>
                    API服务器的完整URL地址。多个上游可以实现负载均衡和故障转移，提高服务可用性
                  </n-tooltip>
                </div>
              </template>
              <div class="upstream-row">
                <div class="upstream-url">
                  <n-input
                    v-model:value="upstream.url"
                    :placeholder="upstreamPlaceholder"
                    @input="
                      () => !props.group && index === 0 && (userModifiedFields.upstream = true)
                    "
                  />
                </div>
                <div class="upstream-weight">
                  <span class="weight-label">权重</span>
                  <n-tooltip trigger="hover" placement="top" style="width: 100%">
                    <template #trigger>
                      <n-input-number
                        v-model:value="upstream.weight"
                        :min="1"
                        placeholder="权重"
                        style="width: 100%"
                      />
                    </template>
                    负载均衡权重，数值越大被选中的概率越高。例如：权重为2的上游被选中的概率是权重为1的两倍
                  </n-tooltip>
                </div>
                <div class="upstream-actions">
                  <n-button
                    v-if="formData.upstreams.length > 1"
                    @click="removeUpstream(index)"
                    type="error"
                    quaternary
                    circle
                    size="small"
                  >
                    <template #icon>
                      <n-icon :component="Remove" />
                    </template>
                  </n-button>
                </div>
              </div>
            </n-form-item>

            <n-form-item>
              <n-button @click="addUpstream" dashed style="width: 100%">
                <template #icon>
                  <n-icon :component="Add" />
                </template>
                添加上游地址
              </n-button>
            </n-form-item>
          </n-form>
        </div>

        <!-- 高级配置 Tab -->
        <div class="tab-content" v-show="activeTab === 'advanced'">
          <n-form
            :model="formData"
            :rules="rules"
            label-placement="left"
            label-width="120px"
            require-mark-placement="right-hanging"
            class="settings-form"
          >
            <div class="config-section">
              <h5 class="config-title-with-tooltip">
                分组配置
                <n-tooltip trigger="hover" placement="top">
                  <template #trigger>
                    <n-icon :component="HelpCircleOutline" class="help-icon config-help" />
                  </template>
                  针对此分组的专用配置参数，如超时时间、重试次数等。这些配置会覆盖全局默认设置
                </n-tooltip>
              </h5>

              <div class="config-items">
                <n-form-item
                  v-for="(configItem, index) in formData.configItems"
                  :key="index"
                  class="config-item-row"
                  :label="`配置 ${index + 1}`"
                  :path="`configItems[${index}].key`"
                  :rule="{
                    required: true,
                    message: '',
                    trigger: ['blur', 'change'],
                  }"
                >
                  <template #label>
                    <div class="form-label-with-tooltip">
                      配置 {{ index + 1 }}
                      <n-tooltip trigger="hover" placement="top">
                        <template #trigger>
                          <n-icon :component="HelpCircleOutline" class="help-icon" />
                        </template>
                        选择要配置的参数类型，然后设置对应的数值。不同参数有不同的作用和取值范围
                      </n-tooltip>
                    </div>
                  </template>
                  <div class="config-item-content">
                    <div class="config-select">
                      <n-select
                        v-model:value="configItem.key"
                        :options="
                          configOptions
                            .filter(opt => opt.key !== 'blacklist_threshold')
                            .map(opt => ({
                              label: opt.name,
                              value: opt.key,
                              disabled:
                                formData.configItems
                                  .map((item: ConfigItem) => item.key)
                                  ?.includes(opt.key) && opt.key !== configItem.key,
                            }))
                        "
                        placeholder="请选择配置参数"
                        @update:value="value => handleConfigKeyChange(index, value)"
                        clearable
                      />
                    </div>
                    <div class="config-value">
                      <n-tooltip trigger="hover" placement="top">
                        <template #trigger>
                          <n-input-number
                            v-if="typeof configItem.value === 'number'"
                            v-model:value="configItem.value"
                            placeholder="参数值"
                            :precision="0"
                            style="width: 100%"
                          />
                          <n-switch
                            v-else-if="typeof configItem.value === 'boolean'"
                            v-model:value="configItem.value"
                            size="small"
                          />
                          <n-input v-else v-model:value="configItem.value" placeholder="参数值" />
                        </template>
                        {{ getConfigOption(configItem.key)?.description || "设置此配置项的值" }}
                      </n-tooltip>
                    </div>
                    <div class="config-actions">
                      <n-button
                        @click="removeConfigItem(index)"
                        type="error"
                        quaternary
                        circle
                        size="small"
                      >
                        <template #icon>
                          <n-icon :component="Remove" />
                        </template>
                      </n-button>
                    </div>
                  </div>
                </n-form-item>
              </div>

              <div style="margin-top: 12px; padding-left: 120px">
                <n-button
                  @click="addConfigItem"
                  dashed
                  style="width: 100%"
                  :disabled="formData.configItems.length >= configOptions.length - 1"
                >
                  <template #icon>
                    <n-icon :component="Add" />
                  </template>
                  添加配置参数
                </n-button>
              </div>
            </div>

            <div class="config-section">
              <h5 class="config-title-with-tooltip">
                自定义请求头
                <n-tooltip trigger="hover" placement="top">
                  <template #trigger>
                    <n-icon :component="HelpCircleOutline" class="help-icon config-help" />
                  </template>
                  <div>
                    在代理请求转发至上游服务前，对 HTTP 请求头进行添加、覆盖或移除操作。
                    <br />
                    支持动态变量：
                    <br />
                    • ${CLIENT_IP} - 客户端IP地址
                    <br />
                    • ${GROUP_NAME} - 分组名称
                    <br />
                    • ${API_KEY} - 当前轮询的API密钥
                    <br />
                    • ${TIMESTAMP_MS} - 毫秒时间戳
                    <br />
                    • ${TIMESTAMP_S} - 秒时间戳
                  </div>
                </n-tooltip>
              </h5>

              <div class="header-rules-items">
                <n-form-item
                  v-for="(headerRule, index) in formData.header_rules"
                  :key="index"
                  class="header-rule-row"
                  :label="`请求头 ${index + 1}`"
                >
                  <template #label>
                    <div class="form-label-with-tooltip">
                      请求头 {{ index + 1 }}
                      <n-tooltip trigger="hover" placement="top">
                        <template #trigger>
                          <n-icon :component="HelpCircleOutline" class="help-icon" />
                        </template>
                        配置HTTP请求头的名称、值和操作类型。移除操作会删除指定的请求头
                      </n-tooltip>
                    </div>
                  </template>
                  <div class="header-rule-content">
                    <div class="header-name">
                      <n-input
                        v-model:value="headerRule.key"
                        placeholder="Header名称"
                        :status="
                          !validateHeaderKeyUniqueness(formData.header_rules, index, headerRule.key)
                            ? 'error'
                            : undefined
                        "
                      />
                      <div
                        v-if="
                          !validateHeaderKeyUniqueness(formData.header_rules, index, headerRule.key)
                        "
                        class="error-message"
                      >
                        Header名称重复
                      </div>
                    </div>
                    <div class="header-value" v-if="headerRule.action === 'set'">
                      <n-input
                        v-model:value="headerRule.value"
                        placeholder="支持变量，例如：${CLIENT_IP}"
                      />
                    </div>
                    <div class="header-value removed-placeholder" v-else>
                      <span class="removed-text">将从请求中移除</span>
                    </div>
                    <div class="header-action">
                      <n-tooltip trigger="hover" placement="top">
                        <template #trigger>
                          <n-switch
                            v-model:value="headerRule.action"
                            :checked-value="'remove'"
                            :unchecked-value="'set'"
                            size="small"
                          />
                        </template>
                        开启移除开关将删除此请求头，关闭则添加或覆盖此请求头
                      </n-tooltip>
                    </div>
                    <div class="header-actions">
                      <n-button
                        @click="removeHeaderRule(index)"
                        type="error"
                        quaternary
                        circle
                        size="small"
                      >
                        <template #icon>
                          <n-icon :component="Remove" />
                        </template>
                      </n-button>
                    </div>
                  </div>
                </n-form-item>
              </div>

              <div style="margin-top: 12px; padding-left: 120px">
                <n-button @click="addHeaderRule" dashed style="width: 100%">
                  <template #icon>
                    <n-icon :component="Add" />
                  </template>
                  添加请求头
                </n-button>
              </div>
            </div>

            <div class="config-section">
              <n-form-item path="param_overrides">
                <template #label>
                  <div class="form-label-with-tooltip">
                    参数覆盖
                    <n-tooltip trigger="hover" placement="top">
                      <template #trigger>
                        <n-icon :component="HelpCircleOutline" class="help-icon config-help" />
                      </template>
                      使用JSON格式定义要覆盖的API请求参数。例如： {"temperature":
                      0.7}。这些参数会在发送请求时合并到原始参数中
                    </n-tooltip>
                  </div>
                </template>
                <n-input
                  v-model:value="formData.param_overrides"
                  type="textarea"
                  placeholder='{"temperature": 0.7}'
                  :rows="4"
                />
              </n-form-item>
            </div>
          </n-form>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.group-settings-tabs {
  width: 100%;
  height: 100%;
}

/* 新的自定义tab布局 */
.tabs-container {
  display: flex;
  height: 100%;
  max-height: 70vh;
  position: relative; /* 为悬浮按钮提供定位上下文 */
}

.tabs-nav-area {
  width: 140px;
  flex-shrink: 0;
  border-right: 1px solid #e5e7eb;
  background-color: #fafbfc;
  position: relative; /* 为按钮提供定位上下文 */
}

.nav-tabs {
  padding: 8px 0;
}

.nav-tab-item {
  display: block;
  padding: 12px 16px;
  margin: 2px 8px;
  border-radius: 8px;
  font-weight: 500;
  font-size: 14px;
  color: #64748b;
  cursor: pointer;
  transition: all 0.2s ease;
  user-select: none;
}

.nav-tab-item:hover {
  background-color: rgba(102, 126, 234, 0.05);
  color: #475569;
}

.nav-tab-active {
  background-color: rgba(102, 126, 234, 0.1);
  color: #667eea;
  font-weight: 600;
}

.nav-tab-disabled {
  opacity: 0.3;
  cursor: not-allowed;
  pointer-events: none;
  background-color: transparent !important;
}

.nav-tab-placeholder {
  height: 48px; /* 增加占位tab的高度，为保存按钮提供足够空间 */
  padding: 14px 16px; /* 调整padding保持视觉平衡 */
  margin-bottom: 8px; /* 增加与下一个tab的间距 */
}

.tabs-content-area {
  flex: 1;
  overflow: hidden;
}

/* 保存按钮覆盖在占位tab上 */
.top-right-save-button {
  position: absolute;
  top: 10px; /* 覆盖占位tab的位置 */
  left: 8px;
  right: 8px; /* 占满tab宽度 */
  height: 48px; /* 与占位tab高度保持一致 */
  display: flex;
  align-items: center; /* 垂直居中 */
  z-index: 9999; /* 大幅提高z-index确保按钮在最顶层 */
  pointer-events: auto; /* 确保按钮可点击 */
}

.top-right-save-button .n-button {
  width: 100%; /* 按钮填满占位tab宽度 */
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
  position: relative; /* 确保按钮本身也有层级 */
  z-index: 1; /* 相对于容器的层级 */
}

.tab-content {
  padding: 16px 24px; /* 恢复正常padding，按钮悬浮不占用空间 */
  height: 100%;
  overflow-y: auto;
}

.settings-form {
  padding: 0;
}

.form-section {
  margin-top: 20px;
}

.form-section:first-child {
  margin-top: 12px;
}

.section-title {
  font-size: 1rem;
  font-weight: 600;
  color: #374151;
  margin: 0 0 12px 0;
  padding-bottom: 8px;
  border-bottom: 2px solid rgba(102, 126, 234, 0.1);
}

:deep(.n-form-item-label) {
  font-weight: 500;
}

:deep(.n-form-item-blank) {
  flex-grow: 1;
}

:deep(.n-input) {
  --n-border-radius: 6px;
}

:deep(.n-select) {
  --n-border-radius: 6px;
}

:deep(.n-input-number) {
  --n-border-radius: 6px;
}

:deep(.n-form-item) {
  margin-bottom: 16px;
}

:deep(.n-form-item-feedback-wrapper) {
  min-height: 10px;
}

.config-section {
  margin-top: 16px;
}

.config-title {
  font-size: 0.9rem;
  font-weight: 600;
  color: #374151;
  margin: 0 0 12px 0;
}

.form-label {
  margin-left: 25px;
  margin-right: 10px;
  height: 34px;
  line-height: 34px;
  font-weight: 500;
}

/* Tooltip相关样式 */
.form-label-with-tooltip {
  display: flex;
  align-items: center;
  gap: 6px;
}

.help-icon {
  color: #9ca3af;
  font-size: 14px;
  cursor: help;
  transition: color 0.2s ease;
}

.help-icon:hover {
  color: #667eea;
}

.section-title-with-tooltip {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
}

.section-help {
  font-size: 16px;
}

.collapse-header-with-tooltip {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 500;
}

.collapse-help {
  font-size: 14px;
}

.config-title-with-tooltip {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.9rem;
  font-weight: 600;
  color: #374151;
  margin: 0 0 12px 0;
}

.config-help {
  font-size: 13px;
}

/* 增强表单样式 */
:deep(.n-form-item-label) {
  font-weight: 500;
  color: #374151;
}

:deep(.n-input) {
  --n-border-radius: 8px;
  --n-border: 1px solid #e5e7eb;
  --n-border-hover: 1px solid #667eea;
  --n-border-focus: 1px solid #667eea;
  --n-box-shadow-focus: 0 0 0 2px rgba(102, 126, 234, 0.1);
}

:deep(.n-select) {
  --n-border-radius: 8px;
}

:deep(.n-input-number) {
  --n-border-radius: 8px;
}

:deep(.n-button) {
  --n-border-radius: 8px;
}

/* 美化tooltip */
:deep(.n-tooltip__trigger) {
  display: inline-flex;
  align-items: center;
}

:deep(.n-tooltip) {
  --n-font-size: 13px;
  --n-border-radius: 8px;
}

:deep(.n-tooltip .n-tooltip__content) {
  max-width: 320px;
  line-height: 1.5;
}

:deep(.n-tooltip .n-tooltip__content div) {
  white-space: pre-line;
}

/* 表单行布局 */
.form-row {
  display: flex;
  gap: 20px;
  align-items: flex-start;
}

.form-item-half {
  flex: 1;
  width: 50%;
}

/* 上游地址行布局 */
.upstream-row {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
}

.upstream-url {
  flex: 1;
}

.upstream-weight {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 0 0 140px;
}

.weight-label {
  font-weight: 500;
  color: #374151;
  white-space: nowrap;
}

.upstream-actions {
  flex: 0 0 32px;
  display: flex;
  justify-content: center;
}

/* 配置项行布局 */
.config-item-row {
  margin-bottom: 12px;
}

.config-item-content {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
}

.config-select {
  flex: 0 0 200px;
}

.config-value {
  flex: 1;
}

.config-actions {
  flex: 0 0 32px;
  display: flex;
  justify-content: center;
}

@media (max-width: 768px) {
  .form-row {
    flex-direction: column;
    gap: 0;
  }

  .form-item-half {
    width: 100%;
  }

  .section-title {
    font-size: 0.9rem;
  }

  .upstream-row,
  .config-item-content {
    flex-direction: column;
    gap: 8px;
    align-items: stretch;
  }

  .upstream-weight {
    flex: 1;
    flex-direction: column;
    align-items: flex-start;
  }

  .config-value {
    flex: 1;
  }

  .upstream-actions,
  .config-actions {
    justify-content: flex-end;
  }
}

/* Header规则相关样式 */
.header-rule-row {
  margin-bottom: 12px;
}

.header-rule-content {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  width: 100%;
}

.header-name {
  flex: 0 0 200px;
  position: relative;
}

.header-value {
  flex: 1;
  display: flex;
  align-items: center;
  min-height: 34px;
}

.header-value.removed-placeholder {
  justify-content: center;
  background-color: #f5f5f5;
  border: 1px solid #e0e0e0;
  border-radius: 6px;
  padding: 0 12px;
}

.removed-text {
  color: #999;
  font-style: italic;
  font-size: 13px;
}

.header-action {
  flex: 0 0 50px;
  display: flex;
  justify-content: center;
  align-items: center;
  height: 34px;
}

.header-actions {
  flex: 0 0 32px;
  display: flex;
  justify-content: center;
  align-items: flex-start;
  height: 34px;
}

.error-message {
  position: absolute;
  top: 100%;
  left: 0;
  font-size: 12px;
  color: #d03050;
  margin-top: 2px;
}

@media (max-width: 768px) {
  .header-rule-content {
    flex-direction: column;
    gap: 8px;
    align-items: stretch;
  }

  .header-name,
  .header-value {
    flex: 1;
  }

  .header-actions {
    justify-content: flex-end;
  }
}

/* 移动端适配 */
@media (max-width: 768px) {
  .tabs-container {
    flex-direction: column;
    max-height: none;
  }

  .tabs-nav-area {
    width: 100%;
    border-right: none;
    border-bottom: 1px solid #e5e7eb;
  }

  .nav-tabs {
    display: flex;
    padding: 8px;
    overflow-x: auto;
  }

  .nav-tab-item {
    flex-shrink: 0;
    margin: 0 4px;
    white-space: nowrap;
  }

  .tab-content {
    max-height: 60vh;
    padding: 16px; /* 恢复正常padding，按钮悬浮覆盖 */
  }

  .top-right-save-button {
    top: 8px; /* 移动端位置 */
    left: 4px;
    right: 4px;
    z-index: 9999; /* 移动端也使用最高层级 */
  }

  .top-right-save-button .n-button {
    font-size: 12px;
    padding: 6px 8px;
  }
}
</style>
