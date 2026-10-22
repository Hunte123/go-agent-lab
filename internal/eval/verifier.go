package eval

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// Check 在 Agent 运行结束后验收终态。workdir 是本次运行的工作目录，
// answer 是 Agent 的最终答复文本。
func (v Verifier) Check(workdir, answer string) error {
	switch v.Type {
	case "file_exists":
		p := filepath.Join(workdir, v.Path)
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("文件不存在: %s", v.Path)
		}
		return nil

	case "file_contains":
		data, err := os.ReadFile(filepath.Join(workdir, v.Path))
		if err != nil {
			return fmt.Errorf("无法读取 %s: %w", v.Path, err)
		}
		re, err := regexp.Compile(v.Regex)
		if err != nil {
			return fmt.Errorf("验收正则非法: %w", err)
		}
		if !re.Match(data) {
			return fmt.Errorf("%s 内容不匹配 %s", v.Path, v.Regex)
		}
		return nil

	case "answer_contains":
		re, err := regexp.Compile(v.Regex)
		if err != nil {
			return fmt.Errorf("验收正则非法: %w", err)
		}
		if !re.MatchString(answer) {
			return fmt.Errorf("答复不包含 %s", v.Regex)
		}
		return nil

	default:
		return fmt.Errorf("未知验收器类型: %s", v.Type)
	}
}

// Label 返回人类可读的规则描述。
func (v Verifier) Label() string {
	if v.Desc != "" {
		return v.Desc
	}
	return fmt.Sprintf("%s %s %s", v.Type, v.Path, v.Regex)
}
