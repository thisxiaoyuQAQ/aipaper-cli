# quality-mini fixture

Quality Engine 第二层「证据链夹具」验收材料（模块 31，spec 7.2）。

配套测试：`internal/e2e/quality_mini_test.go`（全程 mock agent，无网络）。

夹具内容：

- `materials/cbt_insomnia.md`、`materials/digital_sleep.md`：两份短材料，供 snippet 级 evidence 引用 extracted 文本；
- `materials/refs.bib`：3 条 BibTeX 候选，其中 `lee2025unconfirmed` 在测试里**故意不确认**，用于触发未确认引用硬阻断。

测试中埋入的坏样本与期望行为：

| 埋入问题 | 期望行为 |
|---|---|
| draft 引用未确认文献 key | Writer guard / gate 硬阻断 |
| claim 完全没绑定 evidence | Writer guard / gate 硬阻断 |
| 伪造引用 key | Writer guard / gate 硬阻断 |
| abstract 级证据 + 绝对化结论（risk_level=high） | enhanced → warning，strict → needs_revision |
| 同一论断在两章重复 | duplicate_of 分级风险标记，不阻断 |
| 重写超 2 轮仍不达标 | needs_human_review，流程不中断 |
| fast 模式 | 硬阻断仍生效，分级风险降为 warnings-only |
