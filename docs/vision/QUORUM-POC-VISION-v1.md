# QUORUM-AI: POC Vision Document v1

## Document Metadata

| Field | Value |
|-------|-------|
| **Version** | v1.0 |
| **Date** | 2026-01-11 |
| **Status** | Active |
| **Scope** | Proof of Concept (POC) |

---

## 1. Problem Statement

### 1.1 Core Challenges

Large Language Models (LLMs) present significant reliability challenges in software engineering workflows:

| Challenge | Impact | Frequency |
|-----------|--------|-----------|
| **Hallucinations** | Incorrect code, fabricated APIs, non-existent libraries | High |
| **Output Variability** | Same prompt yields different results across runs | Medium-High |
| **Model Bias** | Systematic errors specific to training data | Medium |
| **Overconfidence** | Incorrect assertions stated with high certainty | High |
| **Context Loss** | Important details missed in complex prompts | Medium |

### 1.2 Current Limitations

Single-agent LLM interactions suffer from:

1. **No Cross-Validation**: Outputs are not verified against alternative perspectives
2. **Hidden Errors**: Confidence scores do not correlate with accuracy
3. **No Dialectic Process**: Errors are not challenged or refined
4. **Single Point of Failure**: One model's limitations become the user's limitations

---

## 2. Hypothesis

**Ensemble of autonomous LLM agents with consensus-based validation reduces errors and increases output reliability compared to single-agent execution.**

### 2.1 Theoretical Foundation

This approach draws from established principles:

| Principle | Application in quorum-ai |
|-----------|--------------------------|
| **Ensemble Methods** | Multiple models reduce individual bias |
| **Wisdom of Crowds** | Aggregated judgment outperforms individuals |
| **Dialectic Process** | Thesis-antithesis-synthesis refines understanding |
| **Cross-Validation** | Independent verification catches errors |

### 2.2 Key Assumptions

1. Different LLMs have different failure modes
2. Consensus among independent agents signals higher reliability
3. Structured disagreement surfaces hidden errors
4. Human oversight remains essential for edge cases

---

## 3. Approach: Multi-Agent Ensemble with Consensus Protocol

### 3.1 Agent Architecture

quorum-ai orchestrates multiple autonomous CLI-based LLM agents:

| Agent | CLI Tool | Primary Use Case |
|-------|----------|------------------|
| Claude | `claude` | Analysis, planning, code generation |
| Gemini | `gemini` | Alternative perspective, validation |
| Codex | `codex` | Code-focused tasks (optional) |
| Copilot | `gh copilot` | GitHub-integrated tasks (optional) |

### 3.2 Workflow Phases

The POC implements a three-phase workflow:

```
ANALYZE -> PLAN -> EXECUTE
```

**Phase 1: Analyze**
- Agents independently analyze the task
- Outputs compared using consensus algorithm
- Divergences trigger dialectic refinement

**Phase 2: Plan**
- Consolidated analysis informs plan generation
- Plan parsed into executable tasks with dependencies
- DAG built for parallel execution

**Phase 3: Execute**
- Tasks executed in isolated git worktrees
- Results validated before merge
- Human review gates critical operations

### 3.3 Consensus Protocol: V1/V2/V3 (Thesis-Antithesis-Synthesis)

The dialectic protocol ensures iterative refinement:

| Round | Name | Purpose |
|-------|------|---------|
| **V1** | Thesis | Initial independent analysis by all agents |
| **V2** | Antithesis | Agents critique each other's V1 outputs |
| **V3** | Synthesis | Single agent reconciles divergences |

**Escalation Logic:**

```
V1 Consensus >= 80%  ->  Proceed to PLAN
V1 Consensus >= 60%  ->  Run V2 (critique)
V1 Consensus < 60%   ->  Run V2 + V3 (full dialectic)
Any Consensus < 50%  ->  Human Review Required
```

### 3.4 Consensus Algorithm: Weighted Jaccard Similarity

Agreement is measured using Jaccard similarity across categorized content:

```
Jaccard(A, B) = |A intersection B| / |A union B|
```

**Category Weights:**

| Category | Weight | Rationale |
|----------|--------|-----------|
| Claims | 0.40 | Core factual assertions |
| Risks | 0.30 | Identified concerns and edge cases |
| Recommendations | 0.30 | Proposed actions |

**Consensus Score Calculation:**

```
ConsensusScore = Sum(CategoryWeight * AvgJaccardPerCategory)
```

---

## 4. Success Metrics

### 4.1 Primary Metrics (POC Exit Criteria)

| Metric | Target | Measurement |
|--------|--------|-------------|
| Workflow Success Rate | >= 80% | completed / total |
| Average Consensus Score | >= 75% | avg(consensus.score) |
| Cost per Workflow | <= $5 USD | sum(cost_usd) |
| PRs Without Manual Changes | >= 60% | automated / total PRs |
| Test Coverage | >= 80% | go test -cover |

### 4.2 Secondary Metrics (Observational)

| Metric | Target | Measurement |
|--------|--------|-------------|
| Analyze Phase Duration | <= 30 min | timestamps |
| Plan Phase Duration | <= 15 min | timestamps |
| Execute Phase Duration | <= 60 min (3 tasks) | timestamps |
| V3 Invocation Rate | <= 30% | workflows_with_v3 / total |
| Retry Rate | <= 20% | sum(retries) / count(tasks) |

### 4.3 Validation Experiments

The POC includes controlled experiments:

1. **Threshold Sensitivity**: Test consensus thresholds (0.70, 0.75, 0.80, 0.85, 0.90)
2. **Single vs Multi-Agent**: Compare error rates between single and ensemble execution
3. **Cost Analysis**: Measure token usage and cost per workflow phase
4. **V3 Effectiveness**: Measure quality improvement from synthesis round

---

## 5. POC Scope

### 5.1 In Scope (v1.0)

| Component | Implementation |
|-----------|----------------|
| CLI Framework | Cobra with global flags |
| Configuration | Viper with YAML/ENV/flags hierarchy |
| State Persistence | JSON with atomic writes and locking |
| Agent Adapters | Claude (primary), Gemini (secondary) |
| Consensus | Jaccard similarity with category weights |
| Git Integration | Worktree isolation per execution |
| TUI | Bubbletea with plain-text fallback |
| Logging | slog with secret sanitization |

### 5.2 Out of Scope (v2.0+)

| Component | Rationale |
|-----------|-----------|
| SQLite Persistence | Not needed for single-user POC |
| Plugin System | Go plugins require CGO, complexity not justified |
| Web Dashboard | CLI-first approach for POC |
| Multi-Repository | Single repo focus for validation |
| Custom Consensus Algorithms | Fixed algorithm simplifies validation |

### 5.3 Explicit Constraints

1. **Single User**: No multi-tenancy or concurrent workflows
2. **Local Execution**: No remote orchestration
3. **CLI-Based Agents**: No direct API integration (uses existing CLI tools)
4. **English Only**: Prompts and outputs in English

---

## 6. Limitations and Risks

### 6.1 Known Limitations

| Limitation | Impact | Mitigation |
|------------|--------|------------|
| CLI Tool Dependencies | Requires external tools installed | Doctor command validates prerequisites |
| Rate Limits | API limits may slow execution | Token bucket rate limiter per adapter |
| Cost Accumulation | Multiple agents increase cost | Budget caps and dry-run mode |
| Parsing Fragility | CLI output formats may change | Defensive parsing with fallbacks |

### 6.2 Risk Assessment

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| CLI API Changes | High | Medium | Version-locked adapters, regular testing |
| Runaway Costs | Medium | High | Budget limits, abort on threshold |
| Hallucinations Persist | Medium | High | Human review gates, test isolation |
| State Corruption | Low | High | Atomic writes, checksums, backups |
| Secret Leakage | Low | Critical | Multi-layer regex sanitization |

---

## 7. Exit Criteria

### 7.1 Minimum Viable POC (MUST)

- [ ] End-to-end workflow execution (analyze -> plan -> execute)
- [ ] At least 2 functional CLI adapters (Claude + Gemini)
- [ ] Jaccard consensus implemented and validated
- [ ] Resume from checkpoint functional
- [ ] Complete CI/CD pipeline (lint, test, build, security)
- [ ] Test coverage >= 80%
- [ ] Professional documentation

### 7.2 Success Criteria (SHOULD)

- [ ] Empirical validation report with metrics
- [ ] Single-agent vs multi-agent comparison data
- [ ] Functional TUI with progress display
- [ ] Docker image published

### 7.3 Extended Goals (COULD)

- [ ] 3+ functional CLI adapters
- [ ] Metrics dashboard
- [ ] Performance benchmarks

---

## 8. References

### 8.1 Academic Sources

1. **Self-Consistency** (arXiv:2203.11171): "Self-Consistency Improves Chain of Thought Reasoning in Language Models" - Wang et al., 2022
2. **Ensemble Methods** (arXiv:2305.14739): "LLM-Blender: Ensembling Large Language Models with Pairwise Ranking and Generative Fusion" - Jiang et al., 2023
3. **Multi-Agent Debate** (arXiv:2305.14325): "Improving Factuality and Reasoning in Language Models through Multiagent Debate" - Du et al., 2023
4. **Mixture of Experts** (arXiv:2211.01910): "Switch Transformers: Scaling to Trillion Parameter Models with Simple and Efficient Sparsity" - Fedus et al., 2022

### 8.2 Technical Standards

- Go Project Layout: [github.com/golang-standards/project-layout](https://github.com/golang-standards/project-layout)
- Hexagonal Architecture: Alistair Cockburn's Ports and Adapters pattern
- Keep a Changelog: [keepachangelog.com](https://keepachangelog.com)
- Semantic Versioning: [semver.org](https://semver.org)

---

## 9. Document History

| Version | Date | Author            | Changes |
|---------|------|-------------------|---------|
| v1.0 | 2026-01-11 | hugo-lorenzo-mato | Initial POC vision document |

---

*This document defines the POC scope and success criteria for quorum-ai. Features beyond this scope are intentionally deferred to validate the core hypothesis before expanding functionality.*
