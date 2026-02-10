# Workflow Reports - Generación de Reportes Markdown

Este documento describe el sistema de generación automática de reportes Markdown que documenta cada ejecución del workflow de análisis multi-agente de Quorum AI.

## Tabla de Contenidos

- [Visión General](#visión-general)
- [Estructura de Directorios](#estructura-de-directorios)
- [Fases del Análisis](#fases-del-análisis)
  - [Análisis V1 - Análisis Inicial](#análisis-v1---análisis-inicial)
  - [V(n) Refinamiento Iterativo](#vn-refinamiento-iterativo)
  - [Evaluación del Árbitro](#evaluación-del-árbitro)
  - [Análisis Consolidado](#análisis-consolidado)
- [Configuración](#configuración)
- [Formato de los Reportes](#formato-de-los-reportes)
- [Principios del Análisis](#principios-del-análisis)

## Visión General

Quorum AI genera automáticamente reportes Markdown estructurados para cada ejecución del workflow. Estos reportes documentan:

- El prompt original y optimizado
- Los análisis independientes de cada agente (V1)
- Los refinamientos iterativos V(n) entre agentes
- Las evaluaciones de consenso del árbitro semántico
- El análisis consolidado final
- Los resultados de planificación y ejecución

## Estructura de Directorios

### Modo Multi-Agente (por defecto)

```
.quorum/runs/
└── {workflow-id}/                       # Ej: wf-1705329052-1
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
    │   ├── v2/                          # Refinamiento V2
    │   │   ├── claude-refinement.md
    │   │   ├── gemini-refinement.md
    │   │   └── ...
    │   │
    │   ├── v3/                          # Refinamiento V3 (si necesario)
    │   │   └── ...
    │   │
    │   ├── consensus/                   # Evaluaciones del árbitro
    │   │   ├── arbiter-round-2.md
    │   │   ├── arbiter-round-3.md
    │   │   └── ...
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

### Modo Single-Agent

Cuando se activa el modo single-agent (`--single-agent` o `single_agent.enabled: true`), la estructura se simplifica:

```
.quorum/runs/
└── {workflow-id}/
    ├── metadata.md
    ├── workflow-summary.md
    │
    ├── analyze-phase/
    │   ├── 00-original-prompt.md
    │   ├── 01-optimized-prompt.md
    │   │
    │   ├── single-agent/                # Análisis de un solo agente
    │   │   └── {agent}-{model}.md       # Ej: claude-claude-opus-4-6.md
    │   │
    │   └── consolidated.md              # Igual formato, metadata indica mode: "single_agent"
    │
    ├── plan-phase/
    │   └── ...
    │
    └── execute-phase/
        └── ...
```

**Diferencias clave en modo single-agent:**

- No hay subdirectorios `v1/`, `v2/`, `v3/` (no hay refinamiento iterativo)
- No hay subdirectorio `consensus/` (no hay evaluación del moderador)
- El análisis se guarda en `single-agent/{agent}-{model}.md`
- El `consolidated.md` tiene metadata `mode: "single_agent"` en lugar de métricas de consenso

## Fases del Análisis

### Análisis V1 - Análisis Inicial

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

### V(n) Refinamiento Iterativo

Los rounds V(n) (V2, V3, etc.) representan **refinamientos iterativos** donde cada agente revisa y mejora el análisis de la ronda anterior.

#### Características del Refinamiento V(n)

1. **Evalúa la ronda anterior**: Cada V(n+1) revisa solo los outputs de V(n)
2. **Identifica inconsistencias**: Detecta contradicciones y gaps en los análisis
3. **Cuestiona fundamentación**: Verifica que cada afirmación esté respaldada por evidencia concreta
4. **Valida contra documentación**: Comprueba las recomendaciones contra documentación oficial actualizada
5. **Evalúa completitud**: Identifica aspectos no cubiertos en los análisis anteriores
6. **Perspectiva crítica**: Busca activamente debilidades y puntos ciegos

#### Contenido del Refinamiento V(n)

- **Puntos de Acuerdo Validados**: Conclusiones del análisis anterior que se consideran correctas y bien fundamentadas
- **Puntos de Desacuerdo / Correcciones**: Aspectos donde el análisis anterior es incorrecto, incompleto o mal fundamentado
- **Riesgos Adicionales No Identificados**: Riesgos que el análisis anterior pasó por alto o subestimó

### Evaluación del Árbitro

El árbitro semántico evalúa el consenso entre los agentes después de cada ronda de refinamiento.

#### Proceso de Evaluación

1. **Análisis semántico**: El árbitro evalúa el acuerdo semántico entre los outputs de los agentes
2. **Puntuación de consenso**: Genera un porcentaje de consenso (0-100%)
3. **Identificación de divergencias**: Documenta los puntos de desacuerdo significativos
4. **Recomendación**: Indica si continuar con más rounds o proceder a consolidación

#### Criterios del Árbitro

- **Threshold de consenso**: Por defecto 90% para proceder
- **Rounds mínimos**: Al menos 2 rounds antes de declarar consenso
- **Rounds máximos**: Máximo 5 rounds antes de abortar o consolidar
- **Detección de estancamiento**: Si la mejora entre rounds es < 2%, se considera estancado

### Análisis Consolidado

El documento final que integra todas las perspectivas:

1. **Análisis V1 independientes**: Cada agente realizó un análisis profundo y exhaustivo
2. **Refinamientos V(n)**: Los agentes refinaron iterativamente sus análisis
3. **Evaluaciones del árbitro**: El árbitro semántico validó el consenso
4. **Consolidación final**: Se integran todas las perspectivas en un documento unificado

## Configuración

La generación de reportes se configura en el archivo de configuración de Quorum:

```yaml
report:
  # Directorio base para los reportes (relativo al directorio del proyecto)
  base_dir: ".quorum/runs"

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
| `base_dir` | string | `.quorum/runs` | Directorio donde se guardan los reportes |
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
duration_ms: 12543
---
```

### Secciones Estándar

- **Metodología**: Descripción del enfoque utilizado para el análisis
- **Contenido Principal**: El análisis, refinamiento o evaluación
- **Métricas**: Tokens y duración de la operación
- **Raw Output** (opcional): JSON completo de la respuesta del agente

## Principios del Análisis

### Para Análisis V1

El análisis debe ser:

- **Exhaustivo**: Sin límites artificiales de profundidad
- **Fundamentado**: Basado en evidencia del código y documentación
- **Actualizado**: Alineado con las versiones actuales de frameworks/librerías
- **Práctico**: Con recomendaciones accionables y específicas
- **Objetivo**: Sin sesgos hacia tecnologías o patrones específicos

### Para Refinamientos V(n)

El refinamiento debe ser:

- **Iterativo**: Revisar solo la ronda anterior, no rounds anteriores
- **Riguroso**: No aceptar afirmaciones sin evidencia sólida
- **Constructivo**: Identificar mejoras, no solo criticar
- **Crítico**: Buscar activamente puntos débiles
- **Documentado**: Con referencias a documentación oficial

### Para Evaluación del Árbitro

El árbitro debe:

- **Evaluar semánticamente**: Entender el significado, no solo comparar texto
- **Ser objetivo**: Aplicar criterios consistentes
- **Documentar divergencias**: Explicar los puntos de desacuerdo
- **Fundamentar**: Justificar la puntuación de consenso

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
    path: .quorum/runs/
```

## Limitaciones Conocidas

- Los reportes se generan en español por defecto
- El formato es Markdown estático (no HTML interactivo)
- Los archivos grandes pueden exceder límites de algunos visualizadores

## Contribución

Para mejorar el sistema de reportes:

1. Los renderizadores están en `internal/service/report/renderers.go`
2. La lógica de escritura en `internal/service/report/writer.go`
3. El frontmatter YAML en `internal/service/report/frontmatter.go`

## Ver También

- [ARCHITECTURE.md](./ARCHITECTURE.md) - Arquitectura general de Quorum
- [CONFIGURATION.md](./CONFIGURATION.md) - Guía de configuración completa
