# 39-PaperQualitySkill运行时设计与接口

**状态**: 待开发

## 任务目标

完成“论文质量 Skill 运行时启用”的增量需求与接口落盘，明确 `docs/skills/paper-cli-paper-quality` 只作为蒸馏来源，运行时权威为本地 `PaperQualityPolicy`。

## 前置依赖

- 23-31 Quality Engine 已完成
- `docs/skills/paper-cli-paper-quality` 已存在

## 需要读取

- `docs/skills/paper-cli-paper-quality/SKILL.md`
- `docs/skills/paper-cli-paper-quality/references/prompt-modules.md`
- `docs/skills/paper-cli-paper-quality/references/quality-rubrics.md`
- `docs/interfaces/quality.md`
- `docs/superpowers/specs/2026-06-20-paper-quality-skill-runtime-design.md`

## 实现要求

1. 新增并维护 `docs/superpowers/specs/2026-06-20-paper-quality-skill-runtime-design.md`。
2. 新增并维护 `docs/interfaces/paper_quality_skill.md`。
3. 更新接口索引、需求/架构、开发进度和备忘录。
4. 明确非目标：不运行时读取 docs/skills、不依赖外部 skill runtime、不首期扩 `claim_type` schema。

## 测试要求

- 文档链接可读，接口索引能跳转到新增契约。
- 后续代码实现必须能追溯到本任务定义的边界。

## 验收标准

- 设计稿、接口文档、开发进度、备忘录均包含 Paper Quality Skill 运行时启用说明。
- 任务 40-45 已拆分清楚。

## 已知风险/边界

- 不把设计文档当运行时输入。
- 不改变已有质量 artifact 的 JSON 兼容性。
