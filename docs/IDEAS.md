# Ideas y Prompts Pendientes

Este documento contiene ideas y prompts de análisis que se han planteado para el proyecto pero que aún no se han ejecutado o completado.

## 1. Interfaz Web (Completado parcialmente)

**Estado:** En desarrollo - Ya existe un frontend React básico

**Prompt original:**
```
Quiero habilitar para la aplicacion de este repositorio un servidor web que
permita acceder desde localhost en un puerto y poder dar una interfaz gráfica
al actual desarrollo que ya cuenta con CLI y TUI, pero no son herramientas muy
friendly fuera del sector de la informática. Analiza en profundidad tecnologías
recomendadas, modos de proceder, standares en la industria para estos casos,
manera más correcta de acoplar ese frontal con las funcionalidades que ofrece
el código de la herramienta. Quizá haya que repensar la organización o
arquitectura del proyecto para permitir este nuevo modo de interactuar.
```

**Notas de implementación previas:**
- Stack elegido: Go backend + React SPA embebida + SSE para streaming
- Rutas: chi router
- Frontend: React + Vite + TypeScript
- Embebido en binario con `//go:embed`

---

## 2. Persistencia SQLite

**Estado:** Pendiente de validación

**Prompt:**
```
Analiza en detalle la capa de persistencia a través de sqlite que ya hay
implementada y valida lo necesario y la mejor forma de hacerlo para que
comience a funcionar. Lo ideal sería que a través del fichero de configuración
se pudiera optar por sqlite o seguir con json. Por defecto seguir con json.
```

**Contexto:**
- Ya existe código para SQLite en el proyecto
- Configuración actual permite `state.backend: json` o `state.backend: sqlite`
- Necesita validación y testing

---

## 3. Sistema de Temas para TUI

**Estado:** Pendiente

**Prompt:**
```
Quiero implementar un sistema de temas para esta herramienta. Analiza en
profundidad las opciones disponibles para implementar temas visuales en una
aplicación TUI desarrollada con bubbletea/lipgloss.
```

---

## 4. Templates/Plantillas Visuales para TUI

**Estado:** Pendiente

**Prompt:**
```
¿Cómo podría añadir una serie de plantillas o templates visuales para el TUI
de esta app? Estaría genial aunque no es requisito si complica en exceso,
poder cambiar la template en tiempo real.
```

**Variantes consideradas:**
- Selección dinámica desde el propio TUI
- Cambio en tiempo real sin reiniciar

---

## 5. Mejoras Futuras del Frontend Web

**Estado:** Pendiente - Fase 2

**Características propuestas:**
- Múltiples chats/sesiones simultáneas
- Adjuntar imágenes y archivos
- Edición de configuración vía UI (no YAML)
- Visualización de logs y trazas
- Historial persistente
- Thumbnails para adjuntos

**Modelo de datos sugerido:**
```sql
chats(id, title, created_at, updated_at)
messages(id, chat_id, role, content, created_at, parent_id?, model?, token_count?)
attachments(id, chat_id, message_id?, path, mime, size, sha256, width?, height?, created_at)
flows(id, name, definition_json, created_at)
runs(id, flow_id?, chat_id?, status, started_at, finished_at, error?)
run_events(id, run_id, ts, type, payload_json)
settings(key, value_json, updated_at)
```

---

## Cómo usar estos prompts

Estos prompts pueden ejecutarse con quorum:

```bash
# Análisis
quorum analyze "contenido del prompt"

# O desde el TUI
quorum tui
# Luego /analyze <prompt>
```
