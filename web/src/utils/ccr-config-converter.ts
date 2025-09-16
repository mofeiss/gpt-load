/**
 * CCR 配置转换工具
 * 用于在不同渠道类型间转换自定义模型 JSON 配置
 */

export interface CCRProviderConfig {
  name: string;
  api_base_url: string;
  api_key: string;
  models: string[];
  transformer?: {
    use?: any[];
    [key: string]: any;
  };
}

/**
 * 从现有的 JSON 配置中提取模型列表
 */
export function extractModelsFromConfig(jsonStr: string): string[] {
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

/**
 * 从现有配置中提取其他有用信息（如自定义配置）
 */
export function extractConfigMetadata(jsonStr: string): {
  models: string[];
  customTransformer?: any;
  otherFields?: Record<string, any>;
} {
  if (!jsonStr.trim()) {
    return { models: [] };
  }

  try {
    const parsed = JSON.parse(jsonStr);
    const { name, api_base_url, api_key, models, transformer, ...otherFields } = parsed;

    return {
      models: Array.isArray(models) ? models : [],
      customTransformer: transformer,
      otherFields: Object.keys(otherFields).length > 0 ? otherFields : undefined
    };
  } catch {
    return { models: [] };
  }
}

/**
 * 创建 OpenAI 渠道配置模板
 */
export function createOpenAIChannelConfig(
  groupName: string,
  apiKey: string,
  models: string[] = []
): CCRProviderConfig {
  const modelList = models.length > 0 ? models : ["claude-sonnet-4-20250514"];

  const config: CCRProviderConfig = {
    name: groupName,
    api_base_url: `http://localhost:3001/proxy/${groupName}/v1/chat/completions`,
    api_key: apiKey,
    models: modelList,
    transformer: {
      use: [
        [
          "maxtoken",
          {
            max_tokens: 65535,
          },
        ],
      ],
    }
  };

  // 为每个模型添加 reasoning 配置（OpenAI 特有）
  modelList.forEach(model => {
    if (config.transformer) {
      config.transformer[model] = {
        use: ["reasoning"],
      };
    }
  });

  return config;
}

/**
 * 创建 Anthropic 渠道配置模板
 */
export function createAnthropicChannelConfig(
  groupName: string,
  apiKey: string,
  models: string[] = []
): CCRProviderConfig {
  const modelList = models.length > 0 ? models : ["claude-sonnet-4-20250514"];

  return {
    name: groupName,
    api_base_url: `http://localhost:3001/proxy/${groupName}/v1/messages?beta=true`,
    api_key: apiKey,
    models: modelList,
    transformer: {
      use: ["Anthropic"],
    }
  };
}

/**
 * 创建 Gemini 渠道配置模板
 */
export function createGeminiChannelConfig(
  groupName: string,
  apiKey: string,
  models: string[] = []
): CCRProviderConfig {
  const modelList = models.length > 0 ? models : ["gemini-2.5-pro", "gemini-2.5-flash"];

  return {
    name: groupName,
    api_base_url: `http://localhost:3001/proxy/${groupName}/v1beta/models/`,
    api_key: apiKey,
    models: modelList,
    transformer: {
      use: ["gemini"],
    }
  };
}

/**
 * 转换 CCR 配置到指定渠道类型
 * 这是核心转换函数，保留模型列表并应用新的渠道模板
 */
export function convertCCRConfigToChannel(
  currentJsonStr: string,
  targetChannelType: 'openai' | 'anthropic' | 'gemini',
  groupName: string,
  apiKey: string
): string {
  // 提取现有配置的模型列表
  const existingModels = extractModelsFromConfig(currentJsonStr);

  // 根据目标渠道类型创建新配置（使用正确的 transformer）
  let newConfig: CCRProviderConfig;

  switch (targetChannelType) {
    case 'openai':
      newConfig = createOpenAIChannelConfig(groupName, apiKey, existingModels);
      break;
    case 'anthropic':
      newConfig = createAnthropicChannelConfig(groupName, apiKey, existingModels);
      break;
    case 'gemini':
      newConfig = createGeminiChannelConfig(groupName, apiKey, existingModels);
      break;
    default:
      throw new Error(`不支持的渠道类型: ${targetChannelType}`);
  }

  return JSON.stringify(newConfig, null, 2);
}

/**
 * 获取渠道类型的显示名称
 */
export function getChannelDisplayName(channelType: string): string {
  const channelNames: Record<string, string> = {
    'openai': 'OpenAI',
    'anthropic': 'Anthropic',
    'gemini': 'Gemini'
  };
  return channelNames[channelType] || channelType.toUpperCase();
}

/**
 * 检查 JSON 字符串是否为有效的 CCR 配置
 */
export function isValidCCRConfig(jsonStr: string): boolean {
  if (!jsonStr.trim()) {
    return false;
  }

  try {
    const parsed = JSON.parse(jsonStr);
    return !!(parsed.name && parsed.api_base_url && parsed.api_key);
  } catch {
    return false;
  }
}