# 40-PaperQualityPolicy本地策略底座

**状态**: 待开发

## 任务目标

新增 `internal/quality/paper_quality_policy.go`，把论文质量 skill 内容本地化为无文件 IO、可测试、可按角色注入的运行时策略 helper。

## 前置依赖

- 39-PaperQualitySkill运行时设计与接口

## 需要读取

- `docs/interfaces/paper_quality_skill.md`
- `docs/skills/paper-cli-paper-quality/references/prompt-modules.md`
- `docs/skills/paper-cli-paper-quality/references/quality-rubrics.md`
- `internal/quality/template.go`

## 实现要求

1. 新增 `PaperQualityPolicyVersion = "paper-cli-paper-quality-v1"`。
2. 新增 `PaperQualityPolicy` 结构或等价纯函数。
3. 提供 Coordinator、Architect、EvidenceDepth、SectionPlan、Writer、Verifier、Editor、Report 各 scope 的短规则。
4. 所有函数必须无文件 IO、无 Store、无 LLM 调用，输出顺序稳定。
5. 规则应压缩，不复制整份文档。

## 测试要求

- 新增 `internal/quality/paper_quality_policy_test.go`。
- 测试 version、各 scope 非空、关键短语存在、重复调用输出稳定。

## 验收标准

- `go test ./internal/quality` 通过。
- 后续角色 prompt 可复用同一 helper，避免规则散落。

## 已知风险/边界

- 首期不新增 `claim_type` schema。
- 不把 policy 变成 gate matrix 的新硬阻断。
