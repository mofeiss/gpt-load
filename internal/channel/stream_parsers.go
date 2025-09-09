package channel

import (
	"bufio"
	"encoding/json"
	"fmt"
	"gpt-load/internal/models"
	"io"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
)

// StreamParser 流式响应解析器接口
type StreamParser interface {
	ParseStream(reader io.Reader) (*models.StreamContent, error)
}

// OpenAIStreamParser OpenAI 流式解析器
type OpenAIStreamParser struct{}

// OpenAI 流式响应结构
type openaiStreamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index  int `json:"index"`
		Delta  struct {
			Role             *string                `json:"role,omitempty"`
			Content          *string                `json:"content,omitempty"`
			ReasoningContent *string                `json:"reasoning_content,omitempty"`
			ToolCalls        []openaiToolCall       `json:"tool_calls,omitempty"`
			FunctionCall     *openaiFunctionCall    `json:"function_call,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

type openaiToolCall struct {
	Index    *int                `json:"index,omitempty"`
	ID       *string             `json:"id,omitempty"`
	Type     *string             `json:"type,omitempty"`
	Function *openaiToolFunction `json:"function,omitempty"`
}

type openaiToolFunction struct {
	Name      *string `json:"name,omitempty"`
	Arguments *string `json:"arguments,omitempty"`
}

type openaiFunctionCall struct {
	Name      *string `json:"name,omitempty"`
	Arguments *string `json:"arguments,omitempty"`
}

// ParseStream 解析 OpenAI 流式响应
func (p *OpenAIStreamParser) ParseStream(reader io.Reader) (*models.StreamContent, error) {
	var thinkingChain strings.Builder
	var textMessages strings.Builder
	var toolCalls strings.Builder
	var rawContent strings.Builder

	// 添加数据大小限制，防止内存溢出
	const maxDataSize = 10 * 1024 * 1024 // 10MB 限制
	limitedReader := io.LimitReader(reader, maxDataSize)
	
	scanner := bufio.NewScanner(limitedReader)
	// 设置扫描器的缓冲区大小限制
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max token size
	for scanner.Scan() {
		line := scanner.Text()
		rawContent.WriteString(line + "\n")

		// 跳过空行和非数据行
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		// 移除 "data: " 前缀
		jsonData := strings.TrimPrefix(line, "data: ")
		
		// 跳过 [DONE] 标记
		if strings.TrimSpace(jsonData) == "[DONE]" {
			continue
		}

		var chunk openaiStreamChunk
		if err := json.Unmarshal([]byte(jsonData), &chunk); err != nil {
			logrus.WithError(err).Warn("Failed to unmarshal OpenAI stream chunk")
			continue
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]

		// 解析思维链
		if choice.Delta.ReasoningContent != nil && *choice.Delta.ReasoningContent != "" {
			thinkingChain.WriteString(*choice.Delta.ReasoningContent)
		}

		// 解析文本消息
		if choice.Delta.Content != nil && *choice.Delta.Content != "" {
			textMessages.WriteString(*choice.Delta.Content)
		}

		// 解析工具调用
		if len(choice.Delta.ToolCalls) > 0 {
			if toolCallBytes, err := json.Marshal(choice.Delta.ToolCalls); err == nil {
				toolCalls.WriteString(string(toolCallBytes))
			}
		}

		// 解析函数调用（向后兼容）
		if choice.Delta.FunctionCall != nil {
			if funcCallBytes, err := json.Marshal(choice.Delta.FunctionCall); err == nil {
				toolCalls.WriteString(string(funcCallBytes))
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading stream: %w", err)
	}

	return &models.StreamContent{
		ThinkingChain: strings.TrimSpace(thinkingChain.String()),
		TextMessages:  strings.TrimSpace(textMessages.String()),
		ToolCalls:     strings.TrimSpace(toolCalls.String()),
		RawContent:    rawContent.String(),
	}, nil
}

// AnthropicStreamParser Anthropic 流式解析器
type AnthropicStreamParser struct{}

// Anthropic 流式事件结构
type anthropicStreamEvent struct {
	Type  string          `json:"type"`
	Index *int            `json:"index,omitempty"`
	Delta *anthropicDelta `json:"delta,omitempty"`
}

type anthropicDelta struct {
	Type           string  `json:"type"`
	Text           *string `json:"text,omitempty"`
	PartialJSON    *string `json:"partial_json,omitempty"`
	InputJSONDelta *string `json:"input_json_delta,omitempty"`
}

// ParseStream 解析 Anthropic 流式响应
func (p *AnthropicStreamParser) ParseStream(reader io.Reader) (*models.StreamContent, error) {
	var thinkingChain strings.Builder
	var textMessages strings.Builder
	var toolCalls strings.Builder
	var rawContent strings.Builder

	// 添加数据大小限制，防止内存溢出
	const maxDataSize = 10 * 1024 * 1024 // 10MB 限制
	limitedReader := io.LimitReader(reader, maxDataSize)
	
	scanner := bufio.NewScanner(limitedReader)
	// 设置扫描器的缓冲区大小限制
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max token size

	for scanner.Scan() {
		line := scanner.Text()
		rawContent.WriteString(line + "\n")

		if strings.HasPrefix(line, "event: ") {
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			jsonData := strings.TrimPrefix(line, "data: ")
			
			var event anthropicStreamEvent
			if err := json.Unmarshal([]byte(jsonData), &event); err != nil {
				logrus.WithError(err).Warn("Failed to unmarshal Anthropic stream event")
				continue
			}

			switch event.Type {
			case "content_block_delta":
				if event.Delta == nil {
					continue
				}

				// 解析思维链（Anthropic 可能支持）
				if event.Delta.Type == "thinking_delta" && event.Delta.Text != nil {
					thinkingChain.WriteString(*event.Delta.Text)
				}

				// 解析文本消息
				if event.Delta.Type == "text_delta" && event.Delta.Text != nil {
					textMessages.WriteString(*event.Delta.Text)
				}

				// 解析工具调用
				if event.Delta.Type == "input_json_delta" {
					if event.Delta.PartialJSON != nil && *event.Delta.PartialJSON != "" {
						toolCalls.WriteString(*event.Delta.PartialJSON)
					}
					if event.Delta.InputJSONDelta != nil && *event.Delta.InputJSONDelta != "" {
						toolCalls.WriteString(*event.Delta.InputJSONDelta)
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading stream: %w", err)
	}

	return &models.StreamContent{
		ThinkingChain: strings.TrimSpace(thinkingChain.String()),
		TextMessages:  strings.TrimSpace(textMessages.String()),
		ToolCalls:     strings.TrimSpace(toolCalls.String()),
		RawContent:    rawContent.String(),
	}, nil
}

// GeminiStreamParser Gemini 流式解析器
type GeminiStreamParser struct{}

// Gemini 流式响应结构
type geminiStreamChunk struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

type geminiPart struct {
	Text          *string                  `json:"text,omitempty"`
	FunctionCalls []geminiFunctionCall     `json:"functionCalls,omitempty"`
	FunctionCall  *geminiFunctionCall      `json:"functionCall,omitempty"`
}

type geminiFunctionCall struct {
	Name *string                `json:"name,omitempty"`
	Args map[string]interface{} `json:"args,omitempty"`
}

// ParseStream 解析 Gemini 流式响应
func (p *GeminiStreamParser) ParseStream(reader io.Reader) (*models.StreamContent, error) {
	var textMessages strings.Builder
	var toolCalls strings.Builder
	var rawContent strings.Builder

	// 添加数据大小限制，防止内存溢出
	const maxDataSize = 10 * 1024 * 1024 // 10MB 限制
	limitedReader := io.LimitReader(reader, maxDataSize)
	
	// Gemini 流式响应可能是多个 JSON 对象，使用正则表达式分割
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("error reading stream: %w", err)
	}

	// 检查数据是否为空
	if len(data) == 0 {
		logrus.Warn("Gemini stream data is empty")
		return &models.StreamContent{
			ThinkingChain: "",
			TextMessages:  "",
			ToolCalls:     "",
			RawContent:    "",
		}, nil
	}

	rawContent.WriteString(string(data))

	// 移除 SSE 前缀并提取 JSON
	content := string(data)
	
	// 处理 SSE 格式
	if strings.Contains(content, "data: ") {
		lines := strings.Split(content, "\n")
		var jsonLines []string
		for _, line := range lines {
			if strings.HasPrefix(line, "data: ") {
				jsonData := strings.TrimPrefix(line, "data: ")
				if strings.TrimSpace(jsonData) != "[DONE]" && strings.TrimSpace(jsonData) != "" {
					jsonLines = append(jsonLines, jsonData)
				}
			}
		}
		content = strings.Join(jsonLines, "\n")
	}

	// 用正则表达式分割多个 JSON 对象，添加recover防止panic
	var jsonObjects []string
	func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("Gemini正则表达式处理发生panic: %v", r)
				// 如果正则处理失败，尝试简单的JSON分割
				jsonObjects = []string{content}
			}
		}()
		
		jsonObjectRegex := regexp.MustCompile(`\{[^{}]*(?:\{[^{}]*\}[^{}]*)*\}`)
		jsonObjects = jsonObjectRegex.FindAllString(content, -1)
	}()
	
	// 如果正则表达式没有找到任何对象，尝试整个内容作为JSON
	if len(jsonObjects) == 0 && strings.TrimSpace(content) != "" {
		jsonObjects = []string{content}
	}

	for _, jsonStr := range jsonObjects {
		var chunk geminiStreamChunk
		if err := json.Unmarshal([]byte(jsonStr), &chunk); err != nil {
			logrus.WithError(err).Warn("Failed to unmarshal Gemini stream chunk")
			continue
		}

		for _, candidate := range chunk.Candidates {
			for _, part := range candidate.Content.Parts {
				// 解析文本消息
				if part.Text != nil && *part.Text != "" {
					textMessages.WriteString(*part.Text)
				}

				// 解析工具调用
				if len(part.FunctionCalls) > 0 {
					if funcCallBytes, err := json.Marshal(part.FunctionCalls); err == nil {
						toolCalls.WriteString(string(funcCallBytes))
					}
				}

				// 解析单个工具调用
				if part.FunctionCall != nil {
					if funcCallBytes, err := json.Marshal(part.FunctionCall); err == nil {
						toolCalls.WriteString(string(funcCallBytes))
					}
				}
			}
		}
	}

	return &models.StreamContent{
		ThinkingChain: "", // Gemini 暂不支持思维链
		TextMessages:  strings.TrimSpace(textMessages.String()),
		ToolCalls:     strings.TrimSpace(toolCalls.String()),
		RawContent:    rawContent.String(),
	}, nil
}

// GetStreamParser 根据渠道类型获取流式解析器
func GetStreamParser(channelType string) StreamParser {
	switch strings.ToLower(channelType) {
	case "openai":
		return &OpenAIStreamParser{}
	case "anthropic":
		return &AnthropicStreamParser{}
	case "gemini":
		return &GeminiStreamParser{}
	default:
		// 默认使用 OpenAI 解析器
		return &OpenAIStreamParser{}
	}
}

// FormatStreamContentAsMarkdown 将解析的流式内容格式化为 Markdown 纯文本
func FormatStreamContentAsMarkdown(content *models.StreamContent) string {
	if content == nil {
		return ""
	}

	var result strings.Builder

	// 添加思维链
	if content.ThinkingChain != "" {
		result.WriteString("**思维链**\n")
		result.WriteString("```\n")
		result.WriteString(content.ThinkingChain)
		result.WriteString("\n```\n")
		result.WriteString("\n")
	}

	// 添加文本消息
	if content.TextMessages != "" {
		result.WriteString("**文本消息**\n")
		result.WriteString("```\n")
		result.WriteString(content.TextMessages)
		result.WriteString("\n```\n")
		result.WriteString("\n")
	}

	// 添加工具调用
	if content.ToolCalls != "" {
		result.WriteString("**工具调用**\n")
		result.WriteString("```\n")
		result.WriteString(content.ToolCalls)
		result.WriteString("\n```\n")
	}

	return strings.TrimSpace(result.String())
}