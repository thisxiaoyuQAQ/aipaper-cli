# Quality Engine 验收报告

**项目名称**：aipaper-cli
**验收日期**：2026-06-12
**验收范围**：Quality Engine 模块 23-31（三层验收，spec `2026-06-10-quality-engine-design.md` 第 7 节）
**验收方式**：主 Agent 按 vault/31 执行；主观部分待用户签字（见第五节）

---

## 一、验收结论

✅ **验收通过（含主观签字）** — 三层验收全部完成：第一、二层自动化通过；第三层 mock 结构对照 + 真实 provider 对照（gpt-5.5）完成，用户已主观签字确认（验收结论：通过，签字人 ZHIYU，2026-06-13，见确认请求文档签字栏）。

---

## 二、第一层：自动化结构验收

| 验证项 | 命令 | 结果 |
|---|---|---|
| 编译检查 | `go build ./...` | ✅ exit 0 |
| 全量测试 | `go test ./...` | ✅ 全部通过（0 FAIL） |
| 可执行构建 | `go build -o aipaper-cli.exe ./cmd/aipaper-cli` | ✅ 通过 |

23-30 模块测试覆盖复核（spec 7.1 五项）：

| 7.1 要求 | 证据（测试文件） |
|---|---|
| `internal/quality` schema 校验、原子写入、严格 JSON 读取 | `evidence_test.go`、`sectionplan_test.go`、`claimgraph_test.go`、`verification_test.go` |
| 工具层校验（未确认 key 拒绝 / evidence 不存在拒绝 / claim 无 evidence 阻断） | `internal/agent/writer_quality_test.go`（WriteGuardedDraftBundle 三类阻断逐项测试） |
| `quality_gate_check` 三档门控矩阵 | `internal/quality/gate_test.go`（`TestGateMatrixAcrossModes` 同输入三模式逐行断言、`TestGateHardBlocksEveryMode`） |
| checkpoint 恢复（新 step 中断续跑、产物哈希校验） | `internal/agent/quality_test.go`、`claim_quality_test.go`、`verification_quality_test.go`（CheckpointAndResume、幂等回放） |
| TUI（模式选择落盘、WritingProgress 渲染、ExportSummary 结论行、RecoverPrompt 文案） | `internal/tui/app/state_probe_quality_test.go`、`internal/tui/requirements`、`exportsummary`、`writing` 各包测试 |

复核中发现并修复 1 个环境耦合用例：`cmd/aipaper-cli/main_test.go` 的
`TestRunWithoutArgsCallsTUIRunner` 未隔离 HOME，本机存在 `~/.aipaper/config.json`
时 StateProbe 探测结果变为 `requirements` 导致失败。已在测试内将
HOME/USERPROFILE 指向空临时目录（commit `c526b61`），属测试缺陷，非产品缺陷。

---

## 三、第二层：证据链夹具验收（fixtures/quality-mini）

夹具：`fixtures/quality-mini/`（2 份材料 + 3 条 BibTeX 候选，`lee2025unconfirmed` 故意不确认）。
测试：`internal/e2e/quality_mini_test.go`（全程 mock agent、无网络）。

| 埋入问题 | 期望行为 | 结果 | 断言位置 |
|---|---|---|---|
| draft 引用未确认文献 key | Host 硬阻断 | ✅ `WRITER_CLAIM_UNCONFIRMED_REFERENCE`，章节产物不落盘 | `TestQualityMiniHardBlocksAtWriterGuard` |
| claim 完全没绑定 evidence | Host 硬阻断 | ✅ `WRITER_CLAIM_MISSING_EVIDENCE`，不落盘 | 同上 |
| 伪造引用 key | Host 硬阻断 | ✅ `WRITER_CLAIM_UNCONFIRMED_REFERENCE`（机器无法区分未确认与伪造，一律阻断） | 同上 |
| abstract 级证据 + 绝对化结论（verifier 标 risk_level=high） | enhanced → warning，strict → needs_revision | ✅ `gate_shallow_evidence_strong_claim` 严重度随模式升级 | `TestQualityMiniGateMatrixAndDuplicates` |
| 同一论断在两章重复 | 分级风险标记 | ✅ `duplicate_of=[claim_001]`、`gate_duplicate_claim` warning，不阻断 | 同上 |
| 重写 2 轮仍不达标（rounds=3） | needs_human_review，流程不中断 | ✅ 结论 `needs_human_review`、gate 正常返回无 error；strict 下 `top_priority=true` 且置顶 | 同上 |

fast 模式覆盖：

- 硬阻断仍生效：`TestQualityMiniGateHardBlocksEveryMode` 在 fast/enhanced/strict 三模式下对绕过上游 guard 的陈旧绑定（伪造 key、无 evidence、未知 evidence id）全部 `blocked`；writer guard 本身不感知模式，天然全模式生效。
- 分级风险降为 warnings-only：fast 下 unsupported/浅证据/重复全部 warning，结论 `pass_with_warnings`。

---

## 四、第三层：短综述 before/after 对照

### 4.1 运行方式与跳过说明

- **真实 provider 对照：已补跑（2026-06-13）**。用户在确认请求文档提供 newapi 渠道（`https://api.smmmc.cn`）并指定模型 `gpt-5.5`，视为授权；结果见 4.4 节。初次验收时（2026-06-12）因本机无 API key 曾跳过，跳过原因保留如下：本机环境无 `OPENAI_API_KEY` / `ANTHROPIC_API_KEY` / `AIPAPER_API_KEY`，且按项目备忘录约定真实 provider 调用属外向操作须用户事先授权（沿用模块 21/22 做法）。
- **mock 结构对照：已完成**。`internal/e2e/before_after_test.go`（`TestE2EBeforeAfterStructuralComparison`）：同一组 quality-mini 材料（2 章、低字数）分别走 **兼容旧流程**（无质量产物、claims 不带 evidence_ids、直接 `WriteDraftBundle`）与 **enhanced 模式**（证据表 → writer guard → claim 抽取 → verification → gate → 导出）。
- 材料说明：vault 建议"可扩充 fixtures/materials"；因 `fixtures/materials` 被材料解析单测按条目数断言锁定，本层复用了为模块 31 新建的 `fixtures/quality-mini/materials`（真实风格短材料，2 章题材），未动 `fixtures/materials`。

### 4.2 结构对照结果（测试实测输出）

| 对照维度 | before（兼容旧流程） | after（enhanced） |
|---|---|---|
| 大纲约束 | 无写前约束产物 | SectionQualityPlan 写前约束可用（模块 24；本对照未注入 plan，evidence 链从 writer guard 起生效） |
| 正文论证（evidence 绑定） | 0/3 claims 绑定 evidence，兼容 warning ×3（`ARTIFACT_CLAIM_MISSING_EVIDENCE`，不阻断） | 3/3 claims 绑定 evidence，guard 硬校验通过后才落盘 |
| claim 支撑率 | 不可知（无 verification） | supported 2/3，verdicts 3/3 |
| unsupported claim 数 | 不可见（旧流程无此概念） | 1 条被显式标出（"Digital interventions never work for shift workers"），gate 结论 `needs_revision` |
| report 可读性 | `report.md` 提示 "Quality artifacts missing (compatibility mode)"，`quality-report.md` 不生成且导出不阻塞（issue `EXPORT_QUALITY_ARTIFACTS_MISSING`） | `report.md` 含 Quality Summary；`final/quality-report.md` 生成，含门控/证据深度/支持度/重写/下一步五节；TUI metadata `质量门控：1 章需要修订` |

### 4.3 验收中发现并修复的问题

- 模块 29 `buildQualityConclusion` 把 findings 条数当章数展示（实测 "3 章需要修订" 实为 3 条 findings 分布在 2 章）。已改为按严重度去重统计章节数（commit `c802c49`），修复后实测输出 "质量门控：1 章需要修订"，与 needs_revision 严重度仅命中 ch02 一章一致。
- 行为记录（非缺陷）：兼容模式下 `ExportFinal` 不写 `Metadata["quality_conclusion"]`（留空由 TUI 按兼容模式渲染），`buildQualityConclusion` 的兼容分支为防御性代码。

### 4.4 真实 provider 对照（gpt-5.5，2026-06-13 补跑）

**重要前置发现**：复核中确认产品代码里 **TUI→AgentRuntime 接线缺失**——`WritingProgress.Init()` 返回 nil（注释仍为 "In real implementation, this would launch AgentRuntime"），`app.NewAgentRuntime` 在全部分支历史中无任何 TUI/CLI 调用点，Writer/Editor 亦无 LLM runner 实现。即模块 22「真实 Runtime 接入」的完成声明与代码不符，真实 provider 全流程无法经产品路径运行（见第六节遗留事项 3）。

因此本次对照经 `tools/real-before-after/` harness 直接驱动真实 LLM 走产品契约（config→litellm 链路、`WriteDraftBundle`/`WriteGuardedDraftBundle`、`ExtractChapterClaimGraph`、`SaveVerificationResult`、`ExportFinal` 全为产品代码；Editor 评分两侧均为 mock 通过，使对照只隔离证据协议变量）：

- 连通性 smoke：endpoint 经产品 config→litellm 路径单次调用成功（先以 deepseek-v4-pro 验证 1.5s/60 tokens，后按用户指定改用 gpt-5.5 跑全程）。
- 材料：`fixtures/quality-mini/materials`（2 章、低字数，约 300 词目标），确认 chen/garcia 两篇、lee2025 故意不确认——与 mock 对照同构。

| 对照维度 | before（兼容旧流程，gpt-5.5 真实写作） | after（enhanced，gpt-5.5 真实写作+真实 verifier） |
|---|---|---|
| 写前约束 | 无 | 真实 Architect 生成 SectionQualityPlan（questions/required_evidence/boundaries），注入 Writer 提示词 |
| evidence 绑定 | 8 条 claim 全部无绑定（兼容 warning） | 9 条 claim 全部绑定 evidence，writer guard 一次通过、零阻断 |
| claim 支撑核算 | 不可知（无 verification） | 真实 verifier 9/9 verdict：supported 5、partially_supported 4、unsupported 0 |
| 真实抓到的问题 | 同类过度声称混在草稿中不可见（如 before 的 ch01_claim_002 同样断言 shift workers 依从性下降，无任何溯源） | verifier 逐条点名 4 处 partially_supported 并给出理由（如 claim_001："证据未明确… "；gate 产出 4 条 `gate_partially_supported_claim` warning） |
| 门控结论 | 无（compat） | `pass_with_warnings`；TUI metadata "质量门控：通过但有 4 条警告" |
| 导出产物 | 5 个（无 quality-report，issue `EXPORT_QUALITY_ARTIFACTS_MISSING`） | 6 个（含 `final/quality-report.md`，支持度/证据深度/风险表完整） |

运行产物：`agent-output/real-before-after/run-before/`、`run-after/`（已核查不含任何密钥）。结论：真实模型（gpt-5.5）在 enhanced 流程下每条论断可溯源、过度声称被逐条标记，与 mock 结构对照的结论一致且更直观，供用户主观签字参考。

---

## 五、spec 7.4 验收总线

| 验收项 | 验证方式 | 状态 |
|---|---|---|
| 质量产物完整、可恢复、可校验 | 第一层 | ✅ 通过（23-27 schema/原子写入/恢复测试） |
| 底线问题必被硬阻断 | 第二层夹具 | ✅ 通过（writer guard 三类 + gate 四类全模式） |
| 三档模式门控行为正确 | 第一层矩阵 + 第二层夹具 | ✅ 通过（gate_test 逐行 + quality-mini 模式对照） |
| 旧项目恢复不被破坏 | 第一层恢复测试 | ✅ 通过（claims 兼容读取、writer 兼容警告、StateProbe 质量探测、before 流导出不阻塞） |
| 用户可感知质量提升 | 第三层 before/after | ⏳ mock 结构对照通过；主观签字待用户（见确认请求） |

---

## 六、遗留事项

1. ~~真实 provider before/after 对照待用户提供 API key 并授权后补跑~~ → 已于 2026-06-13 以 gpt-5.5 补跑完成（见 4.4 节）。
2. ~~主观验收签字~~ → 已签字：通过（ZHIYU，2026-06-13），见 `agent-output/request/QualityEngine-before-after-确认请求.md` 签字栏。
3. **模块 22 接线缺口（建议走《Bug修复》流程）**：TUI 的 WritingProgress 从未真正启动 `app.NewAgentRuntime`，Writer/Editor 无 LLM runner 实现，`docs/开发进度.md` 中模块 22「真实 Runtime 接入」的完成记录与代码不符。在补齐接线前，真实 provider 只能经 `tools/real-before-after` harness 驱动，无法从 TUI 端到端运行。
4. 密钥卫生：用户提供的 newapi key 曾以明文写入工作区文件（已移除、未进 git），建议轮换。
