# Workflow Reports - Generación de Reportes Markdown

Este documento describe el sistema de generación automática de reportes Markdown que documenta cada ejecución del workflow de análisis multi-agente de Quorum AI.

## Tabla de Contenidos

- [Visión General](#visión-general)
- [Estructura de Directorios](#estructura-de-directorios)
- [Fases del Análisis](#fases-del-análisis)
  - [Análisis V1 - Análisis Profundo](#análisis-v1---análisis-profundo)
  - [Crítica V2 - Análisis Ultra-Crítico](#crítica-v2---análisis-ultra-crítico)
  - [Reconciliación V3 - Arbitraje Final](#reconciliación-v3---arbitraje-final)
  - [Análisis Consolidado](#análisis-consolidado)
- [Configuración](#configuración)
- [Formato de los Reportes](#formato-de-los-reportes)
- [Principios del Análisis](#principios-del-análisis)

## Visión General

Quorum AI genera automáticamente reportes Markdown estructurados para cada ejecución del workflow. Estos reportes documentan:

- El prompt original y optimizado
- Los análisis independientes de cada agente (V1)
- Las críticas cruzadas entre agentes (V2)
- Las reconciliaciones de divergencias (V3)
- Los reportes de consenso
- El análisis consolidado final
- Los resultados de planificación y ejecución

## Estructura de Directorios

```
.quorum-output/
└── {timestamp}-{workflow-id}/           # Ej: 20240115-143052-wf-1705329052-1
    ├── metadata.md                      # Metadatos de la ejecución
    ├── workflow-summary.md              # Resumen final del workflow
    │
    ├── analyze-phase/                   # Fase de análisis
    │   ├── 00-original-prompt.md        # Prompt original del usuario
    │   ├── 01-optimized-prompt.md       # Prompt tras optimización
    │   │
    │   ├── v1/                          # Análisis V1 (independientes)
    │   │   ├── claude-claude-3-5-sonnet.md
    │   │   ├── gemini-gemini-2.0-flash.md
    │   │   └── ...
    │   │
    │   ├── v2/                          # Críticas V2 (cruzadas)
    │   │   ├── claude-critiques-gemini.md
    │   │   ├── gemini-critiques-claude.md
    │   │   └── ...
    │   │
    │   ├── v3/                          # Reconciliación V3
    │   │   └── claude-reconciliation.md
    │   │
    │   ├── consensus/                   # Reportes de consenso
    │   │   ├── after-v1.md
    │   │   └── after-v2.md
    │   │
    │   └── consolidated.md              # Análisis consolidado final
    │
    ├── plan-phase/                      # Fase de planificación
    │   ├── v1/
    │   │   └── {agent}-plan.md
    │   ├── consensus/
    │   └── final-plan.md
    │
    └── execute-phase/                   # Fase de ejecución
        ├── tasks/
        │   ├── task-001-nombre.md
        │   └── ...
        └── execution-summary.md
```

## Fases del Análisis

### Análisis V1 - Análisis Profundo

El análisis V1 es el análisis inicial realizado **independientemente por cada agente**. Se caracteriza por:

#### Principios Fundamentales

1. **Basado en código**: Revisión directa del código fuente y estructura del proyecto
2. **Buenas prácticas**: Evaluación contra patrones de diseño y prácticas reconocidas de la industria
3. **Documentación oficial**: Verificación con la documentación oficial de frameworks y librerías utilizadas
4. **Versiones y estándares**: Ajustado a las versiones específicas de los lenguajes y herramientas empleadas
5. **Sin limitaciones**: Análisis completo sin restricciones de profundidad o alcance

#### Contenido del Análisis V1

Cada análisis V1 incluye:

- **Claims (Afirmaciones Fundamentadas)**: Afirmaciones técnicas respaldadas por evidencia del código y documentación oficial
- **Risks (Riesgos Identificados)**: Riesgos técnicos, de seguridad, rendimiento y mantenibilidad detectados
- **Recommendations (Recomendaciones Accionables)**: Recomendaciones específicas alineadas con convenciones y estándares del ecosistema

### Crítica V2 - Análisis Ultra-Crítico

La fase V2 representa un **análisis ultra-crítico y exhaustivo** donde cada agente evalúa críticamente el análisis de otro agente.

#### Características de la Crítica V2

1. **Evalúa TODOS los análisis V1**: Contrasta las conclusiones de todos los agentes participantes
2. **Identifica inconsistencias**: Detecta contradicciones y gaps entre diferentes análisis
3. **Cuestiona fundamentación**: Verifica que cada afirmación esté respaldada por evidencia concreta
4. **Valida contra documentación**: Comprueba las recomendaciones contra documentación oficial actualizada
5. **Evalúa completitud**: Identifica aspectos no cubiertos en los análisis originales
6. **Perspectiva adversarial**: Busca activamente debilidades y puntos ciegos

#### Contenido de la Crítica V2

- **Puntos de Acuerdo Validados**: Conclusiones del análisis original que se consideran correctas y bien fundamentadas
- **Puntos de Desacuerdo / Correcciones**: Aspectos donde el análisis original es incorrecto, incompleto o mal fundamentado
- **Riesgos Adicionales No Identificados**: Riesgos que el análisis original pasó por alto o subestimó

### Reconciliación V3 - Arbitraje Final

La reconciliación V3 se activa cuando el consenso entre agentes está por debajo del umbral configurado. Es el **arbitraje final** del proceso.

#### Proceso de Reconciliación

1. **Síntesis de divergencias**: Resuelve las diferencias identificadas entre análisis V1 y críticas V2
2. **Decisión fundamentada**: Cada resolución está respaldada por evidencia técnica objetiva
3. **Priorización de riesgos**: Ordena y consolida riesgos según impacto y probabilidad
4. **Recomendaciones unificadas**: Genera un conjunto coherente de acciones a tomar
5. **Documentación trazable**: Mantiene referencias a los análisis originales

### Análisis Consolidado

El documento final que integra todas las perspectivas:

1. **Análisis V1 independientes**: Cada agente realizó un análisis profundo y exhaustivo
2. **Críticas V2 cruzadas**: Los agentes evaluaron críticamente los análisis de otros
3. **Reconciliación V3** (si aplica): Se resolvieron divergencias significativas
4. **Consolidación final**: Se integran todas las perspectivas en un documento unificado

## Configuración

La generación de reportes se configura en el archivo de configuración de Quorum:

```yaml
report:
  # Directorio base para los reportes (relativo al directorio del proyecto)
  base_dir: ".quorum-output"

  # Usar timestamps en UTC (recomendado para equipos distribuidos)
  use_utc: true

  # Incluir el JSON raw de las respuestas de los agentes
  include_raw: true

  # Habilitar/deshabilitar generación de reportes
  enabled: true
```

### Opciones de Configuración

| Opción | Tipo | Default | Descripción |
|--------|------|---------|-------------|
| `base_dir` | string | `.quorum-output` | Directorio donde se guardan los reportes |
| `use_utc` | bool | `true` | Usar UTC para timestamps (recomendado) |
| `include_raw` | bool | `true` | Incluir JSON raw en los reportes |
| `enabled` | bool | `true` | Habilitar generación de reportes |

## Formato de los Reportes

Cada reporte Markdown incluye:

### YAML Frontmatter

Metadatos estructurados al inicio del archivo:

```yaml
---
type: analysis
version: v1
agent: claude
model: claude-3-5-sonnet-20241022
timestamp: 2024-01-15T14:30:52Z
workflow_id: wf-1705329052-1
tokens_in: 5432
tokens_out: 2341
cost_usd: "0.0234"
duration_ms: 12543
---
```

### Secciones Estándar

- **Metodología**: Descripción del enfoque utilizado para el análisis
- **Contenido Principal**: El análisis, crítica o síntesis
- **Métricas**: Tokens, costo y duración de la operación
- **Raw Output** (opcional): JSON completo de la respuesta del agente

## Principios del Análisis

### Para Análisis V1

El análisis debe ser:

- **Exhaustivo**: Sin límites artificiales de profundidad
- **Fundamentado**: Basado en evidencia del código y documentación
- **Actualizado**: Alineado con las versiones actuales de frameworks/librerías
- **Práctico**: Con recomendaciones accionables y específicas
- **Objetivo**: Sin sesgos hacia tecnologías o patrones específicos

### Para Críticas V2

La crítica debe ser:

- **Integral**: Considerar TODOS los análisis V1 generados
- **Rigurosa**: No aceptar afirmaciones sin evidencia sólida
- **Constructiva**: Identificar mejoras, no solo criticar
- **Adversarial**: Buscar activamente puntos débiles
- **Documentada**: Con referencias a documentación oficial

### Para Reconciliación V3

La reconciliación debe:

- **Resolver divergencias**: No ignorar desacuerdos
- **Priorizar**: Según impacto y probabilidad de los riesgos
- **Fundamentar**: Cada decisión con evidencia técnica
- **Unificar**: Producir un resultado coherente y accionable
- **Trazar**: Mantener referencias a los análisis originales

## Uso de los Reportes

Los reportes generados sirven para:

1. **Auditoría**: Trazabilidad completa de decisiones técnicas
2. **Documentación**: Registro de análisis para referencia futura
3. **Aprendizaje**: Comparación de perspectivas entre diferentes agentes
4. **Mejora continua**: Identificación de patrones en análisis
5. **Cumplimiento**: Evidencia de procesos de revisión técnica

## Integración con CI/CD

Los reportes pueden integrarse en pipelines de CI/CD:

```yaml
# Ejemplo para GitHub Actions
- name: Run Quorum Analysis
  run: quorum analyze --report-dir=./reports

- name: Upload Analysis Reports
  uses: actions/upload-artifact@v3
  with:
    name: quorum-reports
    path: .quorum-output/
```

## Limitaciones Conocidas

- Los reportes se generan en español por defecto
- El formato es Markdown estático (no HTML interactivo)
- Los archivos grandes pueden exceder límites de algunos visualizadores

## Contribución

Para mejorar el sistema de reportes:

1. Los templates están en `internal/service/report/templates.go`
2. La lógica de escritura en `internal/service/report/writer.go`
3. El frontmatter YAML en `internal/service/report/frontmatter.go`

## Ver También

- [ARCHITECTURE.md](./ARCHITECTURE.md) - Arquitectura general de Quorum
- [CONFIGURATION.md](./CONFIGURATION.md) - Guía de configuración completa
- [ANALYZE_OUTPUT_PROPOSAL.md](./ANALYZE_OUTPUT_PROPOSAL.md) - Propuesta original del sistema
