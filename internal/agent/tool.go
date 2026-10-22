// Package agent 实现一个极简但完整的 Agent 内核：
// LLM 思考 → 调用工具 → 观察结果 → 继续思考，直到给出最终答案。
// 不依赖任何 agent 框架，核心循环不足百行，适合学习与二次开发。
package agent

import (
	"encoding/json"
	"fmt"

	"github.com/Hunte123/go-agent-lab/internal/llm"
)

// Tool 是 Agent 可调用的工具：一个带 JSON Schema 声明的 Go 函数。
type Tool struct {
	Name        string
	Description string
	// Parameters 是 JSON Schema 的 properties 片段，形如：
	//   {"code": {"type": "string", "description": "股票代码"}}
	Parameters map[string]any
	Fn         func(args json.RawMessage) (string, error)
}

// Spec 转换为下发给模型的声明。
func (t Tool) Spec() llm.ToolSpec {
	props := t.Parameters
	if props == nil {
		props = map[string]any{}
	}
	return llm.ToolSpec{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        t.Name,
			Description: t.Description,
			Parameters: map[string]any{
				"type":       "object",
				"properties": props,
			},
		},
	}
}

// Registry 管理一组工具。
type Registry struct {
	tools map[string]Tool
	order []string
}

func NewRegistry(tools ...Tool) *Registry {
	r := &Registry{tools: map[string]Tool{}}
	for _, t := range tools {
		r.Add(t)
	}
	return r
}

func (r *Registry) Add(t Tool) {
	if _, ok := r.tools[t.Name]; !ok {
		r.order = append(r.order, t.Name)
	}
	r.tools[t.Name] = t
}

func (r *Registry) Specs() []llm.ToolSpec {
	specs := make([]llm.ToolSpec, 0, len(r.order))
	for _, name := range r.order {
		specs = append(specs, r.tools[name].Spec())
	}
	return specs
}

// Execute 调用工具并把结果转为模型可读的文本。
func (r *Registry) Execute(name string, args json.RawMessage) string {
	tool, ok := r.tools[name]
	if !ok {
		return fmt.Sprintf("错误：未知工具 %q", name)
	}
	out, err := tool.Fn(args)
	if err != nil {
		return fmt.Sprintf("工具 %s 执行失败: %v", name, err)
	}
	return out
}
