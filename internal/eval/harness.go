package eval

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Hunte123/go-agent-lab/internal/agent"
	"github.com/Hunte123/go-agent-lab/internal/tools"
)

// Harness 是评测跑批引擎：任务 × N 次运行 → 统计。
type Harness struct {
	LLM        agent.LLMClient
	BaseWorkdir string // 所有运行目录的父目录
}

// RunResult 记录单次运行的验收结果与计量。
type RunResult struct {
	Task    string
	Run     int // 第几次运行（从 1 开始）
	Passed  bool
	Fails   []string // 未通过的验收规则
	Metrics *agent.RunResult
}

// TaskReport 是单任务的聚合统计。
type TaskReport struct {
	Task      string
	Runs      int
	Passes    int           // 成功次数
	PassRate  float64
	AvgTokens int           // 平均 in+out
	AvgSecs   float64       // 平均耗时
	Results   []RunResult
}

// Run 跑一道题 N 次，每次独立工作目录，返回聚合报告。
func (h *Harness) Run(ctx context.Context, task Task, runs int) *TaskReport {
	report := &TaskReport{Task: task.Name, Runs: runs}

	for i := 1; i <= runs; i++ {
		workdir := filepath.Join(h.BaseWorkdir, fmt.Sprintf("%s-run%d", taskDir(task), i))
		_ = os.RemoveAll(workdir)
		_ = os.MkdirAll(workdir, 0o755)

		// 每次运行重建工具集：文件工具钉死在本次工作目录内
		reg := agent.NewRegistry(tools.GetQuote(), tools.WriteFile(workdir), tools.ReadFile(workdir))

		a := &agent.Agent{
			Name:  "subject",
			SystemPrompt: "你是严谨的执行助手。用提供的工具完成任务，" +
				"完成后给出简短总结。产出文件时使用相对路径。",
			Tools:    reg,
			MaxSteps: task.MaxSteps,
			LLM:      h.LLM,
		}

		metrics, err := a.Run(ctx, task.Prompt)
		rr := RunResult{Task: task.Name, Run: i, Metrics: metrics}

		if err != nil {
			rr.Fails = append(rr.Fails, "运行失败: "+err.Error())
		} else {
			for _, v := range task.Verifiers {
				if cerr := v.Check(workdir, metrics.Answer); cerr != nil {
					rr.Fails = append(rr.Fails, v.Label()+": "+cerr.Error())
				}
			}
		}
		rr.Passed = err == nil && len(rr.Fails) == 0
		if rr.Passed {
			report.Passes++
		}
		report.Results = append(report.Results, rr)
	}

	report.PassRate = float64(report.Passes) / float64(runs) * 100
	var tokens int
	var secs float64
	for _, r := range report.Results {
		if r.Metrics != nil {
			tokens += r.Metrics.TokensIn + r.Metrics.TokensOut
			secs += r.Metrics.Duration.Seconds()
		}
	}
	report.AvgTokens = tokens / runs
	report.AvgSecs = secs / float64(runs)
	return report
}

// PrintSummary 打印一份人类可读的汇总表。
func (r *TaskReport) PrintSummary() {
	status := fmt.Sprintf("✅ %d/%d", r.Passes, r.Runs)
	if r.PassRate < 100 {
		status = fmt.Sprintf("❌ %d/%d", r.Passes, r.Runs)
	}
	fmt.Printf("%-24s 通过 %s  平均tokens %d  平均 %.1fs\n", r.Task, status, r.AvgTokens, r.AvgSecs)
	for _, res := range r.Results {
		if !res.Passed {
			for _, f := range res.Fails {
				fmt.Printf("    run%d 失败: %s\n", res.Run, f)
			}
		}
	}
}

func taskDir(t Task) string {
	if t.WorkSubdir != "" {
		return t.WorkSubdir
	}
	return t.Name
}

var _ = time.Second // 保持 time 导入（统计扩展预留）
