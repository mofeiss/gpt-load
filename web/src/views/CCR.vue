<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useMessage, NSpace, NSpin, NResult, NButton } from "naive-ui";
import axios from "axios";

const iframeRef = ref<HTMLIFrameElement | null>(null);
const isLoading = ref(true);
const loadError = ref(false);
const message = useMessage();

interface Provider {
  name: string;
  api_base_url: string;
  api_key: string;
  models: string[];
}

interface CCRConfig {
  Providers: Provider[];
}

const providers = ref<Provider[]>([]);

async function fetchCCRConfig() {
  try {
    const response = await axios.get("http://127.0.0.1:3456/api/config");
    const config = response.data as CCRConfig;
    if (config.Providers) {
      providers.value = config.Providers;
    }
  } catch (error) {
    console.error("获取 CCR 配置失败:", error);
    message.error("获取 CCR 配置失败");
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

const handleIframeLoad = () => {
  isLoading.value = false;
  loadError.value = false;
};

const handleIframeError = () => {
  isLoading.value = false;
  loadError.value = true;
};

onMounted(() => {
  // 设置iframe加载超时
  setTimeout(() => {
    if (isLoading.value) {
      loadError.value = true;
      isLoading.value = false;
    }
  }, 10000); // 10秒超时

  // 获取 CCR 配置
  fetchCCRConfig();
});
</script>

<template>
  <div class="ccr-container">
    <!-- 提供者按钮区域
    <div v-if="providers.length > 0" class="provider-buttons">
      <n-space :size="8" :wrap="true">
        <n-button
          v-for="provider in providers"
          :key="provider.name"
          size="small"
          @click="copyToClipboard(`/model ${provider.name},${provider.models[0]}`)"
          class="provider-button"
        >
          {{ `${provider.name},${provider.models[0]}` }}
        </n-button>
      </n-space>
    </div> -->

    <div v-if="isLoading" class="loading-container">
      <n-spin size="large">
        <template #description>
          <span class="loading-text">正在加载 CCR 界面...</span>
        </template>
      </n-spin>
    </div>

    <div v-if="loadError" class="error-container">
      <n-result
        status="error"
        title="加载失败"
        description="无法连接到 CCR 服务，请确保服务正在运行"
      >
        <template #footer>
          <n-button
            @click="
              () => {
                loadError = false;
                isLoading = true;
                iframeRef?.contentWindow?.location.reload();
              }
            "
          >
            重新加载
          </n-button>
        </template>
      </n-result>
    </div>

    <iframe
      v-show="!isLoading && !loadError"
      ref="iframeRef"
      src="http://127.0.0.1:3456/ui/"
      class="ccr-iframe"
      @load="handleIframeLoad"
      @error="handleIframeError"
    />
  </div>
</template>

<style scoped>
.ccr-container {
  width: 100%;
  height: calc(100vh - 81px); /* 减去导航栏高度和顶部间距 */
  background: white;
  border-radius: 6px;
  overflow: hidden; /* 确保 iframe 内容遵循圆角 */
}

.provider-buttons {
  padding: 16px;
  border-bottom: 1px solid var(--divider-color);
}

.provider-button {
  width: 200px !important; /* 增加固定宽度 */
  padding: 8px 12px !important; /* 增加上下和左右 padding */
  overflow: hidden !important;
  text-overflow: ellipsis !important;
  white-space: nowrap !important;
  box-sizing: border-box !important; /* 确保 padding 包含在宽度内 */
  height: auto !important; /* 允许高度自适应 */
  min-height: 32px !important; /* 最小高度 */
  line-height: 1.2 !important; /* 调整行高 */
}

.provider-button :deep(.n-button__content) {
  overflow: hidden !important;
  text-overflow: ellipsis !important;
  white-space: nowrap !important;
  width: 100% !important;
}

.ccr-iframe {
  width: 100%;
  height: 100%;
  border: none;
  display: block;
  border-radius: 6px;
}

.loading-container {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  z-index: 10;
}

.loading-text {
  color: var(--text-color-2);
  font-size: 0.9rem;
  margin-top: 8px;
}

.error-container {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  z-index: 10;
  width: 100%;
  max-width: 400px;
  padding: 20px;
}
</style>
