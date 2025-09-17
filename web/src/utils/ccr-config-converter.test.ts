/**
 * CCR 配置转换功能测试
 * 用于验证不同渠道间的配置转换是否正确
 */

import { convertCCRConfigToChannel, extractModelsFromConfig } from "./ccr-config-converter";

// 测试数据 - 基于您提供的真实模板
const openaiConfig = `{
  "name": "kyxcode",
  "api_base_url": "http://localhost:3001/proxy/kyxcode/v1/chat/completions",
  "api_key": "1968121800",
  "models": [
    "claude-sonnet-4-20250514"
  ],
  "transformer": {
    "use": [
      [
        "maxtoken",
        {
          "max_tokens": 65535
        }
      ]
    ],
    "claude-sonnet-4-20250514": {
      "use": [
        "reasoning"
      ]
    }
  }
}`;

const anthropicConfig = `{
  "name": "kyxcode",
  "api_base_url": "http://localhost:3001/proxy/kyxcode/v1/messages?beta=true",
  "api_key": "1968121800",
  "models": [
    "claude-sonnet-4-20250514"
  ],
  "transformer": {
    "use": [
      "Anthropic"
    ]
  }
}`;

const geminiConfig = `{
  "name": "kyxcode",
  "api_base_url": "http://localhost:3001/proxy/kyxcode/v1beta/models/",
  "api_key": "1968121800",
  "models": [
    "gemini-2.5-pro",
    "gemini-2.5-flash"
  ],
  "transformer": {
    "use": [
      "gemini"
    ]
  }
}`;

// 在控制台中运行这些测试
console.log("=== CCR 配置转换测试（修正版）===");

console.log("\n1. 测试 OpenAI -> Anthropic 转换：");
const openaiToAnthropic = convertCCRConfigToChannel(
  openaiConfig,
  "anthropic",
  "kyxcode",
  "1968121800"
);
const anthropicResult = JSON.parse(openaiToAnthropic);
console.log("转换结果：", openaiToAnthropic);
console.log(
  "Transformer 正确性检查：",
  JSON.stringify(anthropicResult.transformer) === '{"use":["Anthropic"]}' ? "✅ 正确" : "❌ 错误"
);

console.log("\n2. 测试 Anthropic -> Gemini 转换：");
const anthropicToGemini = convertCCRConfigToChannel(
  anthropicConfig,
  "gemini",
  "kyxcode",
  "1968121800"
);
const geminiResult = JSON.parse(anthropicToGemini);
console.log("转换结果：", anthropicToGemini);
console.log(
  "Transformer 正确性检查：",
  JSON.stringify(geminiResult.transformer) === '{"use":["gemini"]}' ? "✅ 正确" : "❌ 错误"
);

console.log("\n3. 测试 Gemini -> OpenAI 转换：");
const geminiToOpenai = convertCCRConfigToChannel(geminiConfig, "openai", "kyxcode", "1968121800");
const openaiResult = JSON.parse(geminiToOpenai);
console.log("转换结果：", geminiToOpenai);
console.log("Transformer 结构检查：");
console.log("- 包含 maxtoken：", openaiResult.transformer.use ? "✅ 正确" : "❌ 错误");
console.log(
  "- 模型级 reasoning：",
  openaiResult.transformer["gemini-2.5-pro"] && openaiResult.transformer["gemini-2.5-flash"]
    ? "✅ 正确"
    : "❌ 错误"
);

console.log("\n4. 验证模型列表保持一致性：");
const originalModels = extractModelsFromConfig(openaiConfig);
const convertedModels = extractModelsFromConfig(openaiToAnthropic);
console.log("原始模型:", originalModels);
console.log("转换后模型:", convertedModels);
console.log(
  "模型保持一致:",
  JSON.stringify(originalModels) === JSON.stringify(convertedModels) ? "✅ 正确" : "❌ 错误"
);

console.log("\n5. 测试复杂模型列表转换：");
const complexOpenaiConfig = `{
  "name": "test",
  "api_base_url": "http://localhost:3001/proxy/test/v1/chat/completions",
  "api_key": "test",
  "models": ["claude-sonnet-4-20250514", "gpt-4", "claude-3-5-sonnet"],
  "transformer": {
    "use": [["maxtoken", {"max_tokens": 65535}]],
    "claude-sonnet-4-20250514": {"use": ["reasoning"]},
    "gpt-4": {"use": ["reasoning"]},
    "claude-3-5-sonnet": {"use": ["reasoning"]}
  }
}`;

const complexToAnthropic = convertCCRConfigToChannel(
  complexOpenaiConfig,
  "anthropic",
  "test",
  "test"
);
const complexAnthropicResult = JSON.parse(complexToAnthropic);
console.log("复杂模型列表:", complexAnthropicResult.models);
console.log("Anthropic transformer:", complexAnthropicResult.transformer);

console.log("\n=== 测试完成 ===");
