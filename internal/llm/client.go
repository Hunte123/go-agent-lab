// Package llm 提供一个极简的 OpenAI 兼容 Chat Completions 客户端。
// 任何实现该协议的服务（OpenAI / GLM / DeepSeek / Kimi / Ollama / vLLM…）
// 都可以通过环境变量切换接入，本包不做任何厂商绑定。
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Message 是对话中的一条消息；ToolCalls 在助手消息上非空。
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"` // role=tool 时回填
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // 固定 "function"
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // JSON 字符串
	} `json:"function"`
}

// ToolSpec 是下发给模型的工具声明（function calling）。
type ToolSpec struct {
	Type     string       `json:"type"` // 固定 "function"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"` // JSON Schema 片段
}

// Usage 记录一次请求的 token 消耗，用于成本计量。
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type Client struct {
	BaseURL string // 如 https://api.openai.com/v1
	APIKey  string
	Model   string
	HTTP    *http.Client
}

// FromEnv 按环境变量构建客户端：
//
//	AGENTLAB_BASE_URL（默认 https://api.openai.com/v1）
//	AGENTLAB_API_KEY（必填）
//	AGENTLAB_MODEL   （默认 gpt-4o-mini）
func FromEnv() (*Client, error) {
	key := os.Getenv("AGENTLAB_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("环境变量 AGENTLAB_API_KEY 未设置")
	}
	base := os.Getenv("AGENTLAB_BASE_URL")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	base = strings.TrimRight(base, "/")
	model := os.Getenv("AGENTLAB_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &Client{BaseURL: base, APIKey: key, Model: model, HTTP: &http.Client{Timeout: 120 * time.Second}}, nil
}

type chatRequest struct {
	Model    string     `json:"model"`
	Messages []Message  `json:"messages"`
	Tools    []ToolSpec `json:"tools,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage Usage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// Complete 发送一轮对话。tools 为空时不启用 function calling。
// 返回助手消息（可能携带 ToolCalls）与本次 token 用量。
func (c *Client) Complete(ctx context.Context, msgs []Message, tools []ToolSpec) (Message, Usage, error) {
	body, err := json.Marshal(chatRequest{Model: c.Model, Messages: msgs, Tools: tools})
	if err != nil {
		return Message{}, Usage{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Message{}, Usage{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Message{}, Usage{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return Message{}, Usage{}, err
	}
	var out chatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return Message{}, Usage{}, fmt.Errorf("响应解析失败(HTTP %d): %s", resp.StatusCode, truncate(raw, 200))
	}
	if out.Error != nil {
		return Message{}, Usage{}, fmt.Errorf("API错误[%s]: %s", out.Error.Type, out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return Message{}, Usage{}, fmt.Errorf("响应无 choices: %s", truncate(raw, 200))
	}
	return out.Choices[0].Message, out.Usage, nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
