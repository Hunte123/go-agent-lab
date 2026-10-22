package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Hunte123/go-agent-lab/internal/llm"
)

// Agent 是内核的最小完备实现：SystemPrompt + 工具集 + 步数上限。
type Agent struct {
	Name         string
	SystemPrompt string
	Tools        *Registry
	MaxSteps     int // 防失控，默认 12
	LLM          LLMClient
}

// LLMClient 是内核依赖的最小接口——测试时可注入 Mock。
type LLMClient interface {
	Complete(ctx context.Context, msgs []llm.Message, tools []llm.ToolSpec) (llm.Message, llm.Usage, error)
}

// RunResult 记录一次运行的完整轨迹计量。
type RunResult struct {
	Answer    string        // 最终答复文本
	Steps     int           // 思考轮数
	ToolCalls int           // 工具调用次数
	TokensIn  int           // 累计输入 token
	TokensOut int           // 累计输出 token
	Duration  time.Duration // 总耗时
}

// Run 执行核心循环：思考 → 调用工具 → 观察 → …… → 最终答案。
func (a *Agent) Run(ctx context.Context, task string) (*RunResult, error) {
	if a.MaxSteps <= 0 {
		a.MaxSteps = 12
	}
	start := time.Now()
	res := &RunResult{}

	msgs := []llm.Message{{Role: "system", Content: a.SystemPrompt}}
	msgs = append(msgs, llm.Message{Role: "user", Content: task})

	for step := 1; step <= a.MaxSteps; step++ {
		res.Steps = step
		reply, usage, err := a.LLM.Complete(ctx, msgs, a.Tools.Specs())
		if err != nil {
			return res, fmt.Errorf("第%d轮 LLM 调用失败: %w", step, err)
		}
		res.TokensIn += usage.PromptTokens
		res.TokensOut += usage.CompletionTokens

		// 无工具调用：视作最终答案
		if len(reply.ToolCalls) == 0 {
			res.Answer = reply.Content
			res.Duration = time.Since(start)
			return res, nil
		}

		// 有工具调用：执行并回填观察结果
		msgs = append(msgs, reply)
		for _, call := range reply.ToolCalls {
			res.ToolCalls++
			out := a.Tools.Execute(call.Function.Name, json.RawMessage(call.Function.Arguments))
			msgs = append(msgs, llm.Message{
				Role:       "tool",
				ToolCallID: call.ID,
				Content:    out,
			})
		}
	}
	res.Duration = time.Since(start)
	return res, fmt.Errorf("达到最大步数 %d 仍未给出最终答案", a.MaxSteps)
}

// CostSummary 输出人话版计量。
func (r *RunResult) CostSummary() string {
	return fmt.Sprintf("steps=%d toolCalls=%d tokens(in/out)=%d/%d 耗时=%.1fs",
		r.Steps, r.ToolCalls, r.TokensIn, r.TokensOut, r.Duration.Seconds())
}
