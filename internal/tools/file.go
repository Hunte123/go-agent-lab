package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Hunte123/go-agent-lab/internal/agent"
)

// WriteFile 注册受限的文件写入工具：只允许写 workdir 内的相对路径。
func WriteFile(workdir string) agent.Tool {
	return agent.Tool{
		Name:        "write_file",
		Description: "把文本内容写入工作目录内的文件（相对路径）。用于产出报告、代码、数据文件。",
		Parameters: map[string]any{
			"path":    map[string]any{"type": "string", "description": "相对路径，如 result/price.txt"},
			"content": map[string]any{"type": "string", "description": "文件全文内容"},
		},
		Fn: func(args json.RawMessage) (string, error) {
			var in struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return "", err
			}
			abs, err := safeJoin(workdir, in.Path)
			if err != nil {
				return "", err
			}
			if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
				return "", err
			}
			if err := os.WriteFile(abs, []byte(in.Content), 0o644); err != nil {
				return "", err
			}
			return fmt.Sprintf("已写入 %s（%d 字节）", in.Path, len(in.Content)), nil
		},
	}
}

// ReadFile 注册受限的文件读取工具。
func ReadFile(workdir string) agent.Tool {
	return agent.Tool{
		Name:        "read_file",
		Description: "读取工作目录内文件的内容（相对路径）。",
		Parameters: map[string]any{
			"path": map[string]any{"type": "string", "description": "相对路径"},
		},
		Fn: func(args json.RawMessage) (string, error) {
			var in struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return "", err
			}
			abs, err := safeJoin(workdir, in.Path)
			if err != nil {
				return "", err
			}
			data, err := os.ReadFile(abs)
			if err != nil {
				return "", err
			}
			return string(data), nil
		},
	}
}

// safeJoin 把相对路径钉死在 workdir 内，阻止 Agent 越界读写。
func safeJoin(workdir, rel string) (string, error) {
	if strings.Contains(rel, "..") {
		return "", fmt.Errorf("路径越界: %s", rel)
	}
	abs := filepath.Join(workdir, filepath.Clean("/"+rel))
	root, err := filepath.Abs(workdir)
	if err != nil {
		return "", err
	}
	if abs != root && !strings.HasPrefix(abs, root+string(filepath.Separator)) {
		return "", fmt.Errorf("路径越界: %s", rel)
	}
	return abs, nil
}
