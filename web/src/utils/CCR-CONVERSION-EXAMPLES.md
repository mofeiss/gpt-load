# CCR 配置转换示例

## 转换规则说明

### OpenAI 渠道特征
- API 端点：`/v1/chat/completions`
- Transformer 包含：
  - `maxtoken` 配置（全局）
  - 每个模型的 `reasoning` 配置（模型级）

### Anthropic 渠道特征
- API 端点：`/v1/messages?beta=true`
- Transformer 只包含：`["Anthropic"]` 标识

### Gemini 渠道特征
- API 端点：`/v1beta/models/`
- Transformer 只包含：`["gemini"]` 标识

## 转换示例

### 1. OpenAI → Anthropic

**输入（OpenAI）：**
```json
{
  "name": "kyxcode",
  "api_base_url": "http://localhost:3001/proxy/kyxcode/v1/chat/completions",
  "api_key": "1968121800",
  "models": ["claude-sonnet-4-20250514", "gpt-4"],
  "transformer": {
    "use": [["maxtoken", {"max_tokens": 65535}]],
    "claude-sonnet-4-20250514": {"use": ["reasoning"]},
    "gpt-4": {"use": ["reasoning"]}
  }
}
```

**输出（Anthropic）：**
```json
{
  "name": "kyxcode",
  "api_base_url": "http://localhost:3001/proxy/kyxcode/v1/messages?beta=true",
  "api_key": "1968121800",
  "models": ["claude-sonnet-4-20250514", "gpt-4"],
  "transformer": {
    "use": ["Anthropic"]
  }
}
```

### 2. Anthropic → Gemini

**输入（Anthropic）：**
```json
{
  "name": "kyxcode",
  "api_base_url": "http://localhost:3001/proxy/kyxcode/v1/messages?beta=true",
  "api_key": "1968121800",
  "models": ["claude-sonnet-4-20250514"],
  "transformer": {
    "use": ["Anthropic"]
  }
}
```

**输出（Gemini）：**
```json
{
  "name": "kyxcode",
  "api_base_url": "http://localhost:3001/proxy/kyxcode/v1beta/models/",
  "api_key": "1968121800",
  "models": ["claude-sonnet-4-20250514"],
  "transformer": {
    "use": ["gemini"]
  }
}
```

### 3. Gemini → OpenAI

**输入（Gemini）：**
```json
{
  "name": "kyxcode",
  "api_base_url": "http://localhost:3001/proxy/kyxcode/v1beta/models/",
  "api_key": "1968121800",
  "models": ["gemini-2.5-pro", "gemini-2.5-flash"],
  "transformer": {
    "use": ["gemini"]
  }
}
```

**输出（OpenAI）：**
```json
{
  "name": "kyxcode",
  "api_base_url": "http://localhost:3001/proxy/kyxcode/v1/chat/completions",
  "api_key": "1968121800",
  "models": ["gemini-2.5-pro", "gemini-2.5-flash"],
  "transformer": {
    "use": [["maxtoken", {"max_tokens": 65535}]],
    "gemini-2.5-pro": {"use": ["reasoning"]},
    "gemini-2.5-flash": {"use": ["reasoning"]}
  }
}
```

## 关键改进

1. **完美的 Transformer 转换**：
   - OpenAI：复杂的 transformer 结构（maxtoken + 模型级 reasoning）
   - Anthropic：简化的 transformer（只有 Anthropic 标识）
   - Gemini：简化的 transformer（只有 gemini 标识）

2. **保留模型列表**：
   - 所有自定义添加的模型都会被保留
   - 转换时不会丢失任何模型配置

3. **正确的 API 端点**：
   - 自动更新为对应渠道的正确端点格式
   - 保持分组名称的一致性

4. **智能的默认模型**：
   - 如果没有模型，使用对应渠道的默认模型
   - OpenAI/Anthropic：claude-sonnet-4-20250514
   - Gemini：gemini-2.5-pro, gemini-2.5-flash