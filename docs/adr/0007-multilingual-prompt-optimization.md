# ADR-0007: Multilingual Prompt Support

## Status

Accepted

## Context

The initial POC design specified "English-only" prompts and outputs as a constraint. This restriction was based on the assumption that LLMs perform significantly better in English.

Research on multilingual LLM performance reveals:

1. **Performance gaps are nuanced**: Studies show up to 24.3% performance degradation for low-resource languages, but high-resource languages (Spanish, French, German, Portuguese, Chinese, etc.) show minimal gaps compared to English.

2. **Training corpus correlation**: LLM performance correlates strongly with the proportion of each language in the training corpus. Major world languages are well-represented in modern frontier LLMs (Claude, GPT-4, Gemini).

3. **Modern LLMs are inherently multilingual**: These models naturally detect input language and respond in the same language without explicit instructions.

## Decision

1. **Accept prompts in any language**: No language detection, validation, or restriction on user input.

2. **No forced translation**: The LLM responds in the same language as the user's input. Forcing English output would create poor UX for non-English speakers who wouldn't understand the responses.

3. **No special handling required**: Modern LLMs handle language detection and response language automatically. The system simply passes through prompts without modification.

4. **Trust the LLM**: The LLM will optimize and process the prompt in whatever language it receives. For high-resource languages, performance is comparable to English.

## Consequences

### Positive

- **Accessibility**: Users interact in their native language and receive responses they understand
- **Simplicity**: Zero code changes needed - no language detection, no translation, no flags
- **Natural UX**: System behaves as users expect from modern AI tools
- **Maintainability**: No additional complexity to maintain

### Negative

- **Low-resource languages**: Performance may degrade for languages with limited representation in LLM training data (this is an LLM limitation, not a quorum-ai restriction)

### Neutral

- **Mixed-language prompts**: LLMs handle code-switching naturally; no special handling needed

## Alternatives Considered

### Force English Internal Processing

Initially considered forcing English output in prompt templates to optimize LLM performance. Rejected because:

- Users would receive English responses they may not understand
- Adds unnecessary complexity
- Performance benefit is marginal for high-resource languages
- Modern LLMs handle multilingual input well

### Language Detection with Optional Translation

Considered detecting input language and offering translation. Rejected because:

- Adds library dependencies
- Increases code complexity
- Benefits don't justify the cost for a POC

## References

- [MMLU-ProX: A Multilingual Benchmark for Advanced LLM Evaluation (2025)](https://arxiv.org/abs/2503.10497) - Documents performance gaps across 29 languages
- [Language Ranker: Quantifying LLM Performance Across Languages (2024)](https://arxiv.org/abs/2404.11553) - Shows correlation between training corpus and performance
- [A Survey of Multilingual Large Language Models (2024)](https://www.sciencedirect.com/science/article/pii/S2666389924002903) - Comprehensive overview of multilingual capabilities
- [Best Practices for Open Multilingual LLM Evaluation](https://huggingface.co/blog/catherinearnett/multilingual-best-practices) - Hugging Face evaluation guidelines
