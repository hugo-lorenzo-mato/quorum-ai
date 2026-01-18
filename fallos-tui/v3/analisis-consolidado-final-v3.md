# Informe Consolidado Final de Defectos TUI v3

**Proyecto:** Quorum AI
**Fecha de consolidación:** 2026-01-18T23:00:00Z
**Versión:** 3.0 - Análisis Ultracrítico Final

## Fuentes del Análisis

| Fuente | Modelo | Fecha | Enfoque |
|--------|--------|-------|---------|
| `v2/analisis-consolidado-20260118T180000-v2.md` | claude-opus-4-5 | 2026-01-18 | Estructurado con validación de código |
| `v2/codex-gpt-5-20260118-224512-v2.md` | gpt-5 | 2026-01-18 | Riguroso con verificación de módulos |

**Artefactos adicionales consultados:**
- Código fuente: `internal/tui/chat/*.go`
- Capturas de pantalla: `img.png`, `img_1.png`, `img_2.png`
- Módulos externos: `glamour@v0.10.0/styles/dark.json`
- Documentación oficial: Lip Gloss, Glamour, Bubble Tea (pkg.go.dev)

**Metodología:** Fusión crítica de ambos análisis v2, preservando solo hallazgos con evidencia de código verificable, descartando especulaciones no sustentadas, y priorizando por impacto real en el usuario.

---

## Resumen Ejecutivo

Este análisis consolida los hallazgos de dos revisiones independientes (Claude Opus 4.5 y Codex GPT-5), cruzando y validando cada observación contra el código fuente real. El objetivo es producir una lista definitiva de defectos con evidencia irrefutable.

### Estadísticas de Hallazgos

| Categoría | Confirmados | No Confirmados | Parciales |
|-----------|-------------|----------------|-----------|
| Errores funcionales | 4 | 1 | 0 |
| Errores de coherencia de datos | 2 | 0 | 0 |
| Errores visuales/UX | 6 | 4 | 2 |
| Funcionalidad a eliminar/corregir | 3 | 0 | 0 |
| Riesgos arquitectónicos | 3 | 0 | 0 |

**Total de defectos confirmados con evidencia de código: 18**

---

## PARTE I: DEFECTOS FUNCIONALES CRÍTICOS

### 1.1 Contador de Tokens Muestra Valores Idénticos (In == Out)

| Atributo | Valor |
|----------|-------|
| **Severidad** | CRÍTICA |
| **Estado** | CONFIRMADO por ambos análisis |
| **Archivo** | `internal/tui/chat/model.go` |
| **Líneas** | 2306-2315 (renderHeader) |
| **Impacto** | Métricas de consumo incorrectas, afecta diagnóstico y estimación de costos |

#### Evidencia de Código

```go
// model.go líneas 2306-2315
var tokensIn, tokensOut int
for _, a := range m.agentInfos {
    tokensIn += a.TokensIn
    tokensOut += a.TokensOut
}
tokensIn += m.totalTokens / 2   // <-- BUG: División arbitraria
tokensOut += m.totalTokens / 2  // <-- BUG: Ambos reciben el mismo valor
```

#### Análisis Técnico Detallado

1. **Origen del problema:** `m.totalTokens` se calcula en `WorkflowCompletedMsg` como `TotalTokensIn + TotalTokensOut` (suma total).

2. **Comportamiento erróneo:** Al dividir ese total por 2 y sumarlo a ambos contadores, se fuerza artificialmente que `tokensIn == tokensOut` cuando `agentInfos` está vacío o tiene valores nulos.

3. **Ejemplo numérico:**
   - Si `TotalTokensIn = 400` y `TotalTokensOut = 190`
   - Entonces `m.totalTokens = 590`
   - El header mostrará: `tokensIn = 295`, `tokensOut = 295`
   - Esto contradice la métrica real (400 vs 190)

4. **Observación visual correlacionada:** La imagen `img.png` muestra `295-295`, lo cual es consistente con `590/2 = 295`.

#### Solución Requerida

```go
// Campos adicionales en Model struct
totalTokensIn  int
totalTokensOut int

// En handler de WorkflowCompletedMsg
m.totalTokensIn += msg.TotalTokensIn
m.totalTokensOut += msg.TotalTokensOut

// En renderHeader
tokensIn += m.totalTokensIn   // Usar valor real separado
tokensOut += m.totalTokensOut // Usar valor real separado
```

---

### 1.2 Inconsistencia entre Header y Panel de Logs (Tokens)

| Atributo | Valor |
|----------|-------|
| **Severidad** | ALTA |
| **Estado** | CONFIRMADO (Codex v2) |
| **Archivos** | `model.go` (header), `logs.go` (panel) |
| **Impacto** | Confusión del usuario, datos contradictorios en la misma pantalla |

#### Evidencia de Código

**Header (model.go):**
```go
// Usa m.totalTokens (workflow) + sumas por agente
tokensIn += m.totalTokens / 2
tokensOut += m.totalTokens / 2
```

**Panel de Logs (logs.go via updateLogsPanelTokenStats):**
```go
// Solo usa agentInfos, NO agrega m.totalTokens
for _, a := range m.agentInfos {
    // acumula solo tokens de agentes
}
```

#### Consecuencia Validada

El header puede mostrar un total que incluye tokens de workflow que el panel de logs nunca refleja. Esto genera inconsistencia visible en la misma pantalla: dos componentes muestran métricas diferentes para el mismo concepto.

---

### 1.3 Dropdown de Comandos: No Es Overlay, Altera el Layout

| Atributo | Valor |
|----------|-------|
| **Severidad** | ALTA |
| **Estado** | CONFIRMADO (Codex v2) |
| **Archivo** | `internal/tui/chat/model.go` |
| **Funciones** | `renderInlineSuggestions`, `recalculateLayout`, `renderMainContent` |
| **Impacto** | Experiencia de usuario inconsistente, el chat "salta" al abrir `/` |

#### Evidencia de Código

```go
// Comentario explícito en View()
// "Autocomplete suggestions are now rendered inline in renderMainContent, not as overlay"

// recalculateLayout suma suggestionsHeight a fixedHeight
fixedHeight += suggestionsHeight  // Reduce el viewport disponible
```

#### Análisis

1. `renderInlineSuggestions` se inserta en `renderMainContent` después del input
2. `recalculateLayout` suma `suggestionsHeight` a `fixedHeight`, reduciendo el viewport del chat
3. **Si el requisito de diseño es un overlay flotante**, el comportamiento actual lo incumple

#### Decisión Requerida

- **Opción A:** Aceptar el comportamiento inline como diseño final
- **Opción B:** Implementar un verdadero overlay con z-index visual

---

### 1.4 Hint `^5 stats` Sin Binding Implementado

| Atributo | Valor |
|----------|-------|
| **Severidad** | MEDIA |
| **Estado** | CONFIRMADO por ambos análisis |
| **Archivos** | `model.go:2575` (hint), `stats_widget.go:48,121-125` |
| **Impacto** | La UI sugiere una acción que no funciona |

#### Evidencia de Código

**Footer muestra el hint:**
```go
// model.go línea 2575
keyHintStyle.Render("^5") + labelStyle.Render(" stats"),
```

**Comentario promete funcionalidad:**
```go
// stats_widget.go línea 48
visible: false, // Hidden by default, toggle with Ctrl+5
```

**Handler inexistente:**
```go
// NO EXISTE ningún case para tea.KeyCtrl5 en handleKeyMsg
// StatsWidget.Toggle() existe (línea 121-125) pero nunca se llama
```

#### Solución Requerida

- **Opción A:** Implementar el handler `Ctrl+5` que llame a `StatsWidget.Toggle()`
- **Opción B:** Eliminar el hint `^5 stats` del footer si no se desea la funcionalidad

---

## PARTE II: DEFECTOS VISUALES CONFIRMADOS

### 2.1 Bloques Fantasma en Inline Code (Trailing Background)

| Atributo | Valor |
|----------|-------|
| **Severidad** | ALTA |
| **Estado** | CONFIRMADO con explicación refinada |
| **Archivos** | `model.go:1871-1876`, `glamour@v0.10.0/styles/dark.json` |
| **Causa raíz** | Estilo de Glamour con prefix/suffix con background |

#### Evidencia de Código y Módulo Externo

**Renderizado en model.go:**
```go
// Líneas 1871-1876
if m.mdRenderer != nil {
    if rendered, err := m.mdRenderer.Render(msg.Content); err == nil {
        content = strings.TrimSpace(rendered)  // Solo trim global, no interno
    }
}
sb.WriteString(agentMsgStyle.Render(content))
```

**Estilo de Glamour (dark.json):**
```json
{
  "code": {
    "prefix": " ",
    "suffix": " ",
    "background_color": "#3a3a3a"
  }
}
```

#### Análisis Técnico Refinado

1. **No es un problema de secuencias ANSI sin cerrar** (hipótesis v1 incorrecta)
2. **Es el estilo por defecto de Glamour:** El inline code tiene `prefix` y `suffix` definidos como espacios con `background_color`
3. `strings.TrimSpace()` solo elimina espacios al inicio/final del bloque completo, NO dentro de cada inline code
4. El "bloque fantasma" es el `suffix` (espacio con background) que queda visible al final del inline code

#### Soluciones Propuestas

**Opción A - Tema custom de Glamour (recomendada):**
```go
import "github.com/charmbracelet/glamour/ansi"

customStyle := glamour.DarkStyleConfig
customStyle.Code = ansi.StylePrimitive{
    Color: stringPtr("200"),
    // Sin prefix, suffix ni background
}

renderer, _ := glamour.NewTermRenderer(
    glamour.WithStyles(customStyle),
    glamour.WithWordWrap(dynamicWidth),
)
```

**Opción B - Post-procesamiento:**
Regex para eliminar trailing spaces antes del reset ANSI en bloques de código.

---

### 2.2 Sidebar Derecho Sin Borde Visible (Overflow)

| Atributo | Valor |
|----------|-------|
| **Severidad** | ALTA |
| **Estado** | CONFIRMADO con condiciones específicas |
| **Archivos** | `model.go:1954-1976`, `logs.go:465-469` |
| **Condición** | Terminales menores a ~107 columnas con ambos sidebars |

#### Evidencia de Código

**Cálculo de anchos en model.go:**
```go
// Líneas 1954-1976
if m.showLogs {
    if oneSidebarOpen {
        logsWidth = w * 2 / 5
    } else {
        logsWidth = w / 4
    }
    mainWidth -= logsWidth + 1  // +1 para separador
}
```

**Estilo del panel en logs.go:**
```go
// Líneas 465-469
boxStyle := lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).  // Borde consume 2 columnas
    BorderForeground(borderColor).
    Width(p.width).
    Height(p.height)
```

#### Análisis de Overflow

**Cálculo de ancho mínimo requerido:**
```
explorerMin = 20 + 1 (separador)
logsMin = 25 + 1 (separador)
mainMin = 40
bordes = 2 * 2 (izq + der de cada panel con borde)

Total mínimo con ambos sidebars ≈ 107 columnas
```

**Consecuencia:** Para terminales más estrechas, la suma excede el viewport y el borde derecho queda fuera de pantalla.

#### Solución Requerida

```go
// Normalización de anchos después del cálculo
totalUsed := mainWidth
if m.showExplorer {
    totalUsed += explorerWidth + 1
}
if m.showLogs {
    totalUsed += logsWidth + 1
}

if totalUsed > w {
    excess := totalUsed - w
    // Reducir mainWidth primero (tiene más margen)
    if mainWidth - excess >= 40 {
        mainWidth -= excess
    } else {
        // Reducir sidebars proporcionalmente
        reduction := excess / 2
        explorerWidth -= reduction
        logsWidth -= reduction
    }
}
```

---

### 2.3 Desfase de Fondos en Overlay de Shortcuts

| Atributo | Valor |
|----------|-------|
| **Severidad** | ALTA |
| **Estado** | CONFIRMADO parcialmente |
| **Archivo** | `internal/tui/chat/shortcuts.go:103-231` |
| **Elementos afectados** | Centrado de título, subrayados de columnas |

#### Evidencia de Código

**Centrado manual con offset hardcodeado:**
```go
// Línea 143
sb.WriteString(strings.Repeat(" ", (s.width-lipgloss.Width(title))/2-2))  // -2 hardcodeado
sb.WriteString(title)
```

**Subrayados de ancho fijo:**
```go
// Subrayado constante, no dinámico
strings.Repeat("─", 20)  // Ancho fijo de 20
```

**Padding calculado para teclas:**
```go
// Líneas 169-179
keyWidth := lipgloss.Width(key)
padding := 12 - keyWidth  // Asume ancho fijo de 12
if padding < 1 {
    padding = 1
}
```

#### Análisis

1. El código SÍ usa `lipgloss.Width()` para medir anchos (la hipótesis v1 de `len()` era incorrecta aquí)
2. **Problema real:** El `-2` hardcodeado en el centrado no contempla el padding del box
3. El boxStyle tiene `Padding(1, 2)` y `Width(s.width - 4)`, pero el centrado del contenido interno usa `s.width` directamente
4. Los subrayados de 20 caracteres fijos no coinciden con el ancho dinámico de las columnas

---

### 2.4 Dropdown de Comandos: Cálculo de Ancho Inconsistente

| Atributo | Valor |
|----------|-------|
| **Severidad** | MEDIA |
| **Estado** | CONFIRMADO (Codex v2) |
| **Archivo** | `internal/tui/chat/model.go` |
| **Función** | `renderInlineSuggestions` |

#### Evidencia de Código

```go
// dropdownWidth se clampa a [40..70]
dropdownWidth := clamp(width/3, 40, 70)

// PERO maxDescWidth se calcula con width (panel principal), no dropdownWidth
maxDescWidth := width - maxCmdWidth - 10

// La cabecera "Commands" va FUERA del box
headerLine + "\n" + boxStyle.Render(content)
```

#### Consecuencias

1. Las descripciones pueden exceder el box cuando `width` > `dropdownWidth`
2. La cabecera no comparte el mismo ancho/padding del box, generando desalineación visual

---

### 2.5 Dropdown: Highlight Solo en Columna de Comando

| Atributo | Valor |
|----------|-------|
| **Severidad** | BAJA |
| **Estado** | CONFIRMADO (Codex v2) |
| **Impacto** | Si el UX requiere highlight de fila completa, no se cumple |

#### Evidencia de Código

```go
// El estilo seleccionado aplica solo a " ▸ "+paddedCmd
selectedStyle.Render(" ▸ " + paddedCmd)

// La descripción se renderiza con descStyle SIN background
descStyle.Render(truncatedDesc)
```

---

### 2.6 WordWrap Fijo a 80 para Markdown

| Atributo | Valor |
|----------|-------|
| **Severidad** | MEDIA |
| **Estado** | CONFIRMADO (Codex v2) |
| **Archivo** | `model.go` (inicialización de renderer) |
| **Impacto** | Desalineación en viewports anchos o estrechos |

#### Evidencia de Código

```go
// El wordwrap es fijo, no depende del ancho del panel
glamour.WithWordWrap(80)
```

#### Consecuencia

El markdown rompe líneas a 80 columnas independientemente del ancho real del viewport, creando:
- Columnas irregulares en terminales anchas (>80)
- Wrapping prematuro o incorrecto en terminales estrechas (<80)

---

## PARTE III: RIESGOS ARQUITECTÓNICOS

### 3.1 Desfase entre recalculateLayout y View

| Atributo | Valor |
|----------|-------|
| **Severidad** | MEDIA |
| **Estado** | CONFIRMADO (Codex v2) |
| **Impacto** | Off-by-2 que causa recortes o columnas vacías |

#### Evidencia de Código

```go
// recalculateLayout
availableWidth := m.width - 2  // Resta 2

// View
mainWidth := w  // NO resta 2
```

#### Consecuencia

El viewport se calcula con un ancho distinto al usado para renderizar el panel principal. Esto genera un desfase de 2 columnas que puede producir recortes del borde derecho o columnas vacías según el tamaño de terminal.

---

### 3.2 Uso de `len()` en Cálculos de Ancho (Riesgo Unicode)

| Atributo | Valor |
|----------|-------|
| **Severidad** | MEDIA (potencial) |
| **Estado** | CONFIRMADO pero con matices |
| **Archivos** | `model.go`, `explorer.go` |
| **Impacto actual** | Ninguno con ASCII, pero riesgo latente |

#### Evidencia de Código

```go
// renderInlineSuggestions
cmdLen := len(m.suggestions[i]) + 1  // len() cuenta bytes

// calculateInputLines
// Calcula wraps con len(line)

// ExplorerPanel.formatEntry
name[:maxNameLen-3]  // Truncado por slicing de bytes
```

#### Análisis Crítico

1. `len()` cuenta BYTES, no caracteres visuales ni celdas de terminal
2. **Para ASCII puro:** `len()` == `runewidth.StringWidth()` == `lipgloss.Width()`
3. **Actualmente:** Los nombres de comandos en Quorum son ASCII puro (`clear`, `agent`, `help`)
4. **Riesgo:** Si se añaden comandos con Unicode (CJK, emoji), los cálculos serán incorrectos

#### Recomendación

Migrar a `lipgloss.Width()` en todos los cálculos de ancho para prevenir bugs futuros:

```go
// En lugar de
cmdLen := len(m.suggestions[i]) + 1

// Usar
cmdLen := lipgloss.Width(m.suggestions[i]) + 1
```

---

### 3.3 Propagación de WindowSizeMsg

| Atributo | Valor |
|----------|-------|
| **Severidad** | N/A |
| **Estado** | FUNCIONA CORRECTAMENTE |

#### Evidencia de Código

```go
// model.go línea 1034-1056
case tea.WindowSizeMsg:
    m.width = msg.Width
    m.height = msg.Height
    m.recalculateLayout()  // SÍ propaga a sub-componentes
```

```go
// recalculateLayout (líneas 1714-1819)
if m.showExplorer {
    m.explorerPanel.SetSize(explorerWidth, sidebarHeight)
}
if m.showLogs {
    m.logsPanel.SetSize(logsWidth, sidebarHeight)
}
```

#### Conclusión

La hipótesis v1 de que WindowSizeMsg no se propaga es **INCORRECTA**. El mensaje SÍ se maneja y SÍ propaga a los sub-componentes. El problema está en los **cálculos** de ancho, no en la propagación.

---

## PARTE IV: FUNCIONALIDADES A ELIMINAR O CORREGIR

### 4.1 Focus Mode (F11)

| Atributo | Valor |
|----------|-------|
| **Decisión** | ELIMINAR (según requisitos) |
| **Archivo** | `internal/tui/chat/model.go:664-687` |

#### Código a Eliminar

```go
// Handler principal (líneas 664-687)
case tea.KeyF11:
    m.focusMode = !m.focusMode
    if m.focusMode {
        m.preFocusState.showLogs = m.showLogs
        m.preFocusState.showExplorer = m.showExplorer
        m.showLogs = false
        m.showExplorer = false
    } else {
        m.showLogs = m.preFocusState.showLogs
        m.showExplorer = m.preFocusState.showExplorer
    }
```

#### Elementos Relacionados a Eliminar

| Ubicación | Descripción |
|-----------|-------------|
| `model.go:268-273` | Estado `focusMode` y `preFocusState` |
| `model.go:355` | Inicialización `focusMode: false` |
| `model.go:2521-2526` | Indicador `[F]` en footer |
| `model.go:2576` | Hint `F11 focus` en footer |
| `shortcuts.go:49` | Entrada `{Key: "F11", Description: "Focus/Zen mode"}` |

---

### 4.2 Hint `^5 stats`

| Atributo | Valor |
|----------|-------|
| **Decisión** | ELIMINAR hint O implementar handler |
| **Archivo** | `internal/tui/chat/model.go:2575` |

#### Si se decide eliminar

```go
// Eliminar esta línea del footer
keyHintStyle.Render("^5") + labelStyle.Render(" stats"),
```

#### Si se decide implementar

```go
// Añadir en handleKeyMsg
case tea.KeyCtrl5:
    m.statsWidget.Toggle()
    return m, nil
```

---

## PARTE V: HALLAZGOS NO CONFIRMADOS (DESCARTADOS)

Los siguientes hallazgos aparecen en análisis v1 pero **no tienen sustento en el código actual**:

| # | Hallazgo Reportado | Razón del Descarte |
|---|-------------------|-------------------|
| 1 | "Fondos desfasados" en títulos del overlay | `titleStyle` no define background en `shortcuts.go` |
| 2 | "Bloque fantasma" junto a título "Shortcuts" | No hay background definido en el estilo del título |
| 3 | "Separador tok: 295-295 usa guion de ancho distinto" | El código usa `->` (flecha), no guion |
| 4 | "Indicador `i` 10 more commands below" | El código imprime flechas `up/down`, no letra `i` |
| 5 | "Explorer desaparece al abrir `/`" | No hay lógica que cambie `showExplorer` al abrir suggestions |
| 6 | "Secuencias ANSI no cerradas" | Lip Gloss maneja resets correctamente |

---

## PARTE VI: CORRECCIONES EN ANÁLISIS V1

### Hipótesis Incorrectas Corregidas

| Hipótesis v1 | Corrección |
|--------------|------------|
| "`len()` vs `runewidth` causa problemas globales" | **Parcial:** El código ya usa `lipgloss.Width()` en muchos lugares; solo hay usos puntuales de `len()` |
| "Secuencias ANSI sin cerrar causan bloques fantasma" | **Incorrecto:** Es el estilo de Glamour (prefix/suffix con background) |
| "WindowSizeMsg no se propaga" | **Incorrecto:** Sí se propaga vía `recalculateLayout()` |
| "Panel Explorer desaparece con popup de comandos" | **Incorrecto:** El Explorer está presente pero vacío (sin archivos listados) |

---

## PARTE VII: MATRIZ DE PRIORIZACIÓN FINAL

### Prioridad P0 - Críticos (Corregir Inmediatamente)

| # | Defecto | Impacto | Esfuerzo |
|---|---------|---------|----------|
| 1 | Token counter In/Out idénticos | Métricas incorrectas | Bajo (5-10 líneas) |
| 2 | Inconsistencia header vs logs panel | Confusión de datos | Bajo |
| 3 | Bloques fantasma inline code | Visual roto | Medio |

### Prioridad P1 - Altos (Corregir Pronto)

| # | Defecto | Impacto | Esfuerzo |
|---|---------|---------|----------|
| 4 | Sidebar sin borde (overflow) | Visual roto en terminales estrechas | Medio |
| 5 | Desfase recalculateLayout vs View | Off-by-2 inconsistente | Bajo |
| 6 | Dropdown ancho inconsistente | Descripciones cortadas | Bajo |

### Prioridad P2 - Medios (Planificar)

| # | Defecto | Impacto | Esfuerzo |
|---|---------|---------|----------|
| 7 | Eliminar Focus Mode F11 | Limpieza de código | Bajo |
| 8 | Eliminar/implementar `^5 stats` | Consistencia UI | Bajo |
| 9 | Shortcuts overlay centrado | Visual menor | Bajo |
| 10 | WordWrap fijo 80 | Markdown desalineado | Medio |

### Prioridad P3 - Bajos (Cuando Sea Conveniente)

| # | Defecto | Impacto | Esfuerzo |
|---|---------|---------|----------|
| 11 | Migrar `len()` a `lipgloss.Width()` | Prevención futura | Bajo |
| 12 | Highlight fila completa dropdown | UX menor | Bajo |
| 13 | Dropdown inline vs overlay | Decisión de diseño | Alto |

---

## PARTE VIII: PLAN DE TESTING RECOMENDADO

### Dimensiones de Terminal a Probar

| Configuración | Columnas x Filas | Propósito |
|---------------|------------------|-----------|
| Mínima | 80x24 | Validar overflow y clamps |
| Común | 120x40 | Caso de uso típico |
| Widescreen | 200x60 | Validar stretch y wordwrap |
| Estrecha | 100x30 | Validar sidebars sin overflow |

### Matriz de Casos de Prueba

| Escenario | Explorer | Logs | Comandos | Esperado |
|-----------|----------|------|----------|----------|
| Base | OFF | OFF | OFF | Main panel completo |
| Solo Explorer | ON | OFF | OFF | Explorer + Main, bordes visibles |
| Solo Logs | OFF | ON | OFF | Main + Logs, bordes visibles |
| Ambos sidebars | ON | ON | OFF | Todos los bordes visibles |
| Popup comandos | - | - | ON | Sin alteración de layout (si overlay) |
| Resize dinámico | - | - | - | Transición fluida sin artefactos |

### Casos de Borde Específicos

1. **Token counter:** Verificar que In != Out cuando los valores reales son diferentes
2. **Inline code:** Verificar que no hay bloques con background al final
3. **Terminal 100 columnas + ambos sidebars:** Verificar que el borde derecho es visible
4. **Comandos con descripción larga:** Verificar truncado correcto dentro del box
5. **F11 eliminado:** Verificar que la tecla no tiene efecto
6. **Ctrl+5:** Verificar comportamiento consistente con hint

---

## PARTE IX: MAPA DE ARCHIVOS AFECTADOS

| Archivo | Defectos Relacionados | Líneas Clave |
|---------|----------------------|--------------|
| `model.go` | 1, 2, 3, 4, 5, 6, 7, 8, 10 | 664-687, 1871-1876, 1954-1976, 2306-2315, 2575 |
| `shortcuts.go` | 7, 9 | 49, 103-231, 143, 169-179 |
| `logs.go` | 2, 4 | 465-469 |
| `stats_widget.go` | 8 | 48, 121-125 |
| `explorer.go` | 11 | formatEntry |

---

## PARTE X: DEPENDENCIAS EXTERNAS RELEVANTES

```
github.com/charmbracelet/lipgloss     - Estilos TUI (usa runewidth internamente)
github.com/charmbracelet/glamour      - Markdown rendering (estilos con prefix/suffix)
github.com/charmbracelet/bubbletea    - Framework TUI (WindowSizeMsg)
github.com/mattn/go-runewidth v0.0.16 - Cálculo de ancho (indirecta via lipgloss)
```

**Nota:** Lip Gloss ya usa `go-runewidth` internamente para `lipgloss.Width()`. El proyecto debería usar esta función consistentemente en lugar de `len()`.

---

## CONCLUSIONES FINALES

### Hallazgos Principales

1. **Los problemas más sólidos no son "misterios de ANSI"** sino decisiones específicas de layout y estilos: token stats repartidos a mitades, dropdown inline con cálculos inconsistentes, wordwrap fijo y truncados por bytes.

2. **Los artefactos de inline code se explican completamente** por el estilo por defecto de Glamour (prefix/suffix con background). No hay evidencia de secuencias ANSI sin cerrar.

3. **El desfase entre `recalculateLayout` y `View`** es una fuente de bugs sutiles que debería unificarse.

4. **Varias observaciones de v1 no están sustentadas** por el código actual (fondos desfasados en shortcuts, guiones en tokens, indicador con letra `i`, desaparición del Explorer) y deben considerarse **no confirmadas** hasta reproducir con evidencia visual.

### Orden de Acción Sugerido

1. **Inmediato:** Corregir token counter (cambio de ~10 líneas, impacto alto)
2. **Pronto:** Normalizar cálculos de ancho para prevenir overflow
3. **Planificado:** Crear tema custom de Glamour para inline code
4. **Limpieza:** Eliminar Focus Mode y hints no funcionales

---

**Fin del Informe Consolidado Final v3**

*Generado mediante fusión crítica de análisis v2 (Claude Opus 4.5 + Codex GPT-5)*
*Validación cruzada contra código fuente y documentación oficial*
*Fecha: 2026-01-18*
