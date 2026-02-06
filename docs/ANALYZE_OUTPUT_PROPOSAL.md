# Propuesta: Exportación de Análisis a Markdown

## 1. Resumen Ejecutivo

Implementar la exportación automática de todos los análisis generados durante el workflow a archivos Markdown estructurados, permitiendo trazabilidad completa y revisión humana de cada paso del proceso.

## 2. Estructura de Directorios Propuesta

Cada ejecución del workflow crea su propia carpeta con timestamp, conteniendo todas las fases:

```
.quorum-output/                                          # Raíz de outputs de Quorum
├── 20240115-143000-wf-abc123/                          # Ejecución 1
│   ├── metadata.md                                      # Metadatos de la ejecución
│   ├── analyze-phase/                                   # Fase de análisis
│   │   ├── 00-original-prompt.md                        # Prompt original del usuario
│   │   ├── 01-optimized-prompt.md                       # Prompt optimizado (si aplica)
│   │   ├── v1/                                          # Análisis inicial (paralelo)
│   │   │   ├── claude-sonnet-4.md
│   │   │   ├── gemini-2.5-pro.md
│   │   │   └── codex-gpt4.md
│   │   ├── v2/                                          # Críticas cruzadas (si aplica)
│   │   │   ├── gemini-critiques-claude.md
│   │   │   └── claude-critiques-gemini.md
│   │   ├── v3/                                          # Reconciliación (si aplica)
│   │   │   └── claude-reconciliation.md
│   │   ├── consensus/                                   # Reportes de consenso
│   │   │   ├── after-v1.md
│   │   │   ├── after-v2.md
│   │   │   └── after-v3.md
│   │   └── consolidated.md                              # Análisis consolidado final
│   ├── plan-phase/                                      # Fase de planificación
│   │   ├── v1/
│   │   │   └── claude-plan.md
│   │   ├── consensus/
│   │   │   └── plan-review.md
│   │   └── final-plan.md                                # Plan aprobado
│   ├── execute-phase/                                   # Fase de ejecución
│   │   ├── tasks/                                       # Tareas individuales
│   │   │   ├── task-001-setup-auth.md
│   │   │   ├── task-002-create-models.md
│   │   │   └── task-003-implement-api.md
│   │   └── execution-summary.md                         # Resumen de ejecución
│   └── workflow-summary.md                              # Resumen completo del workflow
│
├── 20240116-091500-wf-def456/                          # Ejecución 2
│   └── ...
│
└── 20240117-102030-wf-ghi789/                          # Ejecución 3
    └── ...
```

### 2.1 Ventajas de Esta Estructura

1. **Aislamiento**: Cada ejecución está completamente aislada
2. **Histórico**: Fácil comparar diferentes ejecuciones del mismo proyecto
3. **Limpieza**: Borrar una ejecución = borrar una carpeta
4. **Trazabilidad**: El timestamp + workflow_id identifica unívocamente cada run
5. **Extensibilidad**: Fácil agregar nuevas fases sin conflictos

## 3. Convención de Nombres de Archivos

### 3.1 Carpeta de Ejecución
```
{YYYYMMDD}-{HHMMSS}-{workflow_id}/
```
Ejemplo: `20240115-143000-wf-abc123/`

### 3.2 Archivos dentro de analyze-phase/

| Archivo | Descripción |
|---------|-------------|
| `00-original-prompt.md` | Prompt original del usuario |
| `01-optimized-prompt.md` | Prompt tras optimización (si se ejecutó optimize) |
| `consolidated.md` | Análisis consolidado final |

### 3.3 Análisis V1/V2/V3
```
{agent}-{model}.md                    # V1: claude-sonnet-4.md
{agent}-critiques-{target}.md         # V2: gemini-critiques-claude.md
{agent}-reconciliation.md             # V3: claude-reconciliation.md
```

### 3.4 Reportes de Consenso
```
after-v1.md    # Consenso tras análisis V1
after-v2.md    # Consenso tras críticas V2
after-v3.md    # Consenso tras reconciliación V3
```

### 3.5 Archivos de Plan/Execute (futuro)
```
plan-phase/final-plan.md
execute-phase/tasks/task-{NNN}-{slug}.md
execute-phase/execution-summary.md
```

## 4. Formato de Contenido Markdown

### 4.0 Metadata de Ejecución (metadata.md)

```markdown
---
workflow_id: wf-abc123
started_at: 2024-01-15T14:30:00Z
completed_at: 2024-01-15T14:35:22Z
status: completed
---

# Workflow Execution: wf-abc123

## Información General
| Campo | Valor |
|-------|-------|
| Workflow ID | wf-abc123 |
| Iniciado | 2024-01-15 14:30:00 UTC |
| Completado | 2024-01-15 14:35:22 UTC |
| Duración | 5m 22s |
| Estado | ✅ Completado |

## Fases Ejecutadas
| Fase | Estado | Duración | Costo |
|------|--------|----------|-------|
| Optimize | ✅ | 2.1s | $0.01 |
| Analyze | ✅ | 45.3s | $0.15 |
| Plan | ✅ | 12.8s | $0.05 |
| Execute | ✅ | 4m 2s | $0.32 |

## Métricas Totales
- **Costo Total**: $0.53 USD
- **Tokens Entrada**: 45,678
- **Tokens Salida**: 12,345
- **Consenso Final**: 0.87

## Agentes Utilizados
- Claude (claude-sonnet-4-20250514)
- Gemini (gemini-2.5-pro)
- Codex (gpt-4-turbo)
```

### 4.0.1 Prompt Original (00-original-prompt.md)

```markdown
---
type: original_prompt
timestamp: 2024-01-15T14:30:00Z
workflow_id: wf-abc123
char_count: 1234
word_count: 256
---

# Prompt Original

## Contenido

Implementar un sistema de autenticación completo para la aplicación web que incluya:

1. Login con email/contraseña
2. Registro de nuevos usuarios
3. Recuperación de contraseña
4. Autenticación con OAuth (Google, GitHub)
5. Protección de rutas privadas
6. Gestión de sesiones con JWT

El sistema debe ser seguro, escalable y seguir las mejores prácticas de la industria.

## Metadata
- **Caracteres**: 1,234
- **Palabras**: 256
- **Idioma detectado**: Español
```

### 4.0.2 Prompt Optimizado (01-optimized-prompt.md)

```markdown
---
type: optimized_prompt
timestamp: 2024-01-15T14:30:02Z
workflow_id: wf-abc123
optimizer_agent: claude
optimizer_model: claude-sonnet-4
original_char_count: 1234
optimized_char_count: 2456
improvement_ratio: 1.99
tokens_used: 892
---

# Prompt Optimizado

## Prompt Original
> Implementar un sistema de autenticación completo para la aplicación web...

## Prompt Optimizado

### Contexto del Proyecto
Se requiere implementar un sistema de autenticación robusto para una aplicación web moderna.

### Requisitos Funcionales
1. **Autenticación Local**
   - Login con email/contraseña con validación
   - Registro con verificación de email
   - Recuperación de contraseña segura con tokens temporales

2. **OAuth Integration**
   - Google OAuth 2.0
   - GitHub OAuth
   - Manejo unificado de identidades

3. **Gestión de Sesiones**
   - JWT con refresh tokens
   - Almacenamiento seguro (httpOnly cookies)
   - Revocación de tokens

4. **Seguridad**
   - Rate limiting en endpoints de auth
   - Protección CSRF
   - Logging de intentos fallidos

### Requisitos No Funcionales
- Escalabilidad horizontal
- Latencia < 200ms para operaciones de auth
- Compatibilidad con GDPR

### Stack Tecnológico Detectado
- Backend: Go
- Framework: Gin/Echo
- Base de datos: PostgreSQL

## Mejoras Aplicadas
| Aspecto | Original | Optimizado |
|---------|----------|------------|
| Especificidad | General | Detallado |
| Requisitos de seguridad | Implícitos | Explícitos |
| Métricas de rendimiento | Ninguna | Definidas |
| Contexto técnico | Ausente | Incluido |
```

### 4.1 Análisis Individual (V1)

```markdown
---
type: analysis
version: v1
agent: claude
model: claude-sonnet-4-20250514
timestamp: 2024-01-15T14:30:22Z
workflow_id: wf-abc123
prompt_hash: sha256:abc123...
tokens_in: 1234
tokens_out: 5678
duration_ms: 12500
---

# Análisis V1: Claude (claude-sonnet-4)

## Prompt Original
> [Prompt del usuario aquí]

## Claims (Afirmaciones)
1. El sistema requiere autenticación basada en JWT
2. La base de datos debe soportar transacciones ACID
3. ...

## Risks (Riesgos Identificados)
1. **Alto**: Posible SQL injection en módulo de búsqueda
2. **Medio**: Falta de rate limiting en API pública
3. ...

## Recommendations (Recomendaciones)
1. Implementar validación de entrada con esquemas JSON
2. Agregar middleware de autenticación global
3. ...

## Raw Output
\```json
{
  "claims": [...],
  "risks": [...],
  "recommendations": [...]
}
\```
```

### 4.2 Crítica V2

```markdown
---
type: critique
version: v2
agent: gemini
model: gemini-2.5-pro
target_agent: claude
target_version: v1
timestamp: 2024-01-15T14:31:22Z
...
---

# Crítica V2: Gemini critica análisis de Claude

## Análisis Original Criticado
[Resumen del análisis V1 de Claude]

## Puntos de Acuerdo
1. ...

## Puntos de Desacuerdo
1. ...

## Riesgos Adicionales Identificados
1. ...

## Recomendaciones Adicionales
1. ...
```

### 4.3 Reporte de Consenso

```markdown
---
type: consensus
version: v1
timestamp: 2024-01-15T14:30:30Z
score: 0.82
threshold: 0.75
needs_escalation: false
needs_human_review: false
agents_count: 3
---

# Reporte de Consenso V1

## Métricas
| Métrica | Valor |
|---------|-------|
| Score Global | 0.82 |
| Threshold | 0.75 |
| Agentes | 3 |

## Scores por Categoría
| Categoría | Score |
|-----------|-------|
| Claims | 0.85 |
| Risks | 0.78 |
| Recommendations | 0.83 |

## Divergencias Detectadas
### Divergencia 1
- **Tipo**: Risk
- **Agentes en desacuerdo**: Claude vs Gemini
- **Descripción**: ...
```

### 4.4 Consolidación Final

```markdown
---
type: consolidated
agent: claude
model: claude-sonnet-4
timestamp: 2024-01-15T14:33:00Z
workflow_id: wf-abc123
analyses_count: 3
synthesized: true
total_tokens: 45678
consensus_score: 0.82
---

# Análisis Consolidado

## Resumen Ejecutivo
[Síntesis del consolidador]

## Claims Unificados
1. ...

## Riesgos Priorizados
| Prioridad | Riesgo | Consenso |
|-----------|--------|----------|
| Alta | ... | 3/3 agentes |
| Media | ... | 2/3 agentes |

## Plan de Recomendaciones
1. ...

## Fuentes
- Claude V1: claims 1,2,3; risks 1,2
- Gemini V1: claims 2,3,4; risks 2,3
- Codex V1: claims 1,3,5; risks 1,4

## Métricas del Proceso
| Fase | Duración | Costo |
|------|----------|-------|
| V1 Analysis | 12.5s | $0.07 |
| V2 Critique | 8.2s | $0.03 |
| Consolidation | 5.1s | $0.02 |
| **Total** | **25.8s** | **$0.12** |
```

## 5. Arquitectura de Implementación

### 5.1 Nuevo Servicio: `WorkflowReportWriter`

```go
// internal/service/report/writer.go

package report

import (
    "os"
    "path/filepath"
    "time"
)

// WorkflowReportWriter maneja la escritura de reportes para todo el workflow
type WorkflowReportWriter struct {
    baseDir      string    // ".quorum-output"
    executionDir string    // "20240115-143000-wf-abc123"
    workflowID   string
    startTime    time.Time
    useUTC       bool
    includeRaw   bool
}

// ReportConfig configura el writer
type ReportConfig struct {
    BaseDir    string // default: ".quorum-output"
    UseUTC     bool   // default: true
    IncludeRaw bool   // include raw JSON output
}

// NewWorkflowReportWriter crea un writer para una nueva ejecución
func NewWorkflowReportWriter(cfg ReportConfig, workflowID string) (*WorkflowReportWriter, error) {
    now := time.Now()
    if cfg.UseUTC {
        now = now.UTC()
    }

    executionDir := fmt.Sprintf("%s-%s",
        now.Format("20060102-150405"),
        workflowID)

    w := &WorkflowReportWriter{
        baseDir:      cfg.BaseDir,
        executionDir: executionDir,
        workflowID:   workflowID,
        startTime:    now,
        useUTC:       cfg.UseUTC,
        includeRaw:   cfg.IncludeRaw,
    }

    return w, w.ensureDirectories()
}

// Rutas de directorios
func (w *WorkflowReportWriter) ExecutionPath() string {
    return filepath.Join(w.baseDir, w.executionDir)
}

func (w *WorkflowReportWriter) AnalyzePhasePath() string {
    return filepath.Join(w.ExecutionPath(), "analyze-phase")
}

func (w *WorkflowReportWriter) PlanPhasePath() string {
    return filepath.Join(w.ExecutionPath(), "plan-phase")
}

func (w *WorkflowReportWriter) ExecutePhasePath() string {
    return filepath.Join(w.ExecutionPath(), "execute-phase")
}

// Métodos de escritura - Analyze Phase
func (w *WorkflowReportWriter) WriteOriginalPrompt(prompt string) error
func (w *WorkflowReportWriter) WriteOptimizedPrompt(original, optimized string, agent, model string, metrics PromptMetrics) error
func (w *WorkflowReportWriter) WriteV1Analysis(output AnalysisOutput, model string) error
func (w *WorkflowReportWriter) WriteV2Critique(output AnalysisOutput, targetAgent string, model string) error
func (w *WorkflowReportWriter) WriteV3Reconciliation(output string, agent, model string) error
func (w *WorkflowReportWriter) WriteConsensusReport(result ConsensusResult, afterPhase string) error
func (w *WorkflowReportWriter) WriteConsolidatedAnalysis(content string, agent, model string, metrics ConsolidationMetrics) error

// Métodos de escritura - Plan Phase
func (w *WorkflowReportWriter) WritePlan(plan string, agent, model string) error
func (w *WorkflowReportWriter) WriteFinalPlan(plan string) error

// Métodos de escritura - Execute Phase
func (w *WorkflowReportWriter) WriteTaskResult(taskID string, result TaskResult) error
func (w *WorkflowReportWriter) WriteExecutionSummary(summary ExecutionSummary) error

// Métodos de escritura - General
func (w *WorkflowReportWriter) WriteMetadata(metrics WorkflowMetrics) error
func (w *WorkflowReportWriter) WriteWorkflowSummary(summary WorkflowSummary) error

// Helpers
func (w *WorkflowReportWriter) ensureDirectories() error
func (w *WorkflowReportWriter) writeMarkdownFile(path string, frontmatter map[string]interface{}, content string) error
```

### 5.2 Integración en `Analyzer`

```go
// internal/service/workflow/analyzer.go

type Analyzer struct {
    consensus    ConsensusEvaluator
    reportWriter *report.AnalysisReportWriter  // NUEVO
}

func NewAnalyzer(consensus ConsensusEvaluator, reportWriter *report.AnalysisReportWriter) *Analyzer

// Modificaciones en Run():
func (a *Analyzer) Run(ctx context.Context, wctx *Context) error {
    // ... código existente ...

    // Después de V1 Analysis
    v1Outputs, err := a.runV1Analysis(ctx, wctx)
    // NUEVO: Escribir reportes V1
    for _, output := range v1Outputs {
        model := resolveModelUsed(wctx, output.AgentName)
        a.reportWriter.WriteV1Analysis(output, model)
    }

    // Después de evaluar consenso
    consensusResult := a.consensus.Evaluate(v1Outputs)
    // NUEVO: Escribir reporte de consenso V1
    a.reportWriter.WriteConsensusReport(consensusResult, 1)

    // ... similar para V2, V3, consolidación ...
}
```

### 5.3 Configuración

```yaml
# .quorum.yaml (o config file)
output:
  enabled: true
  base_dir: ".quorum-output"
  use_utc: true
  include_raw: true
  phases:
    analyze: true
    plan: true
    execute: true
```

## 6. Flujo de Datos Modificado

```
WORKFLOW START (quorum run / quorum analyze)
    ↓
[Create Execution Directory] → .quorum-output/20240115-143000-wf-abc123/
    ↓
[Initialize WorkflowReportWriter]
    ↓
[Write Original Prompt] → analyze-phase/00-original-prompt.md
    ↓
[Optimize Phase] (opcional)
    └─ result → WriteOptimizedPrompt() → analyze-phase/01-optimized-prompt.md
    ↓
[V1 Analysis - Paralelo]
    ├─ Claude → WriteV1Analysis() → analyze-phase/v1/claude-sonnet-4.md
    ├─ Gemini → WriteV1Analysis() → analyze-phase/v1/gemini-2.5-pro.md
    └─ Codex  → WriteV1Analysis() → analyze-phase/v1/codex-gpt4.md
    ↓
[Consensus V1]
    └─ result → WriteConsensusReport("v1") → analyze-phase/consensus/after-v1.md
    ↓
[V2 Critique] (si consenso < threshold)
    ├─ Gemini critica Claude → analyze-phase/v2/gemini-critiques-claude.md
    └─ Claude critica Gemini → analyze-phase/v2/claude-critiques-gemini.md
    ↓
[Consensus V2]
    └─ result → WriteConsensusReport("v2") → analyze-phase/consensus/after-v2.md
    ↓
[V3 Reconciliation] (si consenso V2 < threshold)
    └─ Claude reconcilia → analyze-phase/v3/claude-reconciliation.md
    ↓
[Consensus V3]
    └─ result → WriteConsensusReport("v3") → analyze-phase/consensus/after-v3.md
    ↓
[Consolidation]
    └─ final → WriteConsolidatedAnalysis() → analyze-phase/consolidated.md
    ↓
[Plan Phase] (si continúa workflow)
    └─ plan → WritePlan() → plan-phase/...
    ↓
[Execute Phase] (si continúa workflow)
    └─ tasks → WriteTaskResult() → execute-phase/tasks/...
    ↓
[Finalize]
    ├─ WriteMetadata() → metadata.md
    └─ WriteWorkflowSummary() → workflow-summary.md
```

## 8. Archivos a Crear/Modificar

### Nuevos Archivos
1. `internal/service/report/writer.go` - Servicio de escritura de reportes
2. `internal/service/report/templates.go` - Templates Markdown
3. `internal/service/report/frontmatter.go` - Generación de YAML frontmatter

### Archivos a Modificar
1. `internal/service/workflow/analyzer.go` - Integrar ReportWriter
2. `internal/service/workflow/runner.go` - Inicializar ReportWriter
3. `cmd/quorum/cmd/common.go` - Agregar a PhaseRunnerDeps
4. `internal/core/ports.go` - Agregar OutputConfig si necesario

## 9. Consideraciones Adicionales

### 9.1 Rendimiento
- Escritura de archivos en paralelo donde sea posible
- Buffer de escritura para evitar I/O excesivo
- Opción para deshabilitar en modo `--quiet`

### 9.2 Limpieza
- Comando `quorum clean --outputs` para limpiar outputs antiguos
- Retención configurable (ej: mantener últimas 10 versiones)

### 9.3 Git Integration
- Agregar `.quorum-output/` a `.gitignore` por defecto
- Opción `--commit-outputs` para incluir en commits

### 9.4 Extensibilidad
- Misma estructura aplicable a `plan-phase/` y `execute-phase/`
- Soporte futuro para exportar a otros formatos (HTML, PDF)

## 10. Ejemplo de Uso

```bash
# Primera ejecución
quorum analyze "Implementar sistema de autenticación"

# Ver outputs generados
tree .quorum-output/
# .quorum-output/
# └── 20240115-143000-wf-abc123/
#     ├── metadata.md
#     ├── analyze-phase/
#     │   ├── 00-original-prompt.md
#     │   ├── 01-optimized-prompt.md
#     │   ├── v1/
#     │   │   ├── claude-sonnet-4.md
#     │   │   ├── gemini-2.5-pro.md
#     │   │   └── codex-gpt4.md
#     │   ├── consensus/
#     │   │   └── after-v1.md
#     │   └── consolidated.md
#     └── workflow-summary.md

# Segunda ejecución → nueva carpeta con su timestamp
quorum analyze "Agregar OAuth al sistema"

tree .quorum-output/
# .quorum-output/
# ├── 20240115-143000-wf-abc123/    # Primera ejecución
# │   └── ...
# └── 20240115-160000-wf-def456/    # Segunda ejecución
#     └── ...

# Workflow completo con todas las fases
quorum run "Implementar feature completa"

tree .quorum-output/
# .quorum-output/
# └── 20240115-170000-wf-ghi789/
#     ├── metadata.md
#     ├── analyze-phase/
#     │   ├── 00-original-prompt.md
#     │   ├── 01-optimized-prompt.md
#     │   ├── v1/
#     │   ├── v2/                    # Si hubo escalación
#     │   ├── consensus/
#     │   └── consolidated.md
#     ├── plan-phase/
#     │   ├── v1/
#     │   │   └── claude-plan.md
#     │   └── final-plan.md
#     ├── execute-phase/
#     │   ├── tasks/
#     │   │   ├── task-001-setup-auth.md
#     │   │   ├── task-002-create-models.md
#     │   │   └── task-003-implement-api.md
#     │   └── execution-summary.md
#     └── workflow-summary.md
```

## 11. Próximos Pasos

1. [ ] Aprobar estructura de directorios y naming
2. [ ] Implementar `WorkflowReportWriter` base (`internal/service/report/writer.go`)
3. [ ] Implementar templates Markdown (`internal/service/report/templates.go`)
4. [ ] Implementar generación de frontmatter YAML (`internal/service/report/frontmatter.go`)
5. [ ] Integrar en `Runner` para crear writer al inicio del workflow
6. [ ] Integrar en `Optimizer` para escribir prompt original y optimizado
7. [ ] Integrar en `Analyzer` para escribir análisis V1/V2/V3 y consenso
8. [ ] Integrar en `Planner` para escribir planes
9. [ ] Integrar en `Executor` para escribir resultados de tareas
10. [ ] Agregar configuración en `.quorum.yaml`
11. [ ] Agregar flag `--no-output` para deshabilitar
12. [ ] Tests unitarios
13. [ ] Documentación de usuario
