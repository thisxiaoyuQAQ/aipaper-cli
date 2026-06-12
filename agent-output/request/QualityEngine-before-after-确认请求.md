# QualityEngine before/after 主观验收确认请求

**日期**：2026-06-12
**请求人**：主 Agent（模块 31 三层验收）
**决策类型**：主观验收签字 + 真实 provider 补跑授权

---

## 背景

模块 31 第三层要求"短综述 before/after 验收"由用户主观确认质量提升，Agent 不得自评通过（vault/31 边界约定）。自动化部分已完成：

- mock 结构对照（`internal/e2e/before_after_test.go`）实测结果见 `docs/QualityEngine验收报告.md` 第四节；
- 真实 provider 对照因本机无 API key 且外部调用需授权而跳过。

## 请求一：主观验收签字

请审阅 `docs/QualityEngine验收报告.md` 第 4.2 节结构对照表，确认以下用户可感知差异是否成立：

- [X] enhanced 模式下每条 claim 都有 evidence 绑定，伪造/未确认引用被硬阻断；
- [X] unsupported claim 被显式标出并推动重写（旧流程完全不可见）；
- [X] `final/quality-report.md` 与 `report.md` Quality Summary 可读、有助于判断综述可信度；
- [X] 旧项目（无质量产物）恢复与导出不受影响。

**签字栏**：

> 验收结论（通过 / 需修改 / 拒绝）：通过
> 签字人：ZHIYU
> 日期：2026.6.13

## 请求二：真实 provider 对照补跑授权（可选）

如需用真实 LLM 跑 before/after（产生 API 费用），请提供以下任一项并明确授权：

1. 设置环境变量 `OPENAI_API_KEY` 或 `ANTHROPIC_API_KEY`（或在 `aipaper.json` 配置 `env:` 引用）；
2. 答复"授权真实 provider 对照"，Agent 将以 quality-mini 材料分别在兼容旧流程与 enhanced 模式各跑一次短综述（2 章、低字数），把真实对照结果补录到验收报告第四节。

不补跑不影响自动化验收结论；mock 结构对照已覆盖 spec 7.3 的结构性要求。

---

## 补跑记录（2026-06-13）

- 用户已在本文档提供 newapi 渠道（base_url `https://api.smmmc.cn`）与 API key，并指定模型 **gpt-5.5**，视为授权真实 provider 对照。
- 明文 key 已从本文档移除（仅以环境变量 `SMOKE_API_KEY` 注入运行进程，未写入任何落盘文件/git）；**建议用户轮换该 key**——它曾以明文短暂存在于工作区文件中。
- 补跑结果已录入 `docs/QualityEngine验收报告.md` 第 4.4 节；运行产物见 `agent-output/real-before-after/`（run-before / run-after）。
- 复跑方式：`SMOKE_API_KEY=... SMOKE_BASE_URL=https://api.smmmc.cn/v1 SMOKE_MODEL=gpt-5.5 go run ./tools/real-before-after`
