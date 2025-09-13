<script setup lang="ts">
import { onMounted, ref } from "vue";

const iframeRef = ref<HTMLIFrameElement | null>(null);
const isLoading = ref(true);
const loadError = ref(false);

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
});
</script>

<template>
  <div class="ccr-container">
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
  border-radius: var(--border-radius-lg);
  overflow: hidden; /* 确保 iframe 内容遵循圆角 */
}

.ccr-iframe {
  width: 100%;
  height: 100%;
  border: none;
  display: block;
  border-radius: var(--border-radius-lg);
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
