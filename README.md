# go-agent-lab

一个自包含的 Go Agent 实验项目，三件套合一：

1. **Agent 内核**（`internal/agent`）——不依赖任何框架，核心循环"思考 → 调用工具 → 观察 → 再思考"约百行实现
2. **多 Agent 协作**（`internal/study`）——数据员 → 分析师 → 撰稿人三段流水线，输入股票代码、输出 Markdown 研报
3. **评测框架**（`internal/eval`）——JSON 定义任务、终态验收器、多轮运行统计，量化任何 OpenAI 兼容模型驱动的 Agent

## 快速开始

```bash
go build -o agentlab ./cmd/agentlab

export AGENTLAB_BASE_URL=https://api.openai.com/v1   # 任意 OpenAI 兼容服务
export AGENTLAB_API_KEY=sk-xxx
export AGENTLAB_MODEL=gpt-4o-mini                     # GLM/DeepSeek/Kimi/Ollama 均可

./agentlab study --codes 600519,002737                # 多 Agent 研报流水线
./agentlab eval --tasks examples/tasks --runs 3       # 评测跑批
```

Ollama 本地模型同样支持：`AGENTLAB_BASE_URL=http://localhost:11434/v1 AGENTLAB_MODEL=qwen3:8b`。

## 设计

### Agent 内核

```
task → [ LLM 思考 ] →需要工具？→ 执行工具 → 结果回填 → 继续思考 …
                      ↓ 不需要
                    最终答案 + 计量（步数/工具调用/token/耗时）
```

- 工具 = 带 JSON Schema 声明的普通 Go 函数（`agent.Tool`）
- LLM 通过接口注入（`agent.LLMClient`），测试用 Mock，零 API 成本
- 步数上限防失控，每步 token 计量

### 多 Agent 编排

三个角色显式交接（上一棒的 Answer 即下一棒的任务输入）：

| 角色 | 工具 | 职责 |
|------|------|------|
| 数据员 | get_quote | 行情采集，只陈述数据 |
| 分析师 | 无 | 纯推理，禁止编造数据 |
| 撰稿人 | write_file | 成稿落盘 report.md |

行情工具为新浪（主）+ 腾讯（备）双数据源容灾。

### 评测框架

原则：**只验收终态，不评判过程；验收器在 Agent 够不到的层面执行。**

- 任务用 JSON 定义：`prompt` + 多条 `verifiers`
- 验收器类型：`file_exists` / `file_contains`（正则）/ `answer_contains`
- 每题 N 次独立运行（独立工作目录），输出通过率、平均 token、平均耗时

```json
{
  "name": "quote_fetch",
  "prompt": "查询贵州茅台(600519)的最新行情，把结果写入 price.txt……",
  "verifiers": [
    {"type": "file_exists", "path": "price.txt"},
    {"type": "file_contains", "path": "price.txt", "regex": "茅台"}
  ]
}
```

### 安全

- 文件工具被钉死在每次运行的工作目录内，路径穿越（`../`）直接拒绝
- 单测覆盖核心循环与沙箱约束：`go test ./...`

## 目录

```
cmd/agentlab/        CLI 入口（study / eval）
internal/agent/      Agent 内核与工具系统
internal/llm/        OpenAI 兼容客户端
internal/tools/      行情工具 / 沙箱文件工具
internal/study/      多 Agent 编排
internal/eval/       评测框架
examples/tasks/      示例评测任务
```

## 免责声明

行情数据来自公开免费接口，研究输出由模型生成，仅供学习研究，不构成任何投资建议。

## License

MIT
