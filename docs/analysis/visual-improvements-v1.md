# Análisis de Mejoras Visuales y UX para Quorum AI WebUI

**Fecha:** 2 de Febrero de 2026
**Versión:** v1.0
**Autor:** Gemini CLI Agent

## 1. Resumen Ejecutivo

La interfaz actual de Quorum AI está construida sobre una base sólida utilizando React 18, Vite y Tailwind CSS v4. La arquitectura sigue patrones modernos (variables CSS para temas, componentes funcionales, hooks personalizados). El estilo visual imita la estética "Vercel/Shadcn" (minimalista, monocromático, bordes finos), lo cual es apropiado para una herramienta de desarrollo.

Sin embargo, para elevar la percepción de "producto profesional" y mejorar la usabilidad, se requiere una **sistematización** de los componentes, un refinamiento de las **micro-interacciones** y una mejor adaptación a **dispositivos móviles**.

Este documento detalla las áreas clave de mejora, sin proponer una reescritura total, sino una evolución iterativa del diseño actual.

---

## 2. Diagnóstico del Estado Actual

*   **Stack Tecnológico:** React 18, Tailwind CSS v4, Lucide React, Zustand.
*   **Estilo:** Minimalista, "Dark Mode" first (aunque soporta light), inspirado en interfaces de herramientas de desarrollo modernas.
*   **Puntos Fuertes:**
    *   Sistema de temas robusto basado en variables CSS (`--bg`, `--fg`).
    *   Tipografía adecuada (Inter para UI, JetBrains Mono para código).
    *   Estructura de navegación responsiva (Sidebar en escritorio, Bottom Nav en móvil).
*   **Puntos Débiles:**
    *   **Inconsistencia en "Utility Classes":** Uso repetitivo de clases Tailwind (ej. `bg-primary/10`) en lugar de componentes semánticos, lo que dificulta el mantenimiento visual global.
    *   **Feedback Visual Limitado:** Los estados de carga, error y éxito son funcionales pero básicos. Faltan transiciones suaves en elementos interactivos complejos.
    *   **Densidad de Información:** El Dashboard y las tablas pueden sentirse abrumadores o desordenados en pantallas medianas.
    *   **Componentes "Ad-hoc":** Muchos componentes de UI (Inputs, Cards) están definidos dentro de las páginas o carpetas de configuración, en lugar de ser una librería base compartida.

---

## 3. Propuesta de Mejoras Visuales y de UX

### 3.1. Sistematización del Diseño (Design System)

El objetivo es reducir la deuda técnica visual y asegurar consistencia.

*   **Abstracción de Primitivas de UI:**
    *   Crear una carpeta `src/components/ui` para componentes atómicos "ciegos" al negocio (Button, Input, Card, Badge, Switch).
    *   **Acción:** Mover `ToggleSetting`, `TextInputSetting` a componentes base `Switch` e `Input` que no dependan de la lógica de configuración, y luego componerlos.
*   **Estandarización de Colores Semánticos:**
    *   Actualmente se usa `bg-primary/10` o `text-success`. Se recomienda definir tokens de superficie más específicos en `index.css` o `tailwind.config.js` para evitar calcular opacidades manualmente en el JSX.
    *   *Ejemplo:* En lugar de `bg-error/10`, definir un `--bg-error-subtle` que se adapte mejor en modo oscuro vs claro.
*   **Sombras y Profundidad:**
    *   El diseño actual es muy plano ("flat"). Introducir sombras sutiles (`shadow-sm`, `shadow-md`) en elementos flotantes o tarjetas al hacer hover para mejorar la jerarquía visual (Z-index percibido).

### 3.2. Mejoras en Componentes Específicos

#### A. Dashboard (Bento Grid)
*   **Problema:** La mezcla de carrusel horizontal (snap-x) en móvil y grid en escritorio crea una experiencia desconectada.
*   **Mejora:**
    *   Unificar la visualización de métricas clave. En móvil, usar un grid de 2x2 para las estadísticas en lugar de forzar scroll horizontal si son pocas (4 items).
    *   Mejorar los "Sparklines" (gráficos de línea). Añadir un tooltip al pasar el mouse/dedo para ver el valor exacto del punto.
    *   Añadir textura o degradados sutiles a las tarjetas de "Active Workflow" para que destaquen más sobre las métricas pasivas.

#### B. Formularios y Configuración (Settings)
*   **Problema:** Los formularios son listas largas.
*   **Mejora:**
    *   **Agrupación Visual:** Usar el componente `SettingSection` con un fondo ligeramente distinto (`bg-muted/30`) para separar bloques lógicos claramente.
    *   **Validación en Tiempo Real:** Mejorar el `ValidationMessage` para que aparezca con una animación suave (`AnimatePresence` o transición CSS de altura) y no desplace el contenido bruscamente.
    *   **Inputs:** Añadir un estado de `focus-within` más pronunciado a los contenedores de inputs para guiar al usuario.

#### C. Navegación
*   **Desktop:** La sidebar colapsable es buena. Se puede mejorar añadiendo tooltips a los iconos cuando está colapsada (actualmente solo muestra el icono).
*   **Móvil:** La `MobileBottomNav` ocupa espacio valioso.
    *   *Sugerencia:* Implementar comportamiento "Hide on Scroll" (ocultar al hacer scroll hacia abajo, mostrar al subir) para maximizar el área de lectura en listas largas o chats.

### 3.3. Experiencia de Usuario (UX) y "Polish"

*   **Estados Vacíos (Empty States):**
    *   Reemplazar los textos simples por ilustraciones SVG monocromáticas o iconos grandes con un estilo visual coherente (ej. usando el color `muted-foreground`).
    *   Incluir siempre una acción clara (CTA) en el estado vacío (ej. "Crear primer flujo").
*   **Carga (Loading Skeletons):**
    *   El `LoadingSkeleton` actual es genérico. Crear esqueletos específicos para Tablas y Tarjetas Bento que imiten la estructura real del contenido para reducir la carga cognitiva al aparecer los datos.
*   **Animaciones (Motion Design):**
    *   Usar `framer-motion` (o mantener las clases CSS actuales pero refinarlas) para transiciones de página.
    *   Animar la entrada de elementos en listas (stagger effect) para que no aparezcan de golpe.

### 3.4. Accesibilidad (a11y)

*   **Contraste:** Verificar que los colores de texto `muted-foreground` sobre fondos oscuros cumplan con WCAG AA. A veces el gris sobre negro es difícil de leer.
*   **Teclado:** Asegurar que el foco (`focus-visible`) tenga un "ring" de alto contraste (ya parece estar implementado, pero verificar en todos los componentes personalizados como el `Toggle`).

### 3.5. Mejora Específica: Creación de Workflows (/workflows/new)

**Problema Detectado:**
Actualmente, al seleccionar el modo "Single Agent" (Monoagente) en el formulario de creación, las opciones de configuración (Modelo, Agente, Razonamiento) se despliegan verticalmente desplazando el resto del contenido hacia abajo ("layout shift"). Esto se percibe poco profesional y rompe el flujo visual, especialmente si el usuario cambia rápidamente entre modos. La anidación visual (`ml-7`) aunque lógica, se siente "apretada".

**Propuestas de Solución:**

1.  **Enfoque "Wizard" (Paso a Paso):**
    *   Dividir el proceso en 2 pasos claros:
        1.  **Definición:** Título y Prompt.
        2.  **Estrategia:** Selección de Modo (Multi vs Single) y sus configuraciones específicas, más Archivos adjuntos.
    *   Esto reduce la carga cognitiva y evita formularios kilométricos.

2.  **Tarjetas de Selección (Cards) en lugar de Radios:**
    *   Reemplazar los `input[type="radio"]` actuales por tarjetas grandes seleccionables ("Tiles") dispuestas en grid (especialmente en desktop).
    *   **Comportamiento:** Al hacer clic en la tarjeta "Single Agent", esta podría quedar seleccionada y mostrar sus opciones de configuración en un panel adyacente (desktop) o en un bloque dedicado debajo con una animación de altura suave (`height: auto` transition).

3.  **Transiciones Suaves:**
    *   Si se mantiene el diseño vertical, es imperativo usar animaciones de expansión (ej. `framer-motion` `AnimatePresence` o CSS `grid-template-rows` transition) para que el contenido no "salte" de golpe.

4.  **Separación de Contexto:**
    *   Sacar la configuración del agente fuera de la jerarquía visual del radio button.
    *   *Nuevo Flujo:*
        *   Bloque 1: Prompt (Lo más importante).
        *   Bloque 2: Selector de Modo (Tabs o Cards horizontales).
        *   Bloque 3 (Dinámico): Panel de configuración que cambia suavemente según el modo seleccionado en el Bloque 2.

---

## 4. Plan de Acción Recomendado

No se recomienda aplicar todo a la vez. El orden lógico de implementación sería:

1.  **Fase 1: Refactorización de UI Kit (Cimientos)**
    *   Extraer componentes base (`Button`, `Input`, `Card`, `Badge`) a `src/components/ui`.
    *   Estandarizar sus estilos usando `cva` (class-variance-authority) o mapas de clases simples para gestionar variantes (solid, outline, ghost).

2.  **Fase 2: Pulido del Dashboard (Impacto Visual)**
    *   Mejorar las tarjetas de métricas (gradientes sutiles, mejor tipografía).
    *   Refinar los gráficos (Sparklines).
    *   Mejorar la responsividad del Grid.

3.  **Fase 3: Mejora de Formularios (Usabilidad)**
    *   Aplicar los nuevos componentes de UI a la página de Settings.
    *   Mejorar el feedback de validación.

4.  **Fase 4: Micro-interacciones (Delight)**
    *   Añadir transiciones de entrada.
    *   Mejorar tooltips y estados de hover.

## 5. Referencias Visuales

*   **Estética:** Vercel Dashboard, Linear App.
*   **Sistema de Iconos:** Lucide (ya en uso, mantener).
*   **Tipografía:** Inter (UI), JetBrains Mono (Código/Logs).
*   **Paleta:** Zinc/Slate (neutros) + Un color de acento configurable (actualmente parece ser blanco/negro en modo oscuro, considerar permitir un color "Brand" como azul o violeta sutil).

---
*Fin del análisis.*
