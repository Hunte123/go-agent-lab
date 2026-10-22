// Package study 用三个协作的 Agent 完成"输入代码 → 输出研报"的完整流水线：
//
//	数据员（行情工具）→ 分析师（纯推理）→ 撰稿人（写文件）
//
// 展示 Agent 间显式交接：上一棒的 Answer 作为下一棒的任务输入。
package study

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Hunte123/go-agent-lab/internal/agent"
	"github.com/Hunte123/go-agent-lab/internal/tools"
)

type Study struct {
	LLM     agent.LLMClient
	Workdir string // 报告输出目录
}

// Result 汇总三个 Agent 的运行计量。
type Result struct {
	Data     *agent.RunResult
	Analysis *agent.RunResult
	Report   *agent.RunResult
}

// Run 执行完整流水线，返回报告路径与计量。
func (s *Study) Run(ctx context.Context, codes []string) (*Result, string, error) {
	if len(codes) == 0 {
		return nil, "", fmt.Errorf("未提供股票代码")
	}

	// ── Agent 1 数据员：只负责拿数 ──
	dataAgent := &agent.Agent{
		Name: "data-collector",
		SystemPrompt: "你是行情数据采集员。用 get_quote 工具逐只查询用户给出的股票，" +
			"最后用简洁的列表汇总：名称、代码、最新价、涨跌幅、成交额。只陈述数据，不做任何分析。",
		Tools:    agent.NewRegistry(tools.GetQuote()),
		MaxSteps: 15,
		LLM:      s.LLM,
	}
	data, err := dataAgent.Run(ctx, "查询以下股票的最新行情并汇总: "+strings.Join(codes, "、"))
	if err != nil {
		return &Result{Data: data}, "", fmt.Errorf("数据员失败: %w", err)
	}

	// ── Agent 2 分析师：只负责推理 ──
	analyst := &agent.Agent{
		Name: "analyst",
		SystemPrompt: "你是严谨的市场分析师，基于给定的行情数据做技术面分析：" +
			"涨跌结构、量能、相对强弱。禁止编造数据里没有的信息。" +
			"结尾必须加一句：以上为程序化分析，不构成投资建议。",
		Tools:    agent.NewRegistry(), // 纯推理，无工具
		MaxSteps: 6,
		LLM:      s.LLM,
	}
	analysis, err := analyst.Run(ctx, "请分析以下今日行情数据：\n\n"+data.Answer)
	if err != nil {
		return &Result{Data: data, Analysis: analysis}, "", fmt.Errorf("分析师失败: %w", err)
	}

	// ── Agent 3 撰稿人：只负责成稿，落盘由编排层完成 ──
	// （设计教训：长文本经由工具调用的 JSON 参数传递易碎，
	// 模型转义出错→写入失败→重试循环；改为 Agent 纯输出、代码落盘。）
	reporter := &agent.Agent{
		Name: "reporter",
		SystemPrompt: "你是金融撰稿人。把给定的分析内容整理成结构清晰的 Markdown 研报，" +
			"一级标题用「# 每日行情研报」，包含数据表、分析、免责声明三部分。" +
			"直接把完整研报全文作为最终答复输出，不要调用任何工具。",
		Tools:    agent.NewRegistry(),
		MaxSteps: 6,
		LLM:      s.LLM,
	}
	report, err := reporter.Run(ctx, "请成稿：\n\n"+analysis.Answer)

	res := &Result{Data: data, Analysis: analysis, Report: report}
	if err != nil {
		return res, "", fmt.Errorf("撰稿人失败: %w", err)
	}
	// 编排层落盘：Agent 思考，环境 I/O 归宿主代码
	path := filepath.Join(s.Workdir, "report.md")
	if err := os.WriteFile(path, []byte(report.Answer), 0o644); err != nil {
		return res, "", fmt.Errorf("写入报告失败: %w", err)
	}
	return res, path, nil
}

// Summary 打印三段计量。
func (r *Result) Summary() string {
	return fmt.Sprintf("数据员[%s] → 分析师[%s] → 撰稿人[%s]",
		r.Data.CostSummary(), r.Analysis.CostSummary(), r.Report.CostSummary())
}
