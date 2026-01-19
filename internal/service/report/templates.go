package report

import (
	"fmt"
	"strings"
	"time"
	"unicode"
)

// ========================================
// Helper functions
// ========================================

// countWords counts words in a string
func countWords(s string) int {
	return len(strings.Fields(s))
}

// sanitizeFilename removes or replaces characters unsuitable for filenames
func sanitizeFilename(s string) string {
	var result strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' {
			result.WriteRune(r)
		} else if r == ' ' || r == '/' || r == ':' {
			result.WriteRune('-')
		}
	}
	return strings.ToLower(result.String())
}

// formatDuration formats a duration in a human-readable way
func formatDuration(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	if d < time.Second {
		return fmt.Sprintf("%dms", ms)
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
}

// renderList renders a list of strings as markdown
func renderList(items []string) string {
	if len(items) == 0 {
		return "_No items_\n"
	}
	var sb strings.Builder
	for i, item := range items {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, item))
	}
	return sb.String()
}

// ========================================
// Template Renderers
// ========================================

// renderOptimizedPromptTemplate renders the optimized prompt template
func renderOptimizedPromptTemplate(original, optimized string, metrics PromptMetrics) string {
	var sb strings.Builder

	sb.WriteString("# Prompt Optimizado\n\n")

	sb.WriteString("## Prompt Original\n\n")
	sb.WriteString("> " + strings.ReplaceAll(original, "\n", "\n> ") + "\n\n")

	sb.WriteString("## Prompt Optimizado\n\n")
	sb.WriteString(optimized + "\n\n")

	sb.WriteString("## Mejoras Aplicadas\n\n")
	sb.WriteString("| Aspecto | Original | Optimizado |\n")
	sb.WriteString("|---------|----------|------------|\n")
	sb.WriteString(fmt.Sprintf("| Caracteres | %d | %d |\n", metrics.OriginalCharCount, metrics.OptimizedCharCount))
	sb.WriteString(fmt.Sprintf("| Ratio de mejora | - | %.2fx |\n", metrics.ImprovementRatio))

	sb.WriteString("\n## Métricas\n\n")
	sb.WriteString(fmt.Sprintf("- **Agente**: %s\n", metrics.OptimizerAgent))
	sb.WriteString(fmt.Sprintf("- **Modelo**: %s\n", metrics.OptimizerModel))
	sb.WriteString(fmt.Sprintf("- **Tokens usados**: %d\n", metrics.TokensUsed))
	sb.WriteString(fmt.Sprintf("- **Costo**: $%.4f USD\n", metrics.CostUSD))
	sb.WriteString(fmt.Sprintf("- **Duración**: %s\n", formatDuration(metrics.DurationMS)))

	return sb.String()
}

// renderAnalysisTemplate renders an analysis report
func renderAnalysisTemplate(data AnalysisData, version string, includeRaw bool) string {
	var sb strings.Builder

	title := fmt.Sprintf("# Análisis %s: %s (%s)\n\n", strings.ToUpper(version), data.AgentName, data.Model)
	sb.WriteString(title)

	// V1 Analysis methodology description
	sb.WriteString("## Metodología de Análisis\n\n")
	sb.WriteString("Este análisis ha sido generado siguiendo los principios de **análisis profundo y exhaustivo**:\n\n")
	sb.WriteString("- **Basado en código**: Revisión directa del código fuente y estructura del proyecto\n")
	sb.WriteString("- **Buenas prácticas**: Evaluación contra patrones de diseño y prácticas reconocidas de la industria\n")
	sb.WriteString("- **Documentación oficial**: Verificación con la documentación oficial de frameworks y librerías\n")
	sb.WriteString("- **Versiones y estándares**: Ajustado a las versiones específicas de los lenguajes y herramientas empleadas\n")
	sb.WriteString("- **Sin limitaciones**: Análisis completo sin restricciones de profundidad o alcance\n\n")

	sb.WriteString("---\n\n")

	sb.WriteString("## Claims (Afirmaciones Fundamentadas)\n\n")
	sb.WriteString("_Afirmaciones técnicas respaldadas por evidencia del código y documentación oficial._\n\n")
	sb.WriteString(renderList(data.Claims))

	sb.WriteString("\n## Risks (Riesgos Identificados)\n\n")
	sb.WriteString("_Riesgos técnicos, de seguridad, rendimiento y mantenibilidad detectados._\n\n")
	sb.WriteString(renderList(data.Risks))

	sb.WriteString("\n## Recommendations (Recomendaciones Accionables)\n\n")
	sb.WriteString("_Recomendaciones específicas alineadas con convenciones y estándares del ecosistema._\n\n")
	sb.WriteString(renderList(data.Recommendations))

	sb.WriteString("\n## Métricas del Análisis\n\n")
	sb.WriteString("| Métrica | Valor |\n")
	sb.WriteString("|---------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Tokens entrada | %d |\n", data.TokensIn))
	sb.WriteString(fmt.Sprintf("| Tokens salida | %d |\n", data.TokensOut))
	sb.WriteString(fmt.Sprintf("| Costo | $%.4f USD |\n", data.CostUSD))
	sb.WriteString(fmt.Sprintf("| Duración | %s |\n", formatDuration(data.DurationMS)))

	if includeRaw && data.RawOutput != "" {
		sb.WriteString("\n## Raw Output (JSON)\n\n")
		sb.WriteString("```json\n")
		sb.WriteString(data.RawOutput)
		sb.WriteString("\n```\n")
	}

	return sb.String()
}

// renderCritiqueTemplate renders a V2 critique report
func renderCritiqueTemplate(data CritiqueData, includeRaw bool) string {
	var sb strings.Builder

	title := fmt.Sprintf("# Crítica V2: %s critica a %s\n\n", data.CriticAgent, data.TargetAgent)
	sb.WriteString(title)

	sb.WriteString(fmt.Sprintf("**Agente Crítico**: %s (%s)\n", data.CriticAgent, data.CriticModel))
	sb.WriteString(fmt.Sprintf("**Análisis Criticado**: %s\n\n", data.TargetAgent))

	// V2 Critique methodology description
	sb.WriteString("## Metodología de Crítica V2\n\n")
	sb.WriteString("Esta crítica representa un **análisis ultra-crítico y exhaustivo** que:\n\n")
	sb.WriteString("- **Evalúa TODOS los análisis V1**: Contrasta las conclusiones de todos los agentes participantes\n")
	sb.WriteString("- **Identifica inconsistencias**: Detecta contradicciones y gaps entre diferentes análisis\n")
	sb.WriteString("- **Cuestiona fundamentación**: Verifica que cada afirmación esté respaldada por evidencia concreta\n")
	sb.WriteString("- **Valida contra documentación**: Comprueba las recomendaciones contra documentación oficial actualizada\n")
	sb.WriteString("- **Evalúa completitud**: Identifica aspectos no cubiertos en los análisis originales\n")
	sb.WriteString("- **Perspectiva adversarial**: Busca activamente debilidades y puntos ciegos\n\n")

	sb.WriteString("---\n\n")

	sb.WriteString("## Puntos de Acuerdo Validados\n\n")
	sb.WriteString("_Conclusiones del análisis original que se consideran correctas y bien fundamentadas._\n\n")
	sb.WriteString(renderList(data.Agreements))

	sb.WriteString("\n## Puntos de Desacuerdo / Correcciones\n\n")
	sb.WriteString("_Aspectos donde el análisis original es incorrecto, incompleto o mal fundamentado._\n\n")
	sb.WriteString(renderList(data.Disagreements))

	sb.WriteString("\n## Riesgos Adicionales No Identificados\n\n")
	sb.WriteString("_Riesgos que el análisis original pasó por alto o subestimó._\n\n")
	sb.WriteString(renderList(data.AdditionalRisks))

	sb.WriteString("\n## Métricas de la Crítica\n\n")
	sb.WriteString("| Métrica | Valor |\n")
	sb.WriteString("|---------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Tokens entrada | %d |\n", data.TokensIn))
	sb.WriteString(fmt.Sprintf("| Tokens salida | %d |\n", data.TokensOut))
	sb.WriteString(fmt.Sprintf("| Costo | $%.4f USD |\n", data.CostUSD))
	sb.WriteString(fmt.Sprintf("| Duración | %s |\n", formatDuration(data.DurationMS)))

	if includeRaw && data.RawOutput != "" {
		sb.WriteString("\n## Raw Output (JSON)\n\n")
		sb.WriteString("```json\n")
		sb.WriteString(data.RawOutput)
		sb.WriteString("\n```\n")
	}

	return sb.String()
}

// renderReconciliationTemplate renders a V3 reconciliation report
func renderReconciliationTemplate(data ReconciliationData, includeRaw bool) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Reconciliación V3: %s\n\n", data.Agent))
	sb.WriteString(fmt.Sprintf("**Agente Reconciliador**: %s (%s)\n\n", data.Agent, data.Model))

	// V3 Reconciliation methodology description
	sb.WriteString("## Metodología de Reconciliación V3\n\n")
	sb.WriteString("La reconciliación V3 es el **arbitraje final** del proceso de análisis multi-agente:\n\n")
	sb.WriteString("- **Síntesis de divergencias**: Resuelve las diferencias identificadas entre análisis V1 y críticas V2\n")
	sb.WriteString("- **Decisión fundamentada**: Cada resolución está respaldada por evidencia técnica objetiva\n")
	sb.WriteString("- **Priorización de riesgos**: Ordena y consolida riesgos según impacto y probabilidad\n")
	sb.WriteString("- **Recomendaciones unificadas**: Genera un conjunto coherente de acciones a tomar\n")
	sb.WriteString("- **Documentación trazable**: Mantiene referencias a los análisis originales\n\n")

	sb.WriteString("---\n\n")

	sb.WriteString("## Síntesis Final\n\n")
	sb.WriteString(data.RawOutput + "\n\n")

	sb.WriteString("## Métricas de Reconciliación\n\n")
	sb.WriteString("| Métrica | Valor |\n")
	sb.WriteString("|---------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Tokens entrada | %d |\n", data.TokensIn))
	sb.WriteString(fmt.Sprintf("| Tokens salida | %d |\n", data.TokensOut))
	sb.WriteString(fmt.Sprintf("| Costo | $%.4f USD |\n", data.CostUSD))
	sb.WriteString(fmt.Sprintf("| Duración | %s |\n", formatDuration(data.DurationMS)))

	return sb.String()
}

// renderConsensusTemplate renders a consensus report
func renderConsensusTemplate(data ConsensusData, afterPhase string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Reporte de Consenso (después de %s)\n\n", afterPhase))

	// Status indicator
	status := "✅ Aprobado"
	if data.NeedsHumanReview {
		status = "🚨 Requiere Revisión Humana"
	} else if data.NeedsEscalation {
		status = "⚠️ Requiere Escalación"
	}
	sb.WriteString(fmt.Sprintf("**Estado**: %s\n\n", status))

	sb.WriteString("## Métricas Globales\n\n")
	sb.WriteString("| Métrica | Valor |\n")
	sb.WriteString("|---------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Score Global | %.2f%% |\n", data.Score*100))
	sb.WriteString(fmt.Sprintf("| Threshold | %.2f%% |\n", data.Threshold*100))
	sb.WriteString(fmt.Sprintf("| Agentes | %d |\n", data.AgentsCount))
	sb.WriteString(fmt.Sprintf("| Necesita Escalación | %t |\n", data.NeedsEscalation))
	sb.WriteString(fmt.Sprintf("| Necesita Revisión Humana | %t |\n", data.NeedsHumanReview))

	sb.WriteString("\n## Scores por Categoría\n\n")
	sb.WriteString("| Categoría | Score |\n")
	sb.WriteString("|-----------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Claims | %.2f%% |\n", data.ClaimsScore*100))
	sb.WriteString(fmt.Sprintf("| Risks | %.2f%% |\n", data.RisksScore*100))
	sb.WriteString(fmt.Sprintf("| Recommendations | %.2f%% |\n", data.RecommendationsScore*100))

	if len(data.Divergences) > 0 {
		sb.WriteString("\n## Divergencias Detectadas\n\n")
		for i, div := range data.Divergences {
			sb.WriteString(fmt.Sprintf("### Divergencia %d\n", i+1))
			sb.WriteString(fmt.Sprintf("- **Tipo**: %s\n", div.Type))
			sb.WriteString(fmt.Sprintf("- **Agentes**: %s vs %s\n", div.Agent1, div.Agent2))
			sb.WriteString(fmt.Sprintf("- **Descripción**: %s\n\n", div.Description))
		}
	}

	return sb.String()
}

// renderConsolidationTemplate renders the consolidated analysis
func renderConsolidationTemplate(data ConsolidationData) string {
	var sb strings.Builder

	sb.WriteString("# Análisis Consolidado Final\n\n")

	synthesizedStr := "Síntesis LLM Inteligente"
	if !data.Synthesized {
		synthesizedStr = "Concatenación (fallback)"
	}

	sb.WriteString("## Información del Proceso\n\n")
	sb.WriteString(fmt.Sprintf("- **Método de consolidación**: %s\n", synthesizedStr))
	sb.WriteString(fmt.Sprintf("- **Agente consolidador**: %s (%s)\n", data.Agent, data.Model))
	sb.WriteString(fmt.Sprintf("- **Análisis procesados**: %d\n", data.AnalysesCount))
	sb.WriteString(fmt.Sprintf("- **Score de consenso**: %.2f%%\n\n", data.ConsensusScore*100))

	sb.WriteString("## Proceso de Consolidación\n\n")
	sb.WriteString("Este documento representa la **síntesis final** del análisis multi-agente:\n\n")
	sb.WriteString("1. **Análisis V1 independientes**: Cada agente realizó un análisis profundo y exhaustivo\n")
	sb.WriteString("2. **Críticas V2 cruzadas**: Los agentes evaluaron críticamente los análisis de otros\n")
	sb.WriteString("3. **Reconciliación V3** (si aplica): Se resolvieron divergencias significativas\n")
	sb.WriteString("4. **Consolidación final**: Se integran todas las perspectivas en un documento unificado\n\n")

	sb.WriteString("---\n\n")

	sb.WriteString("## Análisis Consolidado\n\n")
	sb.WriteString(data.Content + "\n\n")

	sb.WriteString("---\n\n")

	sb.WriteString("## Métricas Totales del Proceso de Análisis\n\n")
	sb.WriteString("| Métrica | Valor |\n")
	sb.WriteString("|---------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Tokens entrada (total) | %d |\n", data.TotalTokensIn))
	sb.WriteString(fmt.Sprintf("| Tokens salida (total) | %d |\n", data.TotalTokensOut))
	sb.WriteString(fmt.Sprintf("| Costo total análisis | $%.4f USD |\n", data.TotalCostUSD))
	sb.WriteString(fmt.Sprintf("| Duración consolidación | %s |\n", formatDuration(data.TotalDurationMS)))

	return sb.String()
}

// renderPlanTemplate renders a plan report
func renderPlanTemplate(data PlanData) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Plan: %s (%s)\n\n", data.Agent, data.Model))

	sb.WriteString("## Contenido del Plan\n\n")
	sb.WriteString(data.Content + "\n\n")

	sb.WriteString("## Métricas\n\n")
	sb.WriteString("| Métrica | Valor |\n")
	sb.WriteString("|---------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Tokens entrada | %d |\n", data.TokensIn))
	sb.WriteString(fmt.Sprintf("| Tokens salida | %d |\n", data.TokensOut))
	sb.WriteString(fmt.Sprintf("| Costo | $%.4f USD |\n", data.CostUSD))
	sb.WriteString(fmt.Sprintf("| Duración | %s |\n", formatDuration(data.DurationMS)))

	return sb.String()
}

// renderTaskResultTemplate renders a task result
func renderTaskResultTemplate(data TaskResultData) string {
	var sb strings.Builder

	statusEmoji := "✅"
	if data.Status == "failed" {
		statusEmoji = "❌"
	} else if data.Status == "skipped" {
		statusEmoji = "⏭️"
	}

	sb.WriteString(fmt.Sprintf("# %s Tarea: %s\n\n", statusEmoji, data.TaskName))
	sb.WriteString(fmt.Sprintf("**ID**: %s\n", data.TaskID))
	sb.WriteString(fmt.Sprintf("**Estado**: %s\n", data.Status))
	sb.WriteString(fmt.Sprintf("**Agente**: %s (%s)\n\n", data.Agent, data.Model))

	if data.Status == "completed" && data.Output != "" {
		sb.WriteString("## Resultado\n\n")
		sb.WriteString(data.Output + "\n\n")
	}

	if data.Status == "failed" && data.Error != "" {
		sb.WriteString("## Error\n\n")
		sb.WriteString("```\n")
		sb.WriteString(data.Error)
		sb.WriteString("\n```\n\n")
	}

	sb.WriteString("## Métricas\n\n")
	sb.WriteString("| Métrica | Valor |\n")
	sb.WriteString("|---------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Tokens entrada | %d |\n", data.TokensIn))
	sb.WriteString(fmt.Sprintf("| Tokens salida | %d |\n", data.TokensOut))
	sb.WriteString(fmt.Sprintf("| Costo | $%.4f USD |\n", data.CostUSD))
	sb.WriteString(fmt.Sprintf("| Duración | %s |\n", formatDuration(data.DurationMS)))

	return sb.String()
}

// renderExecutionSummaryTemplate renders the execution summary
func renderExecutionSummaryTemplate(data ExecutionSummaryData) string {
	var sb strings.Builder

	sb.WriteString("# Resumen de Ejecución\n\n")

	sb.WriteString("## Estado General\n\n")
	sb.WriteString("| Métrica | Valor |\n")
	sb.WriteString("|---------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Total de tareas | %d |\n", data.TotalTasks))
	sb.WriteString(fmt.Sprintf("| Completadas | %d ✅ |\n", data.CompletedTasks))
	sb.WriteString(fmt.Sprintf("| Fallidas | %d ❌ |\n", data.FailedTasks))
	sb.WriteString(fmt.Sprintf("| Omitidas | %d ⏭️ |\n", data.SkippedTasks))

	sb.WriteString("\n## Métricas Totales\n\n")
	sb.WriteString("| Métrica | Valor |\n")
	sb.WriteString("|---------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Tokens entrada | %d |\n", data.TotalTokensIn))
	sb.WriteString(fmt.Sprintf("| Tokens salida | %d |\n", data.TotalTokensOut))
	sb.WriteString(fmt.Sprintf("| Costo total | $%.4f USD |\n", data.TotalCostUSD))
	sb.WriteString(fmt.Sprintf("| Duración total | %s |\n", formatDuration(data.TotalDurationMS)))

	if len(data.Tasks) > 0 {
		sb.WriteString("\n## Detalle de Tareas\n\n")
		sb.WriteString("| ID | Nombre | Estado | Costo | Duración |\n")
		sb.WriteString("|-------|--------|--------|-------|----------|\n")
		for _, task := range data.Tasks {
			statusEmoji := "✅"
			if task.Status == "failed" {
				statusEmoji = "❌"
			} else if task.Status == "skipped" {
				statusEmoji = "⏭️"
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | $%.4f | %s |\n",
				task.TaskID, task.TaskName, statusEmoji, task.CostUSD, formatDuration(task.DurationMS)))
		}
	}

	return sb.String()
}

// renderMetadataTemplate renders the workflow metadata
func renderMetadataTemplate(data WorkflowMetadata) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Workflow Execution: %s\n\n", data.WorkflowID))

	statusEmoji := "✅"
	if data.Status == "failed" {
		statusEmoji = "❌"
	} else if data.Status == "running" {
		statusEmoji = "🔄"
	}

	sb.WriteString("## Información General\n\n")
	sb.WriteString("| Campo | Valor |\n")
	sb.WriteString("|-------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Workflow ID | %s |\n", data.WorkflowID))
	sb.WriteString(fmt.Sprintf("| Iniciado | %s |\n", data.StartedAt.Format("2006-01-02 15:04:05 MST")))
	if !data.CompletedAt.IsZero() {
		sb.WriteString(fmt.Sprintf("| Completado | %s |\n", data.CompletedAt.Format("2006-01-02 15:04:05 MST")))
		duration := data.CompletedAt.Sub(data.StartedAt)
		sb.WriteString(fmt.Sprintf("| Duración | %s |\n", duration.Round(time.Second)))
	}
	sb.WriteString(fmt.Sprintf("| Estado | %s %s |\n", statusEmoji, data.Status))

	sb.WriteString("\n## Fases Ejecutadas\n\n")
	for _, phase := range data.PhasesExecuted {
		sb.WriteString(fmt.Sprintf("- %s\n", phase))
	}

	sb.WriteString("\n## Métricas Totales\n\n")
	sb.WriteString(fmt.Sprintf("- **Costo Total**: $%.4f USD\n", data.TotalCostUSD))
	sb.WriteString(fmt.Sprintf("- **Tokens Entrada**: %d\n", data.TotalTokensIn))
	sb.WriteString(fmt.Sprintf("- **Tokens Salida**: %d\n", data.TotalTokensOut))
	sb.WriteString(fmt.Sprintf("- **Consenso Final**: %.2f%%\n", data.ConsensusScore*100))

	if len(data.AgentsUsed) > 0 {
		sb.WriteString("\n## Agentes Utilizados\n\n")
		for _, agent := range data.AgentsUsed {
			sb.WriteString(fmt.Sprintf("- %s\n", agent))
		}
	}

	return sb.String()
}

// renderWorkflowSummaryTemplate renders the final workflow summary
func renderWorkflowSummaryTemplate(data WorkflowMetadata) string {
	var sb strings.Builder

	sb.WriteString("# Resumen del Workflow\n\n")

	statusEmoji := "✅"
	statusText := "Completado exitosamente"
	if data.Status == "failed" {
		statusEmoji = "❌"
		statusText = "Fallido"
	}

	sb.WriteString(fmt.Sprintf("## %s %s\n\n", statusEmoji, statusText))

	sb.WriteString(fmt.Sprintf("**Workflow ID**: `%s`\n\n", data.WorkflowID))

	if !data.CompletedAt.IsZero() {
		duration := data.CompletedAt.Sub(data.StartedAt)
		sb.WriteString(fmt.Sprintf("**Duración total**: %s\n\n", duration.Round(time.Second)))
	}

	sb.WriteString("## Resumen de Fases\n\n")
	sb.WriteString("| Fase | Estado |\n")
	sb.WriteString("|------|--------|\n")
	for _, phase := range data.PhasesExecuted {
		sb.WriteString(fmt.Sprintf("| %s | ✅ |\n", phase))
	}

	sb.WriteString("\n## Métricas Finales\n\n")
	sb.WriteString("| Métrica | Valor |\n")
	sb.WriteString("|---------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Costo Total | $%.4f USD |\n", data.TotalCostUSD))
	sb.WriteString(fmt.Sprintf("| Tokens Totales | %d entrada / %d salida |\n", data.TotalTokensIn, data.TotalTokensOut))
	sb.WriteString(fmt.Sprintf("| Consenso | %.2f%% |\n", data.ConsensusScore*100))

	sb.WriteString("\n## Archivos Generados\n\n")
	sb.WriteString("- `metadata.md` - Metadatos de la ejecución\n")
	sb.WriteString("- `analyze-phase/` - Análisis y consenso\n")
	if containsPhase(data.PhasesExecuted, "plan") {
		sb.WriteString("- `plan-phase/` - Planes generados\n")
	}
	if containsPhase(data.PhasesExecuted, "execute") {
		sb.WriteString("- `execute-phase/` - Resultados de ejecución\n")
	}

	return sb.String()
}

func containsPhase(phases []string, target string) bool {
	for _, p := range phases {
		if strings.ToLower(p) == strings.ToLower(target) {
			return true
		}
	}
	return false
}
