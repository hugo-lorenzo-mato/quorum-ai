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

// ========================================
// Template Renderers
// ========================================

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

// renderArbiterTemplate renders a semantic arbiter evaluation report
func renderArbiterTemplate(data ArbiterData, includeRaw bool) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Evaluación del Árbitro Semántico (Ronda %d)\n\n", data.Round))

	// Score indicator
	scoreEmoji := "🟢"
	if data.Score < 0.70 {
		scoreEmoji = "🔴"
	} else if data.Score < 0.90 {
		scoreEmoji = "🟡"
	}

	sb.WriteString(fmt.Sprintf("## %s Consenso Semántico: %.0f%%\n\n", scoreEmoji, data.Score*100))

	sb.WriteString("## Información del Árbitro\n\n")
	sb.WriteString(fmt.Sprintf("- **Agente**: %s\n", data.Agent))
	sb.WriteString(fmt.Sprintf("- **Modelo**: %s\n", data.Model))
	sb.WriteString(fmt.Sprintf("- **Ronda**: %d\n", data.Round))
	sb.WriteString(fmt.Sprintf("- **Acuerdos identificados**: %d\n", data.AgreementsCount))
	sb.WriteString(fmt.Sprintf("- **Divergencias identificadas**: %d\n\n", data.DivergencesCount))

	sb.WriteString("## Metodología de Evaluación\n\n")
	sb.WriteString("El árbitro semántico evalúa el **consenso real** entre los análisis:\n\n")
	sb.WriteString("- **Evaluación semántica**: Compara significados, no palabras exactas\n")
	sb.WriteString("- **Identificación de acuerdos**: Detecta convergencias genuinas entre agentes\n")
	sb.WriteString("- **Análisis de divergencias**: Identifica diferencias sustanciales a resolver\n")
	sb.WriteString("- **Puntuación objetiva**: Calcula un porcentaje basado en evidencia\n\n")

	sb.WriteString("---\n\n")

	if includeRaw && data.RawOutput != "" {
		sb.WriteString("## Evaluación Completa del Árbitro\n\n")
		sb.WriteString(data.RawOutput + "\n\n")
	}

	sb.WriteString("## Métricas del Árbitro\n\n")
	sb.WriteString("| Métrica | Valor |\n")
	sb.WriteString("|---------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Tokens entrada | %d |\n", data.TokensIn))
	sb.WriteString(fmt.Sprintf("| Tokens salida | %d |\n", data.TokensOut))
	sb.WriteString(fmt.Sprintf("| Costo | $%.4f USD |\n", data.CostUSD))
	sb.WriteString(fmt.Sprintf("| Duración | %s |\n", formatDuration(data.DurationMS)))

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
		if strings.EqualFold(p, target) {
			return true
		}
	}
	return false
}
