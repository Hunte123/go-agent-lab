package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Hunte123/go-agent-lab/internal/llm"
)

// mockLLM 按脚本回放：第一轮发起工具调用，第二轮给出最终答案。
type mockLLM struct{ calls int }

func (m *mockLLM) Complete(_ context.Context, _ []llm.Message, _ []llm.ToolSpec) (llm.Message, llm.Usage, error) {
	m.calls++
	if m.calls == 1 {
		msg := llm.Message{Role: "assistant"}
		call := llm.ToolCall{ID: "c1", Type: "function"}
		call.Function.Name = "echo"
		call.Function.Arguments = `{"text":"hello"}`
		msg.ToolCalls = []llm.ToolCall{call}
		return msg, llm.Usage{PromptTokens: 10, CompletionTokens: 5}, nil
	}
	return llm.Message{Role: "assistant", Content: "最终答案：echo 返回了数据"},
		llm.Usage{PromptTokens: 20, CompletionTokens: 8}, nil
}

// TestAgentLoopToolCallAndFinish 验证核心循环：
// 思考→工具调用→观察→最终答案，以及 token/步数计量。
func TestAgentLoopToolCallAndFinish(t *testing.T) {
	reg := NewRegistry(Tool{
		Name:        "echo",
		Description: "原样返回文本",
		Parameters:  map[string]any{"text": map[string]any{"type": "string"}},
		Fn: func(args json.RawMessage) (string, error) {
			var in struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return "", err
			}
			return "echo:" + in.Text, nil
		},
	})

	a := &Agent{Name: "t", SystemPrompt: "test", Tools: reg, LLM: &mockLLM{}}
	res, err := a.Run(context.Background(), "做点什么")
	if err != nil {
		t.Fatalf("Run 失败: %v", err)
	}
	if res.Steps != 2 {
		t.Errorf("期望 2 步, 实际 %d", res.Steps)
	}
	if res.ToolCalls != 1 {
		t.Errorf("期望 1 次工具调用, 实际 %d", res.ToolCalls)
	}
	if res.TokensIn != 30 || res.TokensOut != 13 {
		t.Errorf("token 计量不对: in=%d out=%d", res.TokensIn, res.TokensOut)
	}
	if !strings.Contains(res.Answer, "最终答案") {
		t.Errorf("答复不对: %s", res.Answer)
	}
}

// TestRegistryUnknownTool 验证未知工具的兜底行为。
func TestRegistryUnknownTool(t *testing.T) {
	out := NewRegistry().Execute("nope", json.RawMessage(`{}`))
	if !strings.Contains(out, "未知工具") {
		t.Errorf("未知工具应返回错误提示: %s", out)
	}
}
