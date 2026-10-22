// Package eval 实现一个最小可用的 Agent 评测框架：
//
//	任务(JSON定义) × N次运行 × 终态验收器 → 成功率/成本/耗时统计
//
// 设计原则：只验收最终状态，不评判过程；验收在 Agent 够不到的层面执行；
// 每次运行使用独立工作目录，保证可复现。
package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Task 是一道评测题：给 Agent 的指令 + 对终态的验收规则。
type Task struct {
	Name       string      `json:"name"`
	Prompt     string      `json:"prompt"`      // 任务指令（描述终态目标）
	MaxSteps   int         `json:"max_steps"`   // 0 则用 Agent 默认
	Verifiers  []Verifier  `json:"verifiers"`   // 全部通过才算成功
	WorkSubdir string      `json:"work_subdir"` // 工作目录名，默认取 task 名
}

// Verifier 是一条终态检查规则。
type Verifier struct {
	Type  string `json:"type"` // file_exists | file_contains | answer_contains
	Path  string `json:"path"` // 相对工作目录
	Regex string `json:"regex"`
	Desc  string `json:"desc"`
}

// LoadTasks 从目录加载所有 .json 任务文件。
func LoadTasks(dir string) ([]Task, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("读取任务目录失败: %w", err)
	}
	var tasks []Task
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var t Task
		if err := json.Unmarshal(raw, &t); err != nil {
			return nil, fmt.Errorf("解析 %s 失败: %w", e.Name(), err)
		}
		if t.Name == "" {
			t.Name = strings.TrimSuffix(e.Name(), ".json")
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}
