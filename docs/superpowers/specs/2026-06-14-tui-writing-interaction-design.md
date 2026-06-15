# TUI 写作页滚动、干预与安全暂停设计

## 背景

当前写作/任务执行页采用四区布局：指标、日志、正文预览、章节进度。用户反馈任务执行时日志会把框拉伸，页面观感不稳定；同时希望在生成过程中能实时追加人工指令，并把暂停/退出快捷键调整为更自然的语义：`Esc` 暂停、`Enter` 继续、`Ctrl+C` 退出程序。

本设计目标是一次性改善写作页交互闭环：四区内容不再撑破布局，每区可独立滚动；底部输入框支持追加指令；暂停采用 checkpoint 安全点，避免硬中断导致草稿或恢复状态损坏。

## 范围

本次改动覆盖：

- `WritingProgress` 四区布局的固定边界、独立滚动、焦点切换和滚动条。
- 写作页底部追加指令输入框。
- `Esc` 安全点暂停、`Enter` 继续、`Ctrl+C` 退出/退出确认。
- TUI 到运行时控制层的控制消息：暂停、继续、追加指令。
- 对应单元测试、集成测试和用户文档更新。

本次不做：

- 不实现直接编辑草稿正文的富文本/多光标编辑。
- 不做硬中断式暂停；暂停必须以 checkpoint/安全边界为准。
- 不把鼠标命中区域作为第一阶段强要求；优先支持滚轮滚动当前焦点区，后续再增强为“鼠标在哪区就滚哪区”。
- 不重新设计四区信息架构。

## 推荐方案

采用“TUI 完整升级 + 运行时安全点协议”的方案。

TUI 层负责稳定布局、焦点、滚动、输入框和快捷键语义。运行时层新增轻量控制协议，支持：

- 请求在最近安全点暂停。
- 从最新 checkpoint 继续。
- 将人工追加指令排队，并在最近可注入边界带入后续生成。

如果底层某个步骤暂时不能热注入指令，TUI 仍保留该指令并显示 pending 数量，确保用户输入不丢失、不伪装成已即时生效。

## TUI 交互设计

### 四区固定布局

写作页继续保留四区：

1. 指标区
2. 日志区
3. 正文预览区
4. 章节进度区

每个区域都按当前终端尺寸计算固定宽高。渲染前先将内容转换为适合区域宽度的行，再截取当前 viewport 可见范围。任何日志、正文或章节标题都不得通过 Lip Gloss 自然宽度反向撑大父布局。

关键文件：

- `internal/tui/writing/model.go`
- `internal/tui/writing/view.go`
- `internal/tui/app/program.go`
- `internal/tui/app/root.go`

### 独立滚动与滚动条

每个区域维护自己的滚动状态：

- `metricsScroll`
- `logsScroll`
- `contentScroll`
- `progressScroll`

每个区域右侧显示轻量滚动条。滚动条只表达当前位置和总量，不需要复杂样式。内容不足一屏时隐藏或显示空轨道均可，但必须保持区域宽度稳定。

日志长行需要按可用宽度安全处理。优先使用 rune/cell-width 安全截断或换行工具，避免直接按 byte 切割 UTF-8 文本。

### 焦点与键盘

写作页新增“当前焦点区域”：

- `Left` / `Right`：在四个区域之间切换焦点。
- `Up` / `Down`：滚动当前焦点区域。
- 焦点区域的边框或标题使用强调色标识。
- 原有 `Space` 自动滚动语义可以保留给日志区；但焦点滚动是主交互。

### 鼠标滚轮

Bubble Tea 程序开启鼠标支持。第一阶段实现为：鼠标滚轮滚动当前焦点区域。

后续如果能稳定维护各区域屏幕坐标，再增强为：滚轮事件落在哪个区域，就滚动哪个区域。

### 底部输入框

四区下方新增单行输入框，用于输入“追加指令”。

行为：

- 普通字符输入进入输入框。
- `Backspace` 删除字符。
- `Enter` 在输入框有内容时提交追加指令。
- 提交后清空输入框，日志显示“已提交人工指令”，并增加 pending 指令计数。
- `Enter` 在输入框为空且状态为 paused 时执行继续生成。

## 运行时控制设计

新增或扩展写作页控制消息：

- `PauseRequested`
- `ResumeRequested`
- `InstructionSubmitted`

### Esc：安全点暂停

`Esc` 不再退出写作页，而是请求安全点暂停。

状态流：

```text
Running --Esc--> PauseRequested --checkpoint saved--> Paused
```

在 `PauseRequested` 状态下，TUI 显示“正在等待安全点暂停”，运行时不应开始新的大步骤；当前正在执行且不可安全中断的步骤允许完成到 checkpoint。

### Enter：继续生成

暂停后，用户按 `Enter` 继续。

恢复时从最新 checkpoint/recovery prompt 继续，并将暂停期间提交的 pending instructions 作为人工补充要求带入后续生成。

### 追加指令

输入框提交的内容不是直接改草稿，而是作为人工补充要求进入运行时队列。

处理原则：

- 能在当前安全边界注入时，尽快注入。
- 不能热注入时，排队到最近安全点。
- 不丢弃用户输入。
- UI 显示 pending 数量，避免误导用户以为每条指令都已立刻生效。

### Ctrl+C：退出程序

`Ctrl+C` 改为退出语义。

运行中按下 `Ctrl+C` 时，进入退出确认或直接走现有全局退出确认路径。确认退出后停止/取消运行时并退出程序；取消确认后回到写作页。

这与旧设计“Ctrl+C 请求保存后停止”不同。文档和页脚快捷键提示必须同步更新。

## 数据流

```text
tea.KeyMsg / tea.MouseMsg
  -> writing.Model.Update
      -> 更新焦点/滚动/输入框状态
      -> 或发出写作控制消息
  -> app.RootModel.Update
      -> 将控制消息转发给 writing runtime controller
      -> runtime 在安全边界 pause/resume/inject instruction
      -> runtime events 回流 RuntimeEventMsg
  -> writing.Model.handleRuntimeEvent
      -> 更新日志、正文、章节、usage、pause 状态
  -> writing.View
      -> 固定四区 + 滚动条 + 输入框 + 状态页脚
```

## 错误处理

- 指令提交失败：输入框不丢内容，日志显示失败原因。
- 暂停请求失败：状态回到 running，并显示错误日志。
- resume 失败：保持 paused，提示用户可重试或退出。
- 无 checkpoint 可恢复：保持 paused/error 状态，并显示“无法继续，需要退出后恢复运行”的明确提示。
- 鼠标不可用或终端不支持鼠标：键盘焦点和滚动仍可完整使用。

## 测试计划

### `internal/tui/writing`

新增/更新单元测试覆盖：

- 长日志不会撑宽布局。
- 四区滚动状态互不影响。
- `Left` / `Right` 切换焦点。
- `Up` / `Down` 只滚动当前焦点区。
- 滚动条渲染存在且不改变区域宽度。
- 输入框字符输入、退格、提交、清空。
- `Esc` 进入 pause requested 状态。
- paused + 空输入框 + `Enter` 发出 resume。
- 非 paused + 空输入框 + `Enter` 不误触发继续。

### `internal/tui/app`

新增/更新集成测试覆盖：

- 写作页控制消息路由到 runtime controller。
- `Ctrl+C` 在写作页进入退出确认/退出流程，而不是暂停流程。
- runtime pause/resume/instruction 事件回流后，写作页状态正确更新。

### `internal/app` 或 runtime controller

新增/更新测试覆盖：

- 收到 pause request 后，在 checkpoint 安全点进入 paused。
- resume 时携带 pending instructions。
- instruction queue 不丢数据，并能在安全边界消费。

### E2E 与文档

- 更新 `docs/user-guide.md` 和相关 TUI 文档中的快捷键说明。
- 更新 e2e 文档断言中关于 `Ctrl+C` 的旧描述。

建议验证命令：

```bash
go test ./internal/tui/writing ./internal/tui/app ./internal/app ./internal/e2e
go test ./...
```

## 验收标准

- 任务执行时，任何日志内容都不会把四区布局撑变形。
- 四个区域可独立滚动，并有右侧滚动条反馈位置。
- 键盘可切换焦点区域并滚动当前区域。
- 鼠标滚轮至少能滚动当前焦点区域。
- 底部输入框可提交追加指令，提交后可在 UI 中看到已排队/已提交反馈。
- `Esc` 请求安全点暂停；checkpoint 后进入 paused。
- paused 状态下 `Enter` 能继续生成。
- `Ctrl+C` 用于退出程序/退出确认，不再作为暂停快捷键。
- 新增和既有测试通过。
