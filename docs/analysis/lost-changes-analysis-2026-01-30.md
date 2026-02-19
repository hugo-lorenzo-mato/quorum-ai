> **Estado: RESUELTO** / **Status: RESOLVED**
>
> All issues documented in this post-mortem have been fixed (AgentEvents persistence restored in commit 897ba5a, RunnerBuilder config bug fixed in commit 00ac672). This document is preserved as a historical post-mortem reference.

# ANÁLISIS EXHAUSTIVO: Cambios Perdidos en PR Consolidado

## Metodología
- Rango temporal: 2026-01-29 22:00 - 2026-01-30 03:00 (5 horas)
- Commit crítico: c185dbc (PR #273 - Consolidated Workflow Isolation Features)
- Análisis: Verificación file-by-file de todos los cambios

---

## HALLAZGOS CRÍTICOS

### 1. ❌ AgentEvents Persistence - ELIMINADA y RESTAURADA

**Commit original:** b2aed63 (2026-01-29 23:57)
**Eliminado en:** c185dbc (2026-01-30 01:27) 
**Restaurado en:** 897ba5a (2026-01-30 03:00)

**Archivos afectados:**
- `internal/adapters/state/sqlite.go`:
  - Save(): Serialización de AgentEvents ELIMINADA
  - loadWorkflowByID(): Deserialización de AgentEvents ELIMINADA
  - INSERT statement: columna agent_events ELIMINADA
  - SELECT statement: columna agent_events ELIMINADA
  
- `internal/adapters/state/sqlite_test.go`:
  - TestSQLiteStateManager_AgentEvents: ELIMINADO completo (130 líneas)
  - TestSQLiteStateManager_AgentEventsEmpty: ELIMINADO completo (30 líneas)

**Impacto:**
Los eventos de agentes se perdían completamente al recargar la UI.
La tabla workflows tenía la columna agent_events pero nunca se escribía/leía.

**Estado actual:**
✅ Persistencia restaurada
✅ Tests restaurados

---

### 2. ⚠️ RunnerBuilder Config Bug - INTRODUCIDO

**Introducido en:** c185dbc 
**Detectado:** 2026-01-30 02:49
**Corregido en:** 00ac672

**Descripción:**
RunnerBuilder.buildRunnerConfig() comparaba punteros en vez de usar flag:
```go
if b.runnerConfig != nil && b.runnerConfig != DefaultRunnerConfig() {
    return b.runnerConfig  // SIEMPRE true - diferentes punteros
}
```

**Impacto:**
El archivo .quorum/config.yaml se cargaba pero se IGNORABA completamente.
Todos los workflows usaban hardcoded defaults (moderator.enabled=false).

**Estado actual:**
✅ Corregido con flag runnerConfigExplicit

---

## CAMBIOS VERIFICADOS COMO PRESENTES

### ✅ 92262b3 - fix(workflow,ui): fix consensus report config
- analyzer.go: Validación de Report.enabled ✓
- analyzer.go: Detección de consecutive zero scores ✓
- analyzer.go: raw_output en checkpoints ✓
- AgentActivity.jsx: startedAt/completedAt/durationMs ✓
- agentStore.js: Timestamp tracking logic ✓
- builder.go: Report config mapping ✓

### ✅ fa9cec8 - fix(api,ui): only show active workflow when running
- workflows.go: Status validation en handleGetActiveWorkflow ✓
- Dashboard.jsx: Conditional ActiveWorkflowBanner ✓

### ✅ 094df1b - chore(config): lower threshold to 0.80
- configs/default.yaml: threshold 0.80 ✓

### ✅ d54a819 - fix(chat,ui): SQL migration parsing
- chat/sqlite.go: splitStatements() comment handling ✓
- Dashboard.jsx: getWorkflowTitle() helper ✓

### ✅ 0cae3ba - feat(agents): validate CLI models
- Cambios en agent validation ✓

---

## COMMITS CONSOLIDADOS EN c185dbc

Estos PRs fueron intencionalmente consolidados:
- PR #267: RunnerBuilder Unification ✓
- PR #266: StateManager Extensions ✓
- PR #270: Runner Initialization ✓
- PR #269: Executor Workflow-Scoped Worktree ✓
- PR #268: Global Shared RateLimiter ✓
- PR #265: WorkflowWorktreeManager ✓
- PR #264: GitClient Merge Extensions ✓
- PR #263: Per-Workflow Locking ✓
- PR #262: Database Migration V6 ✓
- PR #272: Crash Recovery Service ✓

**Estado:** Todos presentes en código actual

---

## ANÁLISIS DETALLADO: Por qué se perdió agent_events

### Timeline:

1. **23:57 (b2aed63)**: Se añade agent_events a sqlite.go
   - Save() serializa AgentEvents
   - loadWorkflowByID() deserializa AgentEvents
   - Tests comprehensivos añadidos

2. **01:27 (c185dbc)**: PR consolidado mergea y SOBRESCRIBE
   - El PR fue creado desde una rama que NO incluía b2aed63
   - Al mergear a main, los cambios en Save()/loadWorkflowByID() se PERDIERON
   - La migración V5 se mantuvo pero el código que la usa desapareció
   - Los tests se eliminaron completamente

3. **02:16 (c866138)**: Mis cambios para workflow_branch
   - Partí del estado SIN agent_events
   - Añadí workflow_branch pero no restauré agent_events
   - Perpetué el bug

### Causa raíz:

El PR consolidado fue creado ANTES de que b2aed63 entrara en main.
Cuando se mergeó, Git tomó la versión del PR (sin agent_events) sobre
la versión de main (con agent_events), causando regresión.

**Lección:** PRs consolidados grandes pueden sobrescribir cambios recientes
si no se rebaseanteriormente.

---

## RECOMENDACIONES

1. ✅ **Implementado:** Restaurar agent_events (897ba5a)
2. ✅ **Implementado:** Fix RunnerBuilder config bug (00ac672)
3. ✅ **Implementado:** Restaurar tests de AgentEvents (pendiente commit)

4. **Pendiente:** Crear test de integración end-to-end que verifique:
   - Config se carga correctamente
   - AgentEvents se persisten
   - UI puede recuperar eventos al recargar

5. **Proceso:** Para futuros PRs consolidados:
   - Rebase sobre main JUSTO antes de mergear
   - Verificar diff completo contra main para detectar regresiones
   - Ejecutar test suite completo antes de push

