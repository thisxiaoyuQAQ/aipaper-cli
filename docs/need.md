# aipaper-cli

全自动 AI 论文创作引擎。Coordinator 在一次 Prompt 里驱动 Architect / Writer / Editor 三个子代理完成整篇论文的创作，Host 只做启动、恢复和观察。


## 特性 待定

- **多智能体协作** — Coordinator 在一次长循环中调度 Architect / Writer / Editor / Humanizer 三个子代理，自主决策创作流程
- **Step 级断点恢复** — 每个工具执行成功后写入 checkpoint，崩溃后精确到 plan/draft/check/commit 步骤级恢复
- **用户实时干预** — 写作过程中随时在输入框注入修改意见（无需暂停），系统自动评估影响范围并重写受影响部分
- **统一 TUI 入口** — 交互界面实时观察进度，也支持携带一句需求直接启动
- **多 LLM 支持** — OpenRouter / Anthropic / Gemini / OpenAI 等等随意切换

## 架构 待定

核心设计：**LLM 驱动，Host 服务**。Coordinator 在一次 Run 中自主决策整本书的创作流程，Host 只做启动、恢复和事件观察。

```
┌─────────────────────────────────────────────────┐
│                Host（薄外壳）                     │
│           启动 / 恢复            │
└──────────────────────┬──────────────────────────┘
                       │ 
┌──────────────────────▼──────────────────────────┐
│                          │
│         │
└────┬──────────┬──────────┬──────────────────────┘

```

- **Host** — 启动 Coordinator、崩溃恢复、事件投影给 TUI。不做任何调度决策
- **Coordinator** — 唯一的决策者，在一次 Run 里驱动规划→写作→评审→总结的完整流程
- **SubAgents** — Architect / Writer / Editor 各自独立 context，通过 Store 中的工件协作
- **Tools** — 原子 IO + checkpoint 写入，只返事实 JSON，不夹带指令

### 智能体职责 待定

| 智能体                | 职责                                 | 工具                                                                                                           |
| --------------------- | ------------------------------------ | -------------------------------------------------------------------------------------------------------------- |             |

### 写作流程 待定

```
用户需求 → Architect 规划骨架 + 首弧部分 → Writer 逐部分写作 → Editor 弧级评审
                                                  ↑                   │
                                                  ├── 重写/打磨 ◄──────┘
                                                  │
                                           Architect 展开下一弧/卷
```


#### 上下文压缩管线

当对话超出模型上下文窗口时，按代价从低到高逐级压缩：

```
     清理旧工具结果    →    截断长文本   →   store 零 LLM 压缩  →    LLM 摘要兜底
```

- **熔断器** — 压缩连续失败时自动跳过并显式告警，采用半开模式，下轮自动重试
- **TUI 健康度渐变** — 上下文占用绿(<70%)→黄(70-85%)→红(>85%)实时展示

## 快速开始 待定

```bash
# 首次运行，自动进入引导流程（选择 Provider → 输入 API Key → Base URL → 模型名）
aipaper-cli
```

进入 TUI 后，启动阶段支持两种前置交互：

- `共创规划`：与 AI 多轮对话澄清需求，**右侧实时同步整理出的创作指令草稿**；AI 每轮主动提供 1-3 条引导建议，按数字键一键填入输入框，按 `Ctrl+S` 进入正式创作
- `材料规划`: 提供科研材料文件夹, **右侧实时同步整理出的创作指令草稿**;AI 每轮主动提供 1-3 条引导建议，按数字键一键填入输入框，按 `Ctrl+S` 进入正式创作

两种模式最终都会收敛为同一份创作指令，再进入同一套创作引擎。

### 管理多篇论文

每本小说绑定到启动目录，产物落在 `{cwd}/output/novel/`。换目录启动 = 换一篇，`cd` 回去启动 = 自动从最近 checkpoint 恢复。配置 `~/.aipaper/config.json` 全局共享，无需复制。

### 配置文件

首次运行时自动引导生成配置文件 `~/.aipaper/config.json`，后续可直接编辑该文件调整设置。删除配置文件后重新运行会再次进入引导流程。

也可以手动创建配置文件，参考 `~/.aipaper/config.example.jsonc`（引导时自动生成）。

```jsonc
{
  "provider": "openrouter",
  "model": "google/gemini-2.5-flash",
  "providers": {
    "openrouter": {
      "api_key": "sk-or-v1-xxx",
      "base_url": "https://openrouter.ai/api/v1",
      "models": ["google/gemini-2.5-flash", "google/gemini-2.5-pro"]
    }
  },
  "style": "default"
}
```

#### 配置文件查找顺序（后者覆盖前者）

1. `~/.aipaper/config.json` — 全局配置
2. `./aipaper.json` — 项目级覆盖（可选）
3. `--config path/to/config.json` — 命令行指定

覆盖规则说明：

- 标量字段按后者覆盖前者，例如 `provider`、`model`、`style`
- `providers` 和 `roles` 按 key 合并，同名项内部按字段覆盖
- 未填写的字段会继承上层配置，例如项目级配置只写 `base_url` 时会保留全局配置中的 `api_key`
- 当前不支持用空字符串显式清空上层已有值；如需清空，请直接编辑更高优先级的配置文件

`providers.<name>.models` 为可选字段，用于声明该 provider 下允许在 TUI `/model` 面板中切换的模型列表；如果未配置，系统会回退为当前配置文件里已经出现过的该 provider 模型。

## 诊断报告 待定

在 TUI 中输入 `/report` 可对当前论文的 output 产物进行诊断分析，产出可执行的发现和改进建议。

诊断覆盖四个维度：

- **质量** — 评审维度持续低分、合同履约率、改写率
- **规划** — 待定

#### 自定义代理

选择任意 Provider 后填写代理地址即可，或使用 Custom Proxy 并指定 API 协议类型。自定义代理的 `api_key` 可选；如果你的代理不需要认证，可以省略：

```jsonc
{
  "provider": "my-proxy",
  "model": "gpt-4o",
  "providers": {
    "my-proxy": {
      "type": "openai",
      "base_url": "https://proxy.example.com/v1"
    }
  }
}
```

支持的 Provider：`openrouter` / `anthropic` / `gemini` / `openai` / `deepseek` / `qwen` / `glm` / `grok` / `ollama` / `bedrock` 及任意自定义代理。

关于 `api_key`：

- `openrouter` / `anthropic` / `gemini` / `openai` / `deepseek` / `qwen` / `glm` / `grok` 这类托管接口通常需要填写 `api_key`
- `ollama` 和 `bedrock` 允许不填 `api_key`
- 显式指定了 `type` 的自定义代理允许不填 `api_key`

例如本地 `ollama` 配置：

```jsonc
{
  "provider": "ollama",
  "model": "qwen3:latest",
  "providers": {
    "ollama": {
      "base_url": "http://localhost:11434"
    }
  }
}
```

## 输出结构 待定


## 断点恢复

写一部长篇论文可能需要数小时甚至数天，中途崩溃、断网、Ctrl+C 都是常见情况。系统在**同一目录再次运行时自动恢复**，无需手动操作。

### 恢复场景 待定

| 中断时机                              | 恢复行为                               |
| ------------------------------------- | -------------------------------------- |

### 工作原理

所有创作产物持久化在 `output/` 目录。每个工具执行成功后写入 checkpoint。重启时：

1. 读取 `progress.json` + 最近 checkpoint + 待处理信号
2. 精确到 step 级生成恢复指令
3. 一次 `Prompt` 启动 Coordinator，进入继续创作

> 文件写入使用 temp + fsync + rename 原子操作，即使在写入过程中断电也不会损坏已有数据。

## 实时干预（Steer）

创作过程中可以随时通过输入框注入修改意见，**不需要暂停或重启**。

### TUI 模式 待定

创作启动后，底部输入框自动切换为干预模式：

输入后按 Enter，系统自动：

1. 记录干预指令到 `run.json`（崩溃恢复用）
2. 注入到正在运行的 Coordinator
3. Coordinator 评估影响范围，决定是修改设定、重写已有内容

### 干预示例

| 干预指令               | 系统可能的响应                         |
| ---------------------- | -------------------------------------- |


## 设计理念

> **把复杂度从代码搬到模型里。** 代码越少，能坏的地方越少。决策权交给更擅长做决策的角色。

### LLM 驱动，越简单越稳定 待定

- **决策权归 LLM** — 流程决策全部由 Coordinator 自主判断，Host 不介入。工具失败时返回结构化错误，由 LLM 自行决定重试或调整策略
- **工具只返事实** — 原子 IO + checkpoint 写入，返回值是 JSON 事实字段，不夹带任何指令字符串
- **Reminder 驱动每轮** — Host 在每轮 LLM 调用前读事实层，运行纯函数 generator 生成 `<system-reminder>` 注入，指令不进持久历史、每轮从事实重算
- **StopGuard 物理守门** — `Phase ≠ Complete` 时 Coordinator 物理上不可 `end_turn`，连续阻拦超限才升级终止
- **拒绝复杂编排** — 没有 task queue、没有 scheduler、没有 policy engine。Coordinator 的一次 Run 就是唯一的控制流
- **模型越强收益越大** — 架构把决策权留在 prompt 和工具语义里，模型升级后直接吃到收益，Host 一行不用改



## 技术栈

- **Go 1.25** — 主语言
- **[agentcore](https://github.com/voocel/agentcore)** — 极简 Agent 内核（tool-calling + streaming）
- **[litellm](https://github.com/voocel/litellm)** — 统一 LLM 接口适配
- **[Bubble Tea](https://github.com/charmbracelet/bubbletea)** — 终端 TUI 框架
