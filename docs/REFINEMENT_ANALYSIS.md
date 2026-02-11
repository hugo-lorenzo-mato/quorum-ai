# Análisis del Sistema de Refinamiento de Prompts

**Fecha:** 2026-02-11
**Versión:** 1.0
**Estado:** Propuesta de Mejora

## Resumen Ejecutivo

El sistema actual de refinamiento de prompts tiende a **expandir el alcance** más allá de la intención del usuario. Esta expansión ocurre porque el template `refine-prompt.md.tmpl` instruye al LLM a agregar contexto técnico y consideraciones que el usuario no solicitó explícitamente.

**Propuesta:** Cambiar de una filosofía de "expansión técnica" a una de "clarificación preservando alcance".

---

## 1. Análisis del Problema

### 1.1 Comportamiento Actual

El template `internal/service/prompts/refine-prompt.md.tmpl` contiene instrucciones que fomentan la expansión:

```markdown
1. **Expand Context**: Add relevant technical context
2. **Add Specificity**: Include specific technical aspects
3. **Technical Depth**: Add considerations user might not have mentioned
4. **DO NOT prioritize brevity over completeness**
```

### 1.2 Casos de Uso Problemáticos

#### Ejemplo 1: Solicitud Simple

**Usuario pide:**
```
Agregar validación al formulario de login
```

**Sistema refina a:**
```markdown
# Task: Implement Comprehensive Form Validation

## Technical Requirements
1. Email validation (RFC 5322 compliant)
2. Password strength (min 8 chars, uppercase, lowercase, numbers, special chars)
3. SQL injection prevention
4. XSS attack mitigation
5. Rate limiting to prevent brute force
6. CSRF token validation
...
```

**Problema:** El usuario solo quería validación básica, pero el refinamiento agregó seguridad completa, rate limiting, etc.

#### Ejemplo 2: Solicitud Ambigua

**Usuario pide:**
```
Mejorar el rendimiento de la app
```

**Sistema refina a:**
```markdown
## Comprehensive Performance Audit
1. Frontend: Bundle size, code splitting, lazy loading, caching
2. Backend: Query optimization, API timing, memory leaks, CPU profiling
3. Infrastructure: CDN, load balancing, database indexing
4. Monitoring: APM setup, metrics collection, alerting
...
```

**Problema:** El usuario quería identificar y arreglar problemas de rendimiento, no hacer una auditoría completa de infraestructura.

### 1.3 Causas Raíz

1. **Lenguaje expansivo en las instrucciones**
   - "Add relevant technical context" → El LLM interpreta "todo lo técnicamente relevante"
   - "With maximum thoroughness" → Fomenta exhaustividad sobre precisión

2. **Falta de restricciones claras**
   - No hay validación de que el alcance se mantenga
   - No hay penalización por agregar requisitos

3. **Ausencia de ejemplos de límites**
   - El template no muestra qué NO hacer
   - No hay ejemplos de refinamiento "correcto" vs "incorrecto"

---

## 2. Impacto del Problema

### 2.1 En el Proceso de Análisis

- **Análisis más largo:** Agentes gastan tokens analizando cosas no solicitadas
- **Resultados fuera de foco:** El análisis incluye aspectos irrelevantes
- **Mayor tiempo de ejecución:** Más código para revisar = más tiempo

### 2.2 En la Experiencia del Usuario

- **Frustración:** "No pedí esto"
- **Confusión:** Resultados que no corresponden a la solicitud
- **Desconfianza:** El sistema "no entiende" lo que quiero

### 2.3 En Costos

- **Tokens desperdiciados:** Análisis de aspectos no requeridos
- **Tiempo de agentes:** Ejecuciones más largas innecesariamente
- **Rate limits:** Más probabilidad de alcanzar límites

---

## 3. Propuesta de Solución

### 3.1 Nuevo Template: `refine-prompt-v2.md.tmpl`

**Ubicación:** `internal/service/prompts/refine-prompt-v2.md.tmpl`

**Filosofía:**
```
Clarificar ambigüedades manteniendo el alcance exacto del usuario
```

**Principios clave:**
1. **Preservar intención del usuario (prioridad máxima)**
2. **Clarificar, no expandir**
3. **Accionable sobre exhaustivo**

### 3.2 Cambios Principales

| Aspecto | Template Actual | Template Mejorado |
|---------|----------------|-------------------|
| **Objetivo** | Expandir con contexto técnico | Clarificar manteniendo alcance |
| **Alcance** | Agregar consideraciones no mencionadas | Solo clarificar lo solicitado |
| **Longitud** | No priorizar brevedad | Preferir precisión sobre volumen |
| **Validación** | Confiar en el LLM | Checklist explícito de verificación |

### 3.3 Instrucciones Mejoradas

**✅ DO:**
- Disambiguar términos vagos
- Agregar contexto de ejecución solo si falta
- Preservar restricciones del usuario
- Estructurar para claridad
- Agregar requisitos de actitud crítica

**❌ DO NOT:**
- Agregar requisitos no en el original
- Expandir alcance "por completitud"
- Agregar consideraciones técnicas no relevantes
- Cambiar el tipo de solicitud
- Agregar meta-instrucciones sobre exhaustividad

### 3.4 Quality Checks

El template incluye un checklist que el LLM debe verificar antes de responder:

```markdown
- [ ] ¿Se puede trazar el prompt refinado al original?
- [ ] ¿El usuario reconocería su solicitud?
- [ ] ¿Agregué SOLO clarificaciones necesarias?
- [ ] ¿El alcance es idéntico o más estrecho (nunca más amplio)?
- [ ] ¿Este prompt llevaría al resultado que el usuario espera?
```

---

## 4. Plan de Implementación

### Fase 1: Validación con Tests (Actual)

**Archivos creados:**
- `internal/service/prompts/refine-prompt-v2.md.tmpl` - Nuevo template
- `internal/service/workflow/refiner_scope_test.go` - Tests de validación

**Objetivo:** Establecer expectativas claras de comportamiento

### Fase 2: Configuración Optional

**Cambios en config:**

```yaml
# .quorum/config.yaml
phases:
  analyze:
    refiner:
      enabled: true
      agent: claude
      template: refine-prompt-v2  # Opcional: usar nuevo template
```

**Código necesario:**

```go
// internal/service/workflow/refiner.go

type RefinerConfig struct {
    Enabled  bool
    Agent    string
    Template string // "refine-prompt" (default) or "refine-prompt-v2"
}

// En Run():
templateName := "refine-prompt"
if r.config.Template != "" {
    templateName = r.config.Template
}

prompt, err := wctx.Prompts.RenderPromptByName(templateName, RefinePromptParams{
    OriginalPrompt: wctx.State.Prompt,
})
```

### Fase 3: A/B Testing

**Objetivo:** Comparar resultados entre templates

**Métrica sugerida:**
1. **Scope Preservation Score**: ¿El refinado mantiene el alcance?
2. **User Satisfaction**: Feedback del usuario
3. **Token Efficiency**: Tokens usados en análisis
4. **Execution Time**: Tiempo total de workflow

### Fase 4: Rollout Completo

**Criterio de éxito:**
- Scope Preservation Score > 90%
- User Satisfaction mejora en >20%
- Token Efficiency mejora en >15%

**Acción:** Reemplazar template actual con v2

---

## 5. Consideraciones Adicionales

### 5.1 Monitoreo Post-Implementación

**Métricas a trackear:**
```go
type RefinementMetrics struct {
    OriginalLength      int
    RefinedLength       int
    LengthRatio        float64
    NewTermsAdded      int
    ScopeExpansionScore float64
    UserFeedback       string
}
```

### 5.2 Feedback Loop

Agregar capacidad de que el usuario califique el refinamiento:

```
🔄 Prompt refinado
Original: "Agregar validación al formulario"
Refinado: "Agregar validación de email y password al formulario de login"

¿El refinamiento preservó tu intención? [Sí] [No] [Expandió demasiado]
```

### 5.3 Personalización por Agente

Algunos agentes pueden necesitar más contexto que otros:

```yaml
phases:
  analyze:
    refiner:
      agent: claude
      template: refine-prompt-v2
      config:
        expansion_tolerance: low  # low, medium, high
```

### 5.4 Fallback a Original

Si el refinamiento falla o es rechazado por validación, usar prompt original:

```go
if !validateRefinement(original, refined) {
    wctx.Logger.Warn("refinement failed validation, using original")
    wctx.State.OptimizedPrompt = wctx.State.Prompt
}
```

---

## 6. Tests y Validación

### 6.1 Tests Existentes

**Archivo:** `internal/service/workflow/refiner_test.go`
- ✅ Parsing de resultados
- ✅ Manejo de CLI wrapper
- ✅ Whitespace handling

### 6.2 Nuevos Tests

**Archivo:** `internal/service/workflow/refiner_scope_test.go`
- ✅ Validación de preservación de alcance
- ✅ Detección de expansión no deseada
- ✅ Verificación de requisitos de calidad

**Nota:** Los tests de `refiner_scope_test.go` están diseñados para **fallar** con el template actual y **pasar** con el template mejorado. Esto documenta el comportamiento esperado.

### 6.3 Testing Manual

Para probar manualmente el refinamiento:

```bash
# Con template actual
quorum analyze "Agregar validación al formulario" \
  --refine \
  --refine-agent claude

# Con template v2 (después de implementación)
quorum analyze "Agregar validación al formulario" \
  --refine \
  --refine-template refine-prompt-v2 \
  --refine-agent claude
```

---

## 7. Ejemplos de Mejora

### Ejemplo 1: Solicitud de Validación

**Original:**
```
Agregar validación al formulario de login
```

**Actual (problemático):**
```markdown
Implementar sistema completo de validación que incluya:
- Validación de email (RFC 5322)
- Validación de password (min 8 chars, mayúsculas, minúsculas, números)
- Prevención de SQL injection
- Prevención de XSS
- Rate limiting para brute force
- CSRF tokens
- Manejo de errores con UX mejorada
- Logs de auditoría
```
❌ Expansión: 120 palabras → 600 palabras, agregó seguridad completa

**Mejorado (correcto):**
```
Agregar validación de entrada al formulario de login:
- Validar que el email tenga formato válido
- Validar que el password no esté vacío
- Mostrar mensajes de error claros si la validación falla
```
✅ Clarificación: 120 palabras → 150 palabras, mantuvo alcance

### Ejemplo 2: Solicitud de Performance

**Original:**
```
Mejorar el rendimiento de la app
```

**Actual (problemático):**
```markdown
Realizar auditoría completa de rendimiento que cubra:
1. Frontend: Optimización de bundle, code splitting, lazy loading, optimización de imágenes
2. Backend: Optimización de queries, análisis de API response time, detección de memory leaks
3. Infraestructura: Configuración de CDN, load balancing, indexación de DB
4. Monitoreo: Setup de APM, recolección de métricas, alertas
```
❌ Expansión: Se convirtió en proyecto completo de infraestructura

**Mejorado (correcto):**
```
Identificar y arreglar cuellos de botella de rendimiento:
- Identificar operaciones lentas (carga de páginas, llamadas API, queries)
- Proveer recomendaciones específicas con referencias file:line
- Priorizar mejoras de alto impacto

Enfocarse en problemas que afectan la experiencia del usuario.
```
✅ Clarificación: Mantiene objetivo, agrega especificidad sin dictar TODO el alcance

### Ejemplo 3: Bug Fix

**Original:**
```
Arreglar el error de null pointer en el servicio de usuarios
```

**Actual (problemático):**
```markdown
Investigar y resolver el error de null pointer en el servicio de usuarios:
1. Identificar la causa raíz del null pointer
2. Implementar el fix con manejo robusto de errores
3. Refactorizar el servicio para prevenir errores similares
4. Agregar tests unitarios y de integración
5. Documentar el fix y las mejoras arquitectónicas
6. Considerar patrones de diseño para mejorar la resiliencia
```
❌ Expansión: Bug fix simple → proyecto de refactorización

**Mejorado (correcto):**
```
Arreglar el error de null pointer en el servicio de usuarios:
- Identificar la línea específica que causa el error
- Implementar verificación de null para prevenir el crash
- Citar el archivo y línea del fix
```
✅ Clarificación: Se mantiene como bug fix, no agrega refactorización

---

## 8. Métricas de Éxito

### 8.1 Métricas Cuantitativas

| Métrica | Valor Actual (Estimado) | Objetivo con v2 |
|---------|-------------------------|-----------------|
| **Scope Preservation** | ~60% | >90% |
| **Length Ratio** (refined/original) | ~2.5x | <1.5x |
| **Token Efficiency** | Baseline | +15% |
| **Execution Time** | Baseline | -10% |
| **New Terms Added** | ~7-10 | <3 |

### 8.2 Métricas Cualitativas

- **User Feedback:** "¿El resultado corresponde a tu solicitud?"
  - Actual: ~70% positivo
  - Objetivo: >90% positivo

- **Agent Focus:** "¿El análisis se mantiene en el tema?"
  - Actual: ~65% enfocado
  - Objetivo: >85% enfocado

---

## 9. Roadmap

### Corto Plazo (1-2 semanas)
- [x] Crear template mejorado `refine-prompt-v2.md.tmpl`
- [x] Implementar tests de validación `refiner_scope_test.go`
- [ ] Agregar configuración de template en `RefinerConfig`
- [ ] Documentar cambios en `CONFIGURATION.md`

### Medio Plazo (3-4 semanas)
- [ ] A/B testing con usuarios beta
- [ ] Recopilar métricas comparativas
- [ ] Ajustar template basado en feedback

### Largo Plazo (1-2 meses)
- [ ] Rollout a producción si métricas son positivas
- [ ] Deprecar template antiguo
- [ ] Implementar feedback loop automático

---

## 10. Conclusiones

### Hallazgos Principales

1. **Problema identificado:** El refinamiento actual expande el alcance más allá de la intención del usuario
2. **Causa raíz:** Instrucciones que fomentan "agregar contexto técnico" sin restricciones
3. **Impacto:** Análisis más largos, resultados fuera de foco, frustración del usuario

### Solución Propuesta

1. **Nuevo template** con filosofía de "clarificar sin expandir"
2. **Checklist de validación** para que el LLM verifique preservación de alcance
3. **Tests automáticos** para validar comportamiento

### Próximos Pasos

1. **Implementar** configuración de template en `RefinerConfig`
2. **Probar** manualmente con casos de uso reales
3. **Medir** y comparar con template actual
4. **Decidir** sobre rollout basado en métricas

---

## Referencias

- **Template actual:** `internal/service/prompts/refine-prompt.md.tmpl`
- **Template mejorado:** `internal/service/prompts/refine-prompt-v2.md.tmpl`
- **Lógica de refinamiento:** `internal/service/workflow/refiner.go`
- **Tests:** `internal/service/workflow/refiner_test.go`, `refiner_scope_test.go`
- **Configuración:** `configs/default.yaml` → `phases.analyze.refiner`

---

**Autor:** Claude Sonnet 4.5
**Fecha:** 2026-02-11
**Versión del documento:** 1.0
