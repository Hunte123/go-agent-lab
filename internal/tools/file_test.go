package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWriteFileSandbox 验证文件工具的路径约束：
// 相对路径可写，越界路径（../）必须被拦截。
func TestWriteFileSandbox(t *testing.T) {
	dir := t.TempDir()
	wf := WriteFile(dir)

	out, err := wf.Fn(json.RawMessage(`{"path":"../evil.txt","content":"x"}`))
	if err == nil || !strings.Contains(err.Error(), "越界") {
		t.Errorf("越界写入未被拦截: out=%q err=%v", out, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "..", "evil.txt")); err == nil {
		t.Error("越界文件竟然写成功了")
	}

	out, err = wf.Fn(json.RawMessage(`{"path":"sub/ok.txt","content":"hello"}`))
	if err != nil {
		t.Errorf("正常写入失败: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "sub", "ok.txt"))
	if err != nil || string(data) != "hello" {
		t.Errorf("文件内容不对: %v %q", err, data)
	}
}
