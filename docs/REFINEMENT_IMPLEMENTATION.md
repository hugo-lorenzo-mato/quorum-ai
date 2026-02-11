# Guía de Implementación: Refinamiento Mejorado de Prompts

**Objetivo:** Implementar el nuevo sistema de refinamiento que preserva el alcance del usuario.

---

## 1. Cambios Necesarios en el Código

### 1.1 Agregar Configuración de Template

**Archivo:** `internal/service/workflow/refiner.go:14-19`

```go
// RefinerConfig configures the refiner (prompt refinement) phase.
type RefinerConfig struct {
	Enabled  bool
	Agent    string
	Template string // NEW: "refine-prompt" (default) or "refine-prompt-v2"
	// Model is resolved from AgentPhaseModels[Agent][optimize] at runtime.
}
```

### 1.2 Modificar la Llamada de Renderizado

**Archivo:** `internal/service/workflow/refiner.go:89-95`

**Antes:**
```go
// Render refinement prompt
prompt, err := wctx.Prompts.RenderRefinePrompt(RefinePromptParams{
	OriginalPrompt: wctx.State.Prompt,
})
```

**Después:**
```go
// Render refinement prompt with configurable template
templateName := r.config.Template
if templateName == "" {
	templateName = "refine-prompt" // default to current template
}

var prompt string
var err error

switch templateName {
case "refine-prompt-v2":
	prompt, err = wctx.Prompts.RenderPromptByName("refine-prompt-v2", RefinePromptParams{
		OriginalPrompt: wctx.State.Prompt,
	})
case "refine-prompt":
	fallthrough
default:
	prompt, err = wctx.Prompts.RenderRefinePrompt(RefinePromptParams{
		OriginalPrompt: wctx.State.Prompt,
	})
}

if err != nil {
	return fmt.Errorf("rendering refine prompt: %w", err)
}
```

### 1.3 Agregar Método RenderPromptByName

**Archivo:** `internal/service/prompt.go` (agregar después de línea 106)

```go
// RenderPromptByName renders a prompt by its template name.
func (r *PromptRenderer) RenderPromptByName(templateName string, params interface{}) (string, error) {
	return r.render(templateName, params)
}
```

**Archivo:** `internal/service/workflow/context.go:209` (actualizar interface)

```go
type PromptRenderer interface {
	RenderRefinePrompt(params RefinePromptParams) (string, error)
	RenderPromptByName(templateName string, params interface{}) (string, error) // NEW
	// ... otros métodos ...
}
```

**Archivo:** `internal/service/workflow/adapters.go:116-120` (agregar método)

```go
// RenderPromptByName renders a prompt by its template name.
func (a *PromptRendererAdapter) RenderPromptByName(templateName string, params interface{}) (string, error) {
	// Convert interface{} to appropriate type based on template
	switch templateName {
	case "refine-prompt", "refine-prompt-v2":
		if p, ok := params.(RefinePromptParams); ok {
			return a.renderer.RenderPromptByName(templateName, service.RefinePromptParams{
				OriginalPrompt: p.OriginalPrompt,
			})
		}
	}
	return "", fmt.Errorf("unsupported template or params type: %s", templateName)
}
```

### 1.4 Actualizar Configuración YAML

**Archivo:** `configs/default.yaml`

```yaml
phases:
  analyze:
    refiner:
      enabled: true
      agent: "claude"
      template: "refine-prompt"  # NEW: or "refine-prompt-v2" for improved version
      # model is resolved from agent.phase_models.refine
```

**Archivo:** `internal/config/config.go` (actualizar struct)

```go
type RefinerConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Agent    string `yaml:"agent"`
	Template string `yaml:"template"` // NEW
}
```

---

## 2. Plan de Implementación Paso a Paso

### Paso 1: Preparación (1 día)
- [x] ✅ Crear template mejorado `refine-prompt-v2.md.tmpl`
- [x] ✅ Crear tests de validación `refiner_scope_test.go`
- [x] ✅ Crear documentación de análisis

### Paso 2: Implementación Base (2-3 días)
- [ ] Agregar campo `Template` a `RefinerConfig` struct
- [ ] Modificar `refiner.go:Run()` para usar template configurable
- [ ] Agregar método `RenderPromptByName` en `prompt.go`
- [ ] Actualizar interface `PromptRenderer` y su adapter
- [ ] Actualizar `default.yaml` con nueva opción
- [ ] Actualizar validador de config para campo `template`

### Paso 3: Testing (2-3 días)
- [ ] Ejecutar tests existentes (deben seguir pasando)
- [ ] Ejecutar nuevos tests de validación
- [ ] Pruebas manuales con ambos templates:
  ```bash
  # Template actual
  quorum analyze "Agregar validación al formulario" --refine

  # Template v2
  # Editar .quorum/config.yaml: template: refine-prompt-v2
  quorum analyze "Agregar validación al formulario" --refine
  ```

### Paso 4: Monitoreo y Métricas (1 semana)
- [ ] Implementar logging de métricas:
  - Longitud original vs refinada
  - Tiempo de refinamiento
  - Tokens usados
- [ ] Comparar resultados entre templates
- [ ] Recopilar feedback de usuarios beta

### Paso 5: Decisión y Rollout (según resultados)
- [ ] Si métricas son positivas (>20% mejora):
  - Cambiar default a `refine-prompt-v2`
  - Deprecar template antiguo
- [ ] Si métricas son mixtas:
  - Mantener ambos templates
  - Permitir configuración por usuario
- [ ] Documentar en `CONFIGURATION.md`

---

## 3. Código Completo de Cambios

### 3.1 Archivo: `internal/config/config.go`

```go
// RefinerConfig configures the prompt refinement phase.
type RefinerConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Agent    string `yaml:"agent"`
	Template string `yaml:"template"` // "refine-prompt" or "refine-prompt-v2"
}
```

### 3.2 Archivo: `internal/config/validator.go`

```go
// En validateRefinerConfig(), agregar:
func validateRefinerConfig(cfg *RefinerConfig, agents map[string]AgentConfig) []ValidationError {
	var errs []ValidationError

	// ... validaciones existentes ...

	// Validate template if specified
	if cfg.Template != "" {
		validTemplates := []string{"refine-prompt", "refine-prompt-v2"}
		if !contains(validTemplates, cfg.Template) {
			errs = append(errs, ValidationError{
				Path:    "phases.analyze.refiner.template",
				Message: fmt.Sprintf("must be one of: %v", validTemplates),
			})
		}
	}

	return errs
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
```

### 3.3 Archivo: `internal/service/workflow/refiner.go`

```go
func (r *Refiner) Run(ctx context.Context, wctx *Context) error {
	// ... código existente hasta línea 89 ...

	// Determine which template to use
	templateName := r.config.Template
	if templateName == "" {
		templateName = "refine-prompt" // default
	}

	// Render refinement prompt with selected template
	var prompt string
	var err error

	params := RefinePromptParams{
		OriginalPrompt: wctx.State.Prompt,
	}

	switch templateName {
	case "refine-prompt-v2":
		prompt, err = wctx.Prompts.RenderPromptByName("refine-prompt-v2", params)
	case "refine-prompt":
		fallthrough
	default:
		prompt, err = wctx.Prompts.RenderRefinePrompt(params)
	}

	if err != nil {
		return fmt.Errorf("rendering refine prompt (%s): %w", templateName, err)
	}

	wctx.Logger.Info("using refinement template", "template", templateName)

	// ... resto del código sin cambios ...
}
```

### 3.4 Archivo: `internal/service/prompt.go`

```go
// Agregar después del método RenderRefinePrompt (línea ~106)

// RenderPromptByName renders a prompt by its template name.
// This is a generic method that allows rendering any loaded template.
func (r *PromptRenderer) RenderPromptByName(templateName string, params interface{}) (string, error) {
	return r.render(templateName, params)
}
```

### 3.5 Archivo: `internal/service/workflow/context.go`

```go
// Actualizar la interface PromptRenderer (línea ~209)
type PromptRenderer interface {
	RenderRefinePrompt(params RefinePromptParams) (string, error)
	RenderPromptByName(templateName string, params interface{}) (string, error) // NEW
	RenderAnalyzeV1(params AnalyzeV1Params) (string, error)
	RenderVNRefine(params VNRefineParams) (string, error)
	RenderModeratorEvaluate(params ModeratorEvaluateParams) (string, error)
}
```

### 3.6 Archivo: `internal/service/workflow/adapters.go`

```go
// Agregar después de RenderRefinePrompt (línea ~120)

// RenderPromptByName renders a prompt by its template name.
func (a *PromptRendererAdapter) RenderPromptByName(templateName string, params interface{}) (string, error) {
	// Type assertion based on template
	switch templateName {
	case "refine-prompt", "refine-prompt-v2":
		if p, ok := params.(RefinePromptParams); ok {
			return a.renderer.RenderPromptByName(templateName, service.RefinePromptParams{
				OriginalPrompt: p.OriginalPrompt,
			})
		}
		return "", fmt.Errorf("invalid params type for %s: expected RefinePromptParams", templateName)

	case "analyze-v1":
		if p, ok := params.(AnalyzeV1Params); ok {
			return a.renderer.RenderPromptByName(templateName, service.AnalyzeV1Params{
				Prompt:         p.Prompt,
				ProjectPath:    p.ProjectPath,
				Context:        p.Context,
				Constraints:    p.Constraints,
				OutputFilePath: p.OutputFilePath,
			})
		}
		return "", fmt.Errorf("invalid params type for %s: expected AnalyzeV1Params", templateName)

	default:
		return "", fmt.Errorf("unsupported template: %s", templateName)
	}
}
```

### 3.7 Archivo: `configs/default.yaml`

```yaml
phases:
  analyze:
    refiner:
      enabled: true
      agent: "claude"
      template: "refine-prompt"  # Options: "refine-prompt" (default), "refine-prompt-v2" (improved)
```

---

## 4. Testing Completo

### 4.1 Tests Unitarios

```bash
# Ejecutar tests existentes
go test ./internal/service/workflow/refiner_test.go -v

# Ejecutar nuevos tests de validación de alcance
go test ./internal/service/workflow/refiner_scope_test.go -v

# Ejecutar tests de configuración
go test ./internal/config/validator_test.go -v -run TestValidateRefinerConfig
```

### 4.2 Tests de Integración

```bash
# Test 1: Validación simple con template actual
quorum analyze "Agregar validación al formulario de login" \
  --refine \
  --refine-agent claude \
  --dry-run

# Test 2: Validación simple con template v2
# Primero, editar .quorum/config.yaml:
#   phases.analyze.refiner.template: refine-prompt-v2
quorum analyze "Agregar validación al formulario de login" \
  --refine \
  --refine-agent claude \
  --dry-run

# Test 3: Bug fix con template actual
quorum analyze "Arreglar el error de null pointer en user.go" \
  --refine \
  --refine-agent claude

# Test 4: Bug fix con template v2
quorum analyze "Arreglar el error de null pointer en user.go" \
  --refine \
  --refine-agent claude \
  --config .quorum/config-v2.yaml  # usar config con template: refine-prompt-v2
```

### 4.3 Comparación de Resultados

Crear script de comparación:

```bash
#!/bin/bash
# compare-refinements.sh

TEST_PROMPTS=(
  "Agregar validación al formulario"
  "Mejorar el rendimiento de la app"
  "Arreglar bug en el servicio de usuarios"
)

for prompt in "${TEST_PROMPTS[@]}"; do
  echo "=== Testing: $prompt ==="

  echo "--- Template actual ---"
  quorum analyze "$prompt" --refine --dry-run > /tmp/refine-actual.txt

  echo "--- Template v2 ---"
  # Cambiar config temporalmente
  sed -i 's/template: refine-prompt/template: refine-prompt-v2/' .quorum/config.yaml
  quorum analyze "$prompt" --refine --dry-run > /tmp/refine-v2.txt

  # Comparar longitudes
  echo "Longitud actual: $(wc -w < /tmp/refine-actual.txt) palabras"
  echo "Longitud v2: $(wc -w < /tmp/refine-v2.txt) palabras"

  # Restaurar config
  sed -i 's/template: refine-prompt-v2/template: refine-prompt/' .quorum/config.yaml

  echo ""
done
```

---

## 5. Métricas y Monitoreo

### 5.1 Agregar Logging de Métricas

**Archivo:** `internal/service/workflow/refiner.go` (agregar antes de línea 200)

```go
// Log refinement metrics
refinementMetrics := map[string]interface{}{
	"template":          templateName,
	"original_length":   len(wctx.State.Prompt),
	"refined_length":    len(refined),
	"length_ratio":      float64(len(refined)) / float64(len(wctx.State.Prompt)),
	"tokens_in":         result.TokensIn,
	"tokens_out":        result.TokensOut,
	"duration_ms":       durationMS,
}

wctx.Logger.Info("refinement completed", refinementMetrics)

if wctx.Output != nil {
	wctx.Output.Log("debug", "refiner",
		fmt.Sprintf("Template: %s, Length: %d → %d (%.1fx)",
			templateName,
			len(wctx.State.Prompt),
			len(refined),
			float64(len(refined))/float64(len(wctx.State.Prompt)),
		),
	)
}
```

### 5.2 Exportar Métricas

Agregar endpoint de métricas (opcional):

```go
// internal/api/metrics.go
type RefinementMetrics struct {
	Template       string  `json:"template"`
	OriginalLength int     `json:"original_length"`
	RefinedLength  int     `json:"refined_length"`
	LengthRatio    float64 `json:"length_ratio"`
	TokensUsed     int     `json:"tokens_used"`
	DurationMS     int64   `json:"duration_ms"`
	Timestamp      string  `json:"timestamp"`
}

// Guardar métricas para análisis posterior
```

---

## 6. Documentación para Usuarios

### 6.1 Actualizar `docs/CONFIGURATION.md`

```markdown
### Refiner Configuration

The refiner phase enhances user prompts for better analysis results.

#### Options

- `enabled` (bool): Enable/disable prompt refinement
- `agent` (string): Which agent to use for refinement (e.g., "claude")
- `template` (string): Refinement template to use
  - `refine-prompt` (default): Original template, adds extensive technical context
  - `refine-prompt-v2`: Improved template, preserves user intent and scope

#### Example

```yaml
phases:
  analyze:
    refiner:
      enabled: true
      agent: "claude"
      template: "refine-prompt-v2"  # Use improved template
```

#### Choosing a Template

**Use `refine-prompt` if:**
- You want maximum technical detail and context
- Your prompts are very high-level and need expansion
- You're doing exploratory analysis

**Use `refine-prompt-v2` if:**
- You want to preserve your original request scope
- Your prompts are already clear and specific
- You want focused, efficient analysis
```

### 6.2 Actualizar `README.md`

Agregar sección sobre refinamiento:

```markdown
## Prompt Refinement

Quorum AI can optionally refine your prompts to improve analysis quality:

```bash
# Enable refinement in your config
quorum analyze "your prompt" --refine
```

Two refinement strategies are available:

1. **refine-prompt** (default): Adds extensive technical context
2. **refine-prompt-v2**: Clarifies your request while preserving scope

Configure in `.quorum/config.yaml`:

```yaml
phases:
  analyze:
    refiner:
      template: "refine-prompt-v2"  # Recommended for focused analysis
```
```

---

## 7. Checklist de Implementación

### Código
- [ ] `RefinerConfig` struct actualizado con campo `Template`
- [ ] `refiner.go:Run()` modificado para usar template configurable
- [ ] `prompt.go` con método `RenderPromptByName` agregado
- [ ] Interface `PromptRenderer` actualizada
- [ ] Adapter `PromptRendererAdapter` actualizado
- [ ] Validador de config actualizado
- [ ] `default.yaml` actualizado con nueva opción

### Tests
- [ ] Tests existentes ejecutados y pasando
- [ ] Nuevos tests de validación de alcance ejecutados
- [ ] Tests de configuración para nuevo campo
- [ ] Tests de integración manual completados
- [ ] Script de comparación de templates ejecutado

### Documentación
- [ ] `REFINEMENT_ANALYSIS.md` creado
- [ ] `REFINEMENT_FLOW.md` creado
- [ ] `REFINEMENT_IMPLEMENTATION.md` creado
- [ ] `CONFIGURATION.md` actualizado
- [ ] `README.md` actualizado

### Métricas
- [ ] Logging de métricas implementado
- [ ] Comparación de resultados documentada
- [ ] Feedback de usuarios recopilado

### Decisión
- [ ] Análisis de métricas completado
- [ ] Decisión de rollout tomada
- [ ] Template default actualizado (si aplicable)
- [ ] Template antiguo deprecado (si aplicable)

---

## 8. Riesgos y Mitigaciones

### Riesgo 1: Prompts Demasiado Cortos
**Descripción:** Con template v2, prompts pueden ser demasiado concisos y perder contexto útil.

**Mitigación:**
- Monitorear tasa de errores en análisis
- Si aumenta >10%, ajustar template v2 para agregar más contexto
- Permitir configuración de "expansion_tolerance"

### Riesgo 2: Incompatibilidad con Agentes Específicos
**Descripción:** Algunos agentes pueden necesitar más contexto que otros.

**Mitigación:**
- Configuración por agente:
  ```yaml
  phases:
    analyze:
      refiner:
        template: "refine-prompt-v2"
        agent_overrides:
          gemini:
            template: "refine-prompt"  # Gemini necesita más contexto
  ```

### Riesgo 3: Resistencia de Usuarios al Cambio
**Descripción:** Usuarios acostumbrados al template actual pueden preferir el comportamiento antiguo.

**Mitigación:**
- Mantener ambos templates disponibles
- No cambiar el default inmediatamente
- Documentar diferencias claramente
- Proporcionar ejemplos de cuándo usar cada uno

---

## 9. Próximos Pasos Inmediatos

1. **Hoy:**
   - [ ] Revisar este documento y archivos creados
   - [ ] Decidir si proceder con implementación

2. **Esta semana:**
   - [ ] Implementar cambios de código (Pasos 1-2)
   - [ ] Ejecutar tests completos (Paso 3)

3. **Próxima semana:**
   - [ ] Testing manual con usuarios beta
   - [ ] Recopilación de métricas

4. **En 2-3 semanas:**
   - [ ] Análisis de resultados
   - [ ] Decisión de rollout

---

**Autor:** Claude Sonnet 4.5
**Fecha:** 2026-02-11
**Versión:** 1.0
