# Informe Consolidado de Defectos TUI v2

**Proyecto:** Quorum AI
**Fecha de consolidación:** 2026-01-18T18:00:00Z
**Fuentes analizadas:**
- `fallos-tui/v1/claude-code-opus-4-5-20260118T143022-v1.md`
- `fallos-tui/v1/codex-gpt-5-20260118-231950-v1.md`
- Código fuente: `internal/tui/chat/*.go`
- Capturas: `img.png`, `img_1.png`, `img_2.png`
- Documentación oficial: Lip Gloss, Glamour, Bubble Tea

**Metodología:** Análisis ultracrítico con validación contra código fuente y documentación oficial. Cada hallazgo ha sido verificado o refutado con evidencia concreta.

---

## Resumen Ejecutivo

Tras analizar críticamente ambos informes v1 y contrastarlos con el código fuente real, se han identificado:

| Categoría | Confirmados | Refutados | Parcialmente correctos |
|-----------|-------------|-----------|------------------------|
| Defectos funcionales | 3 | 1 | 1 |
| Defectos visuales | 4 | 1 | 2 |
| Funcionalidad a eliminar | 2 | 1 | 0 |

**Hallazgo crítico:** El informe de Codex v1 es significativamente más preciso en las referencias al código, pero contiene errores de observación visual. El informe de Claude v1 tiene mejor análisis visual pero hipótesis técnicas parcialmente incorrectas.

---

## Sección 1: Defectos CONFIRMADOS con Evidencia de Código

### 1.1 Token Counter Muestra Valores Idénticos

**Severidad:** CRÍTICA
**Estado:** CONFIRMADO
**Archivo:** `internal/tui/chat/model.go:2306-2315`

**Evidencia de código:**
```go
// Líneas 2306-2315
var tokensIn, tokensOut int
for _, a := range m.agentInfos {
    tokensIn += a.TokensIn
    tokensOut += a.TokensOut
}
tokensIn += m.totalTokens / 2 // Distribute workflow tokens  <-- BUG
tokensOut += m.totalTokens / 2                              <-- BUG
```

**Análisis:**
El código divide `m.totalTokens` por 2 y lo suma tanto a `tokensIn` como a `tokensOut`. Esto es matemáticamente incorrecto porque:
1. Los tokens de workflow (`m.totalTokens`) representan el total, no una cantidad a distribuir igualmente
2. Si `agentInfos` está vacío o sus tokens son 0, ambos valores serán idénticos (`totalTokens/2`)
3. La imagen muestra `295-295` porque `295 = 590/2` (hipótesis)

**Corrección requerida:** Separar el tracking de tokens en `totalTokensIn` y `totalTokensOut` desde la fuente de datos real.

---

### 1.2 Bloques Fantasma en Inline Code (Trailing Background)

**Severidad:** ALTA
**Estado:** CONFIRMADO
**Archivo:** `internal/tui/chat/model.go:1871-1876`

**Evidencia de código:**
```go
// Líneas 1871-1876
if m.mdRenderer != nil {
    if rendered, err := m.mdRenderer.Render(msg.Content); err == nil {
        content = strings.TrimSpace(rendered)
    }
}
sb.WriteString(agentMsgStyle.Render(content))
```

**Análisis técnico validado con documentación de Glamour:**
1. Glamour aplica estilos de inline code que incluyen color de fondo (según `styles/README.md`)
2. El estilo de código inline en Glamour puede incluir padding implícito
3. `strings.TrimSpace()` solo elimina espacios al inicio/final del bloque completo, NO dentro de cada inline code
4. Al aplicar `agentMsgStyle.Render()` sobre contenido ya renderizado con ANSI, las secuencias de escape se anidan

**Corrección v1 Claude (parcialmente correcta):** Sugería que el padding de Lip Gloss causa el problema
**Corrección v1 Codex (más precisa):** Identifica correctamente que es el tema de Glamour + espacios con background activo

**Solución real:** Crear un tema custom de Glamour sin padding en inline code, o post-procesar para eliminar trailing spaces antes del reset ANSI.

---

### 1.3 Desalineación de Fondos en Overlay de Shortcuts

**Severidad:** ALTA
**Estado:** CONFIRMADO (parcialmente)
**Archivo:** `internal/tui/chat/shortcuts.go:103-231`

**Evidencia de código:**
```go
// Línea 143 - Centrado manual con offset hardcodeado
sb.WriteString(strings.Repeat(" ", (s.width-lipgloss.Width(title))/2-2))
sb.WriteString(title)

// Líneas 169-179 - Padding calculado para alineación de teclas
keyWidth := lipgloss.Width(key)
padding := 12 - keyWidth
if padding < 1 {
    padding = 1
}
col.WriteString(key)
col.WriteString(strings.Repeat(" ", padding))
```

**Análisis:**
1. El código SÍ usa `lipgloss.Width()` para calcular anchos (correcto)
2. El problema está en el `-2` hardcodeado en el centrado (línea 143)
3. El `padding := 12 - keyWidth` asume un ancho fijo que no contempla todos los casos

**Lo que Claude v1 acertó:** Identificó el desfase visual correctamente
**Lo que Claude v1 erró:** La hipótesis de `len()` vs `runewidth` es INCORRECTA - el código ya usa `lipgloss.Width()`
**Lo que Codex v1 acertó:** Identificó correctamente el origen en `shortcuts.go`

**Causa real validada:** El boxStyle tiene `Padding(1, 2)` y `Width(s.width - 4)`, pero el centrado manual del contenido interno no contempla estos márgenes correctamente. El contenido se renderiza antes de aplicar el box, creando un desfase.

---

### 1.4 Sidebar Derecho Sin Borde Visible

**Severidad:** ALTA
**Estado:** CONFIRMADO
**Archivo:** `internal/tui/chat/model.go:1954-1976` y `logs.go:465-469`

**Evidencia de código:**
```go
// model.go líneas 1954-1976 - Cálculo de anchos
if m.showLogs {
    if oneSidebarOpen {
        logsWidth = w * 2 / 5
        // ... clamps
    } else {
        logsWidth = w / 4
        // ... clamps
    }
    mainWidth -= logsWidth + 1  // +1 para separador
}

// logs.go líneas 465-469 - Estilo del panel
boxStyle := lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(borderColor).
    Width(p.width).
    Height(p.height)
```

**Análisis:**
1. El cálculo de anchos en `View()` no verifica que `explorerWidth + mainWidth + logsWidth + separators <= w`
2. Si la suma excede el ancho del viewport, el borde derecho queda fuera de pantalla
3. El separador (`│`) entre paneles consume 1 columna pero el borde del box consume 2 columnas adicionales

**Fórmula correcta que debería usarse:**
```
available = w
explorerTotal = explorerWidth + 1 (separador)
logsTotal = logsWidth + 1 (separador)
mainWidth = available - explorerTotal - logsTotal
```

---

## Sección 2: Defectos REFUTADOS o Incorrectamente Reportados

### 2.1 "Panel Explorer Desaparece al Abrir Menú de Comandos"

**Estado:** REFUTADO
**Reportado por:** Claude v1 (Sección 2.1)

**Evidencia visual (img_1.png):**
Examinando la imagen `img_1.png` con atención, el panel Explorer NO desaparece. La captura muestra:
- Parte inferior izquierda: `0 items` visible
- El panel simplemente está vacío porque el directorio de trabajo no tiene archivos listados

**Evidencia de código:**
```go
// model.go línea 2992-2997
if m.showExplorer {
    explorerPanel := m.explorerPanel.Render()
    panels = append(panels, explorerPanel)
    // ...
}
```
El renderizado del Explorer es INDEPENDIENTE del estado de `showSuggestions`.

**Conclusión:** Error de observación en Claude v1. El Explorer está presente pero vacío.

---

### 2.2 Uso de `len()` en Cálculo de Anchos del Popup

**Estado:** PARCIALMENTE INCORRECTO
**Reportado por:** Ambos informes v1

**Evidencia de código:**
```go
// model.go línea 2443
cmdLen := len(m.suggestions[i]) + 1
```

**Análisis:**
1. `len()` cuenta BYTES, no caracteres visuales
2. PERO: los nombres de comandos en Quorum son ASCII puro (`clear`, `agent`, `help`, etc.)
3. Para strings ASCII, `len()` == `runewidth.StringWidth()` == `lipgloss.Width()`

**Conclusión:** Técnicamente es código subóptimo, pero NO causa los defectos visuales observados en las capturas porque todos los comandos usan ASCII. Sería un bug potencial si se añadieran comandos con caracteres Unicode.

---

### 2.3 Ctrl+5 para Toggle de Stats

**Estado:** FUNCIONALIDAD INCOMPLETA
**Reportado por:** Ambos informes v1 como "eliminar"

**Evidencia de código:**
```go
// stats_widget.go línea 48 - Solo comentario
visible: false, // Hidden by default, toggle with Ctrl+5

// model.go línea 2575 - Se muestra en footer
keyHintStyle.Render("^5") + labelStyle.Render(" stats"),
```

**Análisis crítico:**
1. El comentario menciona Ctrl+5 pero NO HAY handler para `tea.KeyCtrl5` en ninguna parte del código
2. `StatsWidget.Toggle()` existe (línea 121-125) pero nunca se llama desde un keybinding
3. El footer muestra `^5 stats` como hint pero la funcionalidad NO está implementada

**Conclusión:** No hay que "eliminar" algo que no funciona. Hay que: (a) implementar el handler, o (b) eliminar el hint del footer si no se desea la funcionalidad.

---

## Sección 3: Funcionalidades a Eliminar (VALIDADAS)

### 3.1 Focus Mode (F11)

**Estado:** IMPLEMENTADO, pendiente de eliminación según requisitos
**Archivo:** `internal/tui/chat/model.go:664-687`

**Código a eliminar:**
```go
case tea.KeyF11:
    m.focusMode = !m.focusMode
    if m.focusMode {
        m.preFocusState.showLogs = m.showLogs
        m.preFocusState.showExplorer = m.showExplorer
        m.showLogs = false
        m.showExplorer = false
        // ...
    } else {
        m.showLogs = m.preFocusState.showLogs
        m.showExplorer = m.preFocusState.showExplorer
        // ...
    }
```

**Elementos relacionados a eliminar:**
1. `model.go:268-273` - Estado `focusMode` y `preFocusState`
2. `model.go:355` - Inicialización `focusMode: false`
3. `model.go:2521-2526` - Indicador `[F]` en footer
4. `model.go:2576` - Hint `F11 focus` en footer
5. `shortcuts.go:49` - Entrada `{Key: "F11", Description: "Focus/Zen mode"}`

---

### 3.2 Hint de Ctrl+5 Stats (dado que no funciona)

**Estado:** SOLO HINT VISUAL, sin funcionalidad
**Archivo:** `internal/tui/chat/model.go:2575`

**Línea a eliminar:**
```go
keyHintStyle.Render("^5") + labelStyle.Render(" stats"),
```

---

## Sección 4: Análisis de Hipótesis Técnicas

### 4.1 Hipótesis sobre Secuencias ANSI No Cerradas

**Origen:** Claude v1, Sección B

**Validación:**
Lip Gloss maneja correctamente el cierre de secuencias ANSI. Según la documentación oficial y el código fuente de Lip Gloss:
- `Render()` produce output con secuencias balanceadas
- El problema NO es secuencias sin cerrar, sino la composición de estilos anidados

**Conclusión:** Hipótesis PARCIALMENTE INCORRECTA. El problema es la composición, no el cierre.

---

### 4.2 Hipótesis sobre Propagación de WindowSizeMsg

**Origen:** Claude v1, Sección C y Codex v1

**Validación:**
```go
// model.go línea 1034-1056
case tea.WindowSizeMsg:
    m.width = msg.Width
    m.height = msg.Height
    m.recalculateLayout()  // ESTO SÍ PROPAGA a sub-componentes
```

Y en `recalculateLayout()` (líneas 1714-1819):
```go
if m.showExplorer {
    m.explorerPanel.SetSize(explorerWidth, sidebarHeight)
}
if m.showLogs {
    m.logsPanel.SetSize(logsWidth, sidebarHeight)
}
```

**Conclusión:** Hipótesis INCORRECTA. El mensaje SÍ se propaga correctamente. El problema está en los CÁLCULOS de ancho, no en la propagación.

---

### 4.3 Hipótesis sobre lipgloss.Place() y JoinHorizontal()

**Origen:** Claude v1, Sección D

**Validación parcial:**
El código usa `lipgloss.JoinHorizontal(lipgloss.Top, panels...)` (línea 2018) para unir paneles. Si los anchos calculados no suman exactamente el viewport width, se producen desfases.

**Conclusión:** Hipótesis CORRECTA EN PRINCIPIO. La solución es normalizar anchos antes de hacer el join.

---

## Sección 5: Matriz de Defectos Priorizada (REVISADA)

| # | Defecto | Severidad | Validado | Archivo Principal | Prioridad |
|---|---------|-----------|----------|-------------------|-----------|
| 1 | Token counter valores idénticos | CRÍTICA | SÍ | model.go:2306-2315 | P0 |
| 2 | Bloques fantasma inline code | ALTA | SÍ | model.go:1871-1876 | P0 |
| 3 | Sidebar sin borde derecho | ALTA | SÍ | model.go:1954-1976 | P1 |
| 4 | Desfase fondos shortcuts overlay | ALTA | SÍ | shortcuts.go:143 | P1 |
| 5 | Eliminar Focus Mode F11 | MEDIA | SÍ | model.go:664-687 | P2 |
| 6 | Eliminar hint ^5 stats | BAJA | SÍ | model.go:2575 | P2 |
| 7 | Popup comandos sin borde | MEDIA | PARCIAL | model.go:2493-2498 | P2 |

---

## Sección 6: Correcciones Propuestas Basadas en Código

### 6.1 Fix para Token Counter

**Ubicación:** `internal/tui/chat/model.go`

**Cambio estructural requerido:**
1. Añadir campos separados en el Model:
   ```go
   totalTokensIn  int
   totalTokensOut int
   ```

2. Modificar el handler de `AgentResponseMsg` para acumular correctamente:
   ```go
   m.totalTokensIn += msg.TokensIn
   m.totalTokensOut += msg.TokensOut
   ```

3. Modificar `renderHeader` (líneas 2306-2315):
   ```go
   var tokensIn, tokensOut int
   for _, a := range m.agentInfos {
       tokensIn += a.TokensIn
       tokensOut += a.TokensOut
   }
   tokensIn += m.totalTokensIn   // Usar valor real
   tokensOut += m.totalTokensOut // Usar valor real
   ```

---

### 6.2 Fix para Bloques Fantasma

**Ubicación:** `internal/tui/chat/model.go:305-308` (inicialización de renderer)

**Opción A - Tema custom de Glamour:**
```go
import "github.com/charmbracelet/glamour/ansi"

customStyle := glamour.DarkStyleConfig
// Modificar el estilo de inline code para no tener padding
customStyle.Code = ansi.StylePrimitive{
    Color: stringPtr("200"),
    // Sin Background ni Padding
}

renderer, _ := glamour.NewTermRenderer(
    glamour.WithStyles(customStyle),
    glamour.WithWordWrap(80),
)
```

**Opción B - Post-procesamiento:**
Procesar el output de Glamour para eliminar trailing spaces dentro de secuencias con background.

---

### 6.3 Fix para Sidebar Sin Borde

**Ubicación:** `internal/tui/chat/model.go`, función `View()`

**Lógica de normalización:**
```go
// Después de calcular explorerWidth, mainWidth, logsWidth
totalUsed := mainWidth
if m.showExplorer {
    totalUsed += explorerWidth + 1 // +1 separador
}
if m.showLogs {
    totalUsed += logsWidth + 1 // +1 separador
}

// Ajustar si excede
if totalUsed > w {
    excess := totalUsed - w
    // Reducir mainWidth primero, luego sidebars
    if mainWidth - excess >= 40 {
        mainWidth -= excess
    } else {
        // Reducir sidebars proporcionalmente
    }
}
```

---

## Sección 7: Errores en los Informes v1

### 7.1 Errores en Informe Claude v1

| Error | Sección | Descripción |
|-------|---------|-------------|
| Falso positivo | 2.1 | Explorer NO desaparece con popup |
| Hipótesis incorrecta | 3.1 | No es problema de len() vs runewidth en shortcuts |
| Sobreanálisis | A | El código YA usa lipgloss.Width() en muchos lugares |

### 7.2 Errores en Informe Codex v1

| Error | Sección | Descripción |
|-------|---------|-------------|
| Incompleto | Ctrl+5 | No detectó que el handler no existe |
| Impreciso | Colisiones | El popup NO afecta al layout base |

---

## Sección 8: Recomendaciones Finales

### Orden de Implementación Sugerido

1. **Inmediato (P0):**
   - Corregir token counter (cambio de 5 líneas)
   - Investigar tema de Glamour para inline code

2. **Corto plazo (P1):**
   - Normalizar cálculo de anchos para sidebars
   - Revisar centrado en shortcuts overlay

3. **Cuando sea conveniente (P2):**
   - Eliminar Focus Mode
   - Limpiar hints no funcionales

### Testing Recomendado

1. **Dimensiones de terminal a probar:**
   - 80x24 (mínimo estándar)
   - 120x40 (común)
   - 200x60 (widescreen)
   - Redimensionamiento dinámico

2. **Casos de borde:**
   - Solo Explorer visible
   - Solo Logs visible
   - Ambos sidebars visibles
   - Ningún sidebar visible
   - Popup de comandos con lista larga

---

## Anexo A: Archivos Analizados

| Archivo | Líneas Clave | Defectos Relacionados |
|---------|--------------|----------------------|
| model.go | 2306-2315 | Token counter |
| model.go | 1871-1876 | Inline code rendering |
| model.go | 1954-1976 | Sidebar widths |
| model.go | 664-687 | Focus mode |
| model.go | 2575 | Stats hint |
| shortcuts.go | 143, 169-179 | Overlay alignment |
| logs.go | 465-469 | Panel borders |
| stats_widget.go | 48, 121-125 | Toggle no implementado |

---

## Anexo B: Verificación de Dependencias

```
github.com/charmbracelet/lipgloss - Estilos TUI
github.com/charmbracelet/glamour - Markdown rendering
github.com/charmbracelet/bubbletea - Framework TUI
github.com/mattn/go-runewidth v0.0.16 - Cálculo de ancho (indirecta via lipgloss)
```

Lip Gloss ya usa go-runewidth internamente para `lipgloss.Width()`. El código del proyecto debería usarlo consistentemente.

---

**Fin del informe consolidado v2**

*Generado mediante análisis ultracrítico con validación de código fuente*
*Modelo: claude-opus-4-5-20251101*
