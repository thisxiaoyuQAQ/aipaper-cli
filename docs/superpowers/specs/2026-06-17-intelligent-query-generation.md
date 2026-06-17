# 智能查询生成策略设计规范

**日期**：2026-06-17  
**状态**：待评审  
**问题来源**：用户反馈搜索结果不相关（主题"明朝君主哪个更厉害"返回大量朝鲜史和无关文献）

---

## 1. 问题陈述

### 1.1 当前行为

主题："明朝君主哪个更厉害"  
当前查询生成：
```
基础查询: "明朝君主哪个更厉害"
扩展查询: "明朝君主哪个更厉害 evidence literature review"
```

搜索结果：
- 90% 朝鲜"明君"（wise monarch）研究
- 日韩学术期刊文献
- 甚至卫星遥感等完全无关论文

### 1.2 根本原因

1. **语义破坏**：硬编码英文后缀"evidence literature review"破坏中文语义
2. **关键词误匹配**："明朝君主"被学术数据库拆解为"明君"+"主"，"明君"匹配到朝鲜史研究
3. **无中文优化**：查询生成策略未针对中文学术搜索优化
4. **无专有名词保护**：朝代名、人名等专有名词未被识别和保护

### 1.3 影响范围

- 中文历史、文化、社会科学类主题尤其严重
- 包含专有名词的主题容易被误解
- 跨语言查询时语义丢失

---

## 2. 设计目标

1. **提高相关性**：中文主题搜索结果相关率从 10% 提升到 70%+
2. **语言自适应**：根据主题语言自动选择合适的查询策略
3. **专有名词保护**：识别并保护朝代、人名、地名等
4. **多角度覆盖**：生成多个互补的查询，提高召回率
5. **向后兼容**：不破坏现有英文查询效果

---

## 3. 解决方案设计

### 3.1 整体架构

```
Requirements (Topic, Questions, Language)
    ↓
QueryGenerator
    ├─ LanguageDetector (检测主题语言)
    ├─ EntityRecognizer (识别专有名词)
    ├─ StrategySelector (选择查询策略)
    └─ QueryBuilder (构建查询组)
        ↓
Query[] (基础查询 + 扩展查询)
```

### 3.2 查询策略矩阵

| 语言 | 主题类型 | 基础查询 | 扩展策略 |
|------|---------|---------|---------|
| 中文 | 历史比较 | "{主题} 研究" | "{关键实体A} vs {关键实体B}", "{主题} 评价", "{主题} 比较分析" |
| 中文 | 通用学术 | "{主题} 综述" | "{主题} 研究现状", "{研究问题} 文献", "{主题} 实证研究" |
| 英文 | 通用学术 | "{topic}" | "{topic} systematic review", "{question} evidence literature review" |

### 3.3 语言检测规则

```go
func DetectLanguage(topic string) string {
    // 1. 优先使用 Requirements.Language（用户指定）
    // 2. 检测中文字符占比
    chineseRatio := countChinese(topic) / len([]rune(topic))
    if chineseRatio > 0.3 {
        return "zh-CN"
    }
    return "en"
}
```

### 3.4 实体识别规则（中文）

**朝代名识别**：
```
正则: (夏|商|周|秦|汉|三国|晋|南北朝|隋|唐|五代|宋|元|明|清|民国)(朝)?
示例: "明朝" → 实体类型=Dynasty
```

**人名模式**：
```
君主模式: (皇帝|君主|帝王|天子)
人名: 朱元璋、朱棣、康熙、乾隆等（可扩展词典）
```

**比较标记**：
```
关键词: (哪个|谁|比较|对比|vs|versus)
识别后 → 查询类型=Comparison
```

### 3.5 查询生成规则

#### 规则 1：中文历史比较查询

**触发条件**：语言=中文 AND 包含比较标记 AND 包含朝代/人名

**生成策略**：
```
基础查询: "{主题去除比较词} 研究"
示例: "明朝君主 研究" (去掉"哪个更厉害")

扩展查询1: "{朝代}{实体类型} 评价"
示例: "明朝皇帝 评价"

扩展查询2: "{朝代} 政治 史学研究"
示例: "明朝 政治 史学研究"

扩展查询3: "{提取的人名A} {提取的人名B} 比较"
示例: "朱元璋 朱棣 比较" (如果研究问题中提及)
```

#### 规则 2：中文通用学术查询

**触发条件**：语言=中文 AND 不匹配规则1

**生成策略**：
```
基础查询: "{主题} 综述"

扩展查询1: "{主题} 研究现状"

扩展查询2: "{研究问题} 文献综述"

扩展查询3: "{主题} {学科关键词}"
学科关键词根据主题自动推断或使用 Scope
```

#### 规则 3：英文学术查询（保持现有）

**触发条件**：语言=英文

**生成策略**：
```
基础查询: "{topic}"

扩展查询1: "{topic} systematic review"

扩展查询2: "{research question} evidence literature review"

扩展查询3: "{topic} empirical study"
```

### 3.6 查询质量评分（可选）

对生成的每个查询进行质量评分：

```go
func ScoreQuery(query string, topic string, language string) float64 {
    score := 1.0
    
    // 惩罚过短查询
    if len([]rune(query)) < 3 {
        score *= 0.5
    }
    
    // 惩罚过长查询（可能过于具体）
    if len([]rune(query)) > 50 {
        score *= 0.8
    }
    
    // 中文查询：惩罚包含英文学术后缀
    if language == "zh-CN" && containsEnglishSuffix(query) {
        score *= 0.3
    }
    
    // 奖励包含主题核心词
    if containsTopicKeywords(query, topic) {
        score *= 1.2
    }
    
    return score
}
```

---

## 4. 接口设计

### 4.1 新增类型

```go
// QueryStrategy 查询生成策略
type QueryStrategy struct {
    Language      string   // zh-CN, en
    TopicType     string   // comparison, general, review
    Entities      []Entity // 识别的实体
    Keywords      []string // 关键词
}

// Entity 识别的实体
type Entity struct {
    Text string // 实体文本
    Type string // Dynasty, Person, Place, etc.
    Start int   // 在原文中的起始位置
    End   int
}

// QueryWithMetadata 带元数据的查询
type QueryWithMetadata struct {
    Query
    Strategy    QueryStrategy
    Score       float64  // 质量评分 0-1
    Reason      string   // 生成原因说明
}
```

### 4.2 修改函数签名

```go
// 替换现有的 QueryFromRequirements
func GenerateQueries(req contracts.Requirements, limit int) []QueryWithMetadata

// 替换现有的 ExpansionQueriesFromRequirements  
func GenerateExpansionQueries(req contracts.Requirements, base QueryWithMetadata, needed int) []QueryWithMetadata
```

### 4.3 向后兼容

保留现有函数作为简单包装：

```go
func QueryFromRequirements(req contracts.Requirements, limit int) Query {
    queries := GenerateQueries(req, limit)
    if len(queries) > 0 {
        return queries[0].Query
    }
    // fallback
}
```

---

## 5. 实现计划

### 5.1 模块拆分

```
internal/search/
├── query_generator.go       # QueryGenerator 主逻辑
├── language_detector.go     # 语言检测
├── entity_recognizer.go     # 实体识别（中文）
├── strategy_selector.go     # 策略选择
├── query_builder.go         # 查询构建
├── query_generator_test.go  # 单元测试
└── types.go                 # 类型定义（扩展现有）
```

### 5.2 实施步骤

**Step 1**: 语言检测与策略选择 (2h)
- 实现 `DetectLanguage`
- 实现 `SelectStrategy` 根据语言和主题特征选择策略

**Step 2**: 中文实体识别 (3h)
- 朝代名识别（正则）
- 比较标记识别
- 人名识别（简单词典）

**Step 3**: 查询生成器核心 (4h)
- 实现 `GenerateQueries`
- 实现三种策略的查询生成
- 查询质量评分

**Step 4**: 集成与测试 (3h)
- 修改 `run.go` 使用新生成器
- 编写单元测试（覆盖率 > 85%）
- 编写集成测试（真实案例对比）

**Step 5**: 文档与验收 (2h)
- 更新用户文档
- before/after 对比验收

**总计**：约 14 小时（2 个工作日）

---

## 6. 测试用例

### 6.1 中文历史比较主题

**输入**：
```json
{
  "topic": "明朝君主哪个更厉害",
  "research_questions": ["朱元璋和朱棣的政绩比较"],
  "language": "zh-CN"
}
```

**期望输出**：
```
基础查询: "明朝君主 研究"
扩展查询1: "明朝皇帝 评价"
扩展查询2: "明朝 政治 史学研究"
扩展查询3: "朱元璋 朱棣 比较"
```

### 6.2 中文通用学术主题

**输入**：
```json
{
  "topic": "深度学习在医学影像中的应用",
  "language": "zh-CN"
}
```

**期望输出**：
```
基础查询: "深度学习在医学影像中的应用 综述"
扩展查询1: "深度学习在医学影像中的应用 研究现状"
扩展查询2: "医学影像 深度学习 文献综述"
```

### 6.3 英文学术主题（保持现有）

**输入**：
```json
{
  "topic": "Retrieval augmented generation",
  "research_questions": ["How does RAG improve accuracy"],
  "language": "en"
}
```

**期望输出**：
```
基础查询: "Retrieval augmented generation"
扩展查询1: "Retrieval augmented generation systematic review"
扩展查询2: "How does RAG improve accuracy evidence literature review"
```

---

## 7. 验收标准

### 7.1 功能验收

- [ ] 中文主题不再硬编码英文后缀
- [ ] 朝代名、比较标记正确识别
- [ ] 生成查询数量符合预期（base + expansion）
- [ ] 查询质量评分合理（无低分查询进入搜索）

### 7.2 效果验收

使用真实案例对比 before/after：

| 主题 | Before 相关率 | After 相关率 | 目标 |
|------|--------------|-------------|------|
| 明朝君主哪个更厉害 | ~10% | ≥70% | ✓ |
| 深度学习医学应用 | ~40% | ≥75% | ✓ |
| RAG 研究（英文） | ~80% | ≥80% | 保持 |

相关率定义：前 10 个候选中，标题/摘要与主题明确相关的比例

### 7.3 性能验收

- 查询生成耗时 < 50ms（单次）
- 不影响现有搜索总体耗时（网络 IO 为主）

---

## 8. 风险与缓解

### 8.1 风险

**R1**: 实体识别规则不够全面，遗漏关键词  
**缓解**: 采用宽松匹配 + 人工扩展词典，阶段性迭代

**R2**: 中文查询在英文为主的数据库效果仍不佳  
**缓解**: 
- 提供双语查询选项（中文主题 + 英文翻译）
- 优先使用支持中文的数据源（CNKI, 万方等，后续扩展）

**R3**: 查询策略过于复杂，难以维护  
**缓解**: 
- 策略配置化（YAML/JSON 定义规则）
- 每个策略独立单元测试
- 文档清晰记录每条规则的适用场景

### 8.2 回退方案

如果新策略效果不佳：
- 保留旧策略作为 fallback
- 提供配置开关 `QUERY_STRATEGY=legacy|intelligent`
- 用户可在 SearchProgress 屏幕手动修改查询

---

## 9. 后续扩展

**Phase 2** (可选，不在当前 scope)：
- 支持用户自定义查询模板
- 机器学习查询优化（基于历史确认率）
- 集成 LLM 生成更智能的查询（调用 OpenAI/Claude API）
- 支持更多中文学术数据源（CNKI, 万方, 维普）

**Phase 3** (可选)：
- 查询分析与推荐（TUI 中展示"建议的查询"）
- A/B 测试不同策略效果
- 用户反馈循环（记录确认的文献反推查询质量）

---

## 10. 参考资料

- Google Scholar 中文查询优化实践
- Semantic Scholar API 文档
- 中文 NLP 实体识别标准
- 学术搜索查询优化论文（待补充）

---

**审批流程**：
1. 用户确认问题描述和目标 ✓
2. 技术方案评审（待定）
3. 实施与验收（待定）
