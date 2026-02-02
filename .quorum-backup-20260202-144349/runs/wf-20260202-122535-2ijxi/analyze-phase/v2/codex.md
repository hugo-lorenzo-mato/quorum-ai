# Análisis de viabilidad: gestión multi-tenant/multi-proyecto desde una única interfaz en Quorum AI

## 0) Contexto y objetivo
Se analiza la viabilidad técnica de gestionar múltiples proyectos/tenants de Quorum AI desde una sola WebUI con un selector en la esquina superior derecha. El análisis se fundamenta en el código actual (CLI, TUI, WebUI y backend), señalando acoplamientos a “un único proyecto”, riesgos, y cambios necesarios para habilitar multi‑proyecto.

---

## 1) Arquitectura actual (evidencia en código)

### 1.1 Configuración y estado (backend + CLI)
- La configuración por defecto se busca en el proyecto actual bajo `.quorum/config.yaml` (si existe) y, si no, en `.quorum.yaml` o en `~/.config/quorum`, lo que ancla la configuración al directorio de trabajo. (`internal/config/loader.go:75`, `internal/config/loader.go:76`, `internal/config/loader.go:81`, `internal/config/loader.go:86`, `internal/config/loader.go:88`)
- En el servidor, la ruta del archivo de configuración está fijada a `.quorum/config.yaml` sin un prefijo de proyecto. (`internal/api/config.go:360`, `internal/api/config.go:362`)
- El CLI expone `--config` pero por defecto usa `.quorum/config.yaml` (el texto del flag es explícito). (`cmd/quorum/cmd/root.go:56`, `cmd/quorum/cmd/root.go:58`)
- La inicialización de un proyecto (`quorum init`) crea `.quorum/` y subdirectorios (state/logs/runs) en el *cwd*. (`cmd/quorum/cmd/init.go:31`, `cmd/quorum/cmd/init.go:36`, `cmd/quorum/cmd/init.go:62`, `cmd/quorum/cmd/init.go:66`)
- El *state manager* se crea una sola vez en el servidor con una ruta derivada de la configuración; si está vacía, se usa `.quorum/state/state.json`. (`cmd/quorum/cmd/serve.go:97`, `cmd/quorum/cmd/serve.go:100`, `cmd/quorum/cmd/serve.go:102`)
- El *state manager* JSON usa un `active.json` por directorio base (`.quorum/state`) para el workflow activo. (`internal/adapters/state/json.go:41`, `internal/adapters/state/json.go:45`, `internal/adapters/state/json.go:204`, `internal/adapters/state/json.go:223`, `internal/adapters/state/json.go:239`)
- El esquema SQLite no incluye un `project_id`; el workflow “activo” está en una tabla singleton `active_workflow`. (`internal/adapters/state/migrations/001_initial_schema.sql:11`, `internal/adapters/state/migrations/001_initial_schema.sql:12`, `internal/adapters/state/migrations/001_initial_schema.sql:18`)

**Implicación:** la configuración y el estado están acoplados al directorio de trabajo; no existe un concepto de “proyecto” en el backend más allá del cwd.

### 1.2 Persistencia de artefactos en `.quorum`
- Los reportes de ejecución se guardan bajo `.quorum/runs`. (`internal/service/report/writer.go:15`, `internal/service/report/writer.go:24`)
- Los adjuntos se guardan bajo `<root>/.quorum/attachments`. (`internal/attachments/store.go:38`, `internal/attachments/store.go:39`)
- El servidor crea el chat store en `.quorum/chat` o `.quorum/chat.db` según backend. (`cmd/quorum/cmd/serve.go:117`, `cmd/quorum/cmd/serve.go:120`, `cmd/quorum/cmd/serve.go:122`)

**Implicación:** los artefactos persistentes son *per‑project* por ruta, no por un identificador lógico de tenant.

### 1.3 Backend/API y aislamiento actual
- El API server fija su `root` al `os.Getwd()` y lo usa para resolver paths. (`internal/api/server.go:132`, `internal/api/server.go:137`)
- El API de archivos valida que el path solicitado se mantenga bajo `root` (cwd), bloqueando escapes. (`internal/api/files.go:265`, `internal/api/files.go:267`, `internal/api/files.go:285`)
- El handler de chat asigna `projectRoot` al `os.Getwd()` al crear sesiones y lo persiste. (`internal/adapters/web/chat.go:222`, `internal/adapters/web/chat.go:223`, `internal/adapters/web/chat.go:240`, `internal/adapters/web/chat.go:252`)
- Para adjuntar archivos en chat se valida que estén bajo `projectRoot`, que por defecto es el cwd. (`internal/adapters/web/chat.go:786`, `internal/adapters/web/chat.go:791`, `internal/adapters/web/chat.go:800`, `internal/adapters/web/chat.go:813`)
- SSE se suscribe a *todos* los eventos del `EventBus`, sin filtro por proyecto. (`internal/api/sse.go:33`, `internal/api/sse.go:34`)
- El `EventBus` sólo modela `workflow_id`, no hay `project_id` en el evento base. (`internal/events/bus.go:11`, `internal/events/bus.go:22`, `internal/events/bus.go:31`)
- La capa HTTP no registra middleware de autenticación/autoría; sólo RequestID/RealIP/Recoverer/Timeout/CORS. (`internal/api/server.go:163`, `internal/api/server.go:166`, `internal/api/server.go:167`, `internal/api/server.go:171`)

**Implicación:** el servidor opera sobre un único root y expone endpoints sin diferenciación de proyecto; un multi‑tenant real requeriría un *scope* explícito por request.

### 1.4 WebUI (React + Zustand)
- Routing central en `App.jsx` con rutas para Dashboard, Workflows, Kanban, Chat y Settings. (`frontend/src/App.jsx:31`, `frontend/src/App.jsx:33`, `frontend/src/App.jsx:37`)
- El API base de frontend es `/api/v1`, sin parámetro de proyecto. (`frontend/src/lib/api.js:1`, `frontend/src/lib/api.js:4`)
- SSE usa `/api/v1/sse/events` y actualiza stores globales. (`frontend/src/hooks/useSSE.js:9`, `frontend/src/hooks/useSSE.js:42`)
- El estado de workflows es global (un array único) y no está segmentado por proyecto. (`frontend/src/stores/workflowStore.js:6`, `frontend/src/stores/workflowStore.js:8`)
- La configuración se carga/actualiza desde un único `/config/`. (`frontend/src/stores/configStore.js:67`, `frontend/src/stores/configStore.js:72`)
- El layout tiene un “slot” de acciones en la esquina superior derecha, actualmente vacío, donde se podría integrar un selector de proyecto. (`frontend/src/components/Layout.jsx:266`, `frontend/src/components/Layout.jsx:287`, `frontend/src/components/Layout.jsx:288`)

**Implicación:** la WebUI asume un único backend y un único conjunto de workflows/config.

### 1.5 CLI y TUI
- El CLI usa `.quorum/config.yaml` por defecto y permite `--config`, pero no hay concepto de “proyecto activo” a nivel de runtime; el proyecto se define por cwd/config. (`cmd/quorum/cmd/root.go:56`, `cmd/quorum/cmd/root.go:75`, `cmd/quorum/cmd/root.go:81`)
- En la TUI, el explorador usa `os.Getwd()` como raíz y prohíbe navegar por encima de ese directorio. (`internal/tui/chat/explorer.go:61`, `internal/tui/chat/explorer.go:63`, `internal/tui/chat/explorer.go:66`)
- El visor de archivos en TUI rechaza paths fuera del cwd. (`internal/tui/chat/file_viewer.go:47`, `internal/tui/chat/file_viewer.go:53`, `internal/tui/chat/file_viewer.go:56`)

**Implicación:** CLI/TUI están ligadas a un único proyecto por directorio; multi‑proyecto requeriría un mecanismo explícito de selección de root.

### 1.6 Diagrama de arquitectura actual

```
+-------------------+        SSE /api/v1/sse/events        +---------------------+
|  WebUI (React)    | <----------------------------------- |   API Server (cwd)  |
|  Zustand stores   | -----------------------------------> |  /api/v1/*          |
|  API_BASE=/api/v1 |   REST /api/v1/workflows, /config   |  root=os.Getwd()    |
+-------------------+                                      +----------+----------+
                                                                      |
                                                                      v
                                       .quorum/ (dentro del cwd actual)
                                       - config.yaml
                                       - state/state.db|state.json
                                       - runs/
                                       - attachments/
                                       - chat(.db)
```

Evidencia de rutas `.quorum`: `internal/config/loader.go:75`, `internal/config/loader.go:224`, `internal/service/report/writer.go:24`, `internal/attachments/store.go:39`, `cmd/quorum/cmd/serve.go:120`.

---

## 2) Diagnóstico de viabilidad para multi‑tenant (desde el estado actual)

### 2.1 Aislamiento de datos y riesgos
1) **Persistencia per‑project implícita por ruta**: todo lo persistente vive bajo `.quorum/` en el cwd (config, estado, reportes, adjuntos, chat). Esto funciona para *un* proyecto por servidor. Para multi‑tenant en un solo servidor, habría colisión de paths si no se introduce un `project_id` o separación por root. (ver rutas en `internal/config/loader.go:75`, `internal/api/config.go:360`, `internal/service/report/writer.go:24`, `internal/attachments/store.go:39`, `cmd/quorum/cmd/serve.go:120`)
2) **SSE sin namespace**: el EventBus no contiene `project_id`, y el SSE se suscribe a “todo”. Si el servidor gestionara múltiples proyectos, los clientes recibirían eventos mezclados. (`internal/events/bus.go:11`, `internal/events/bus.go:22`, `internal/api/sse.go:33`)
3) **Workflow activo global por storage**: tanto en JSON (`active.json`) como en SQLite (`active_workflow`) hay un único workflow activo por storage. En multi‑proyecto en un solo DB, esto colapsaría. (`internal/adapters/state/json.go:41`, `internal/adapters/state/json.go:223`, `internal/adapters/state/migrations/001_initial_schema.sql:11`)
4) **Acceso a archivos limitado al root**: el API de archivos y el chat restringen lectura a un root único (cwd). Esto evita escapes, pero también impide acceso a múltiples roots sin cambios. (`internal/api/files.go:265`, `internal/api/files.go:285`, `internal/adapters/web/chat.go:790`, `internal/adapters/web/chat.go:813`)
5) **Sin autenticación**: el router del API sólo instala middlewares de infraestructura (RequestID, RealIP, Recoverer, Timeout, CORS). Un modelo multi‑tenant sin auth expone riesgo de lectura cruzada. (`internal/api/server.go:163`, `internal/api/server.go:167`, `internal/api/server.go:171`)

### 2.2 Viabilidad “as‑is”
Con el código actual, **no es viable** un multi‑tenant real en un único proceso sin refactor porque:
- El root y los paths son globales por servidor (`internal/api/server.go:132`, `internal/api/files.go:265`).
- El state manager y el EventBus son únicos por servidor (`cmd/quorum/cmd/serve.go:97`, `internal/api/sse.go:33`).
- La WebUI no puede seleccionar proyecto ni direccionar requests a un tenant específico (`frontend/src/lib/api.js:1`, `frontend/src/hooks/useSSE.js:9`).

---

## 3) Matriz de pros y contras (multi‑tenant en WebUI)

| Aspecto | Beneficios concretos | Riesgos técnicos (con evidencia) | Probabilidad / Impacto | Complejidad |
|---|---|---|---|---|
| Productividad | Unificar visibilidad de workflows y reportes desde una sola UI. | Si se comparte EventBus sin namespace, se mezclan eventos entre proyectos. (`internal/api/sse.go:33`, `internal/events/bus.go:22`) | Alta / Alta | Media‑Alta |
| Visibilidad global | Panel único para priorizar trabajos del equipo. | “Workflow activo” es global por storage; colisiona si múltiples proyectos comparten DB. (`internal/adapters/state/migrations/001_initial_schema.sql:11`, `internal/adapters/state/json.go:223`) | Media / Alta | Alta |
| Coste operacional | Menos instancias de servidor si se hace multi‑tenant real. | El backend actual asume `root=os.Getwd()`; requiere re‑arquitectura de rutas y stores. (`internal/api/server.go:132`, `internal/api/files.go:265`) | Alta / Media | Alta |
| Seguridad | Aislamiento lógico por proyecto mejora el control de accesos (si se implementa). | No hay middleware de auth; multi‑tenant sin auth implica fuga cross‑tenant. (`internal/api/server.go:163`, `internal/api/server.go:171`) | Media / Alta | Alta |
| Rendimiento | Posible reutilización de procesos y cachés. | Cachés/handles en memoria no están namespaced por proyecto. (`internal/api/unified_tracker.go:86`, `internal/api/unified_tracker.go:93`) | Media / Media | Media |
| Mantenibilidad | Modelo explícito de “proyecto” aclara límites de datos. | Cambios atraviesan config, state, file API, chat, SSE, stores y rutas. (evidencia en secciones 1.1–1.4) | Alta / Media | Alta |

---

## 4) Dificultades principales (con módulos afectados y mitigaciones)

### D1) Contexto de proyecto en backend (config + state + root)
**Evidencia:** config fijo en `.quorum/config.yaml` y root del servidor es cwd. (`internal/api/config.go:360`, `internal/api/server.go:132`)

**Problema técnico:** todas las rutas y managers asumen un único root, por lo que no existe “tenant” lógico.

**Módulos afectados:**
- `internal/api/server.go` (root global) (`internal/api/server.go:132`)
- `internal/api/config.go` (ruta fija de config) (`internal/api/config.go:360`)
- `internal/adapters/state/*` (state por path/DB único) (`internal/adapters/state/json.go:41`, `internal/adapters/state/migrations/001_initial_schema.sql:18`)

**Mitigaciones:**
1) **ProjectContext explícito**: introducir un `ProjectID -> ProjectContext` que resuelva `root`, `configLoader`, `stateManager`, `attachmentsStore`, `eventBus`. Esto exige routing por proyecto (ver Alternativa B). *Trade‑off:* alta complejidad, pero aislamiento fuerte.
2) **Separación por procesos**: ejecutar un servidor por proyecto y cambiar el endpoint en frontend. *Trade‑off:* menos cambios backend, pero mayor carga operativa (ver Alternativa A).

### D2) Eventos y SSE sin namespace
**Evidencia:** SSE se suscribe a todos los eventos y `EventBus` no modela `project_id`. (`internal/api/sse.go:33`, `internal/events/bus.go:22`)

**Problema técnico:** en multi‑tenant, el cliente vería eventos de otros proyectos si el bus es compartido.

**Módulos afectados:** `internal/events/*`, `internal/api/sse.go`, `frontend/src/hooks/useSSE.js`.

**Mitigaciones:**
- Bus por proyecto (un `EventBus` por `ProjectContext`). *Trade‑off:* más memoria, pero separación limpia.
- Agregar `project_id` a `BaseEvent` y filtrar en SSE. *Trade‑off:* implica cambios transversales en eventos.

### D3) Persistencia de estado sin project_id
**Evidencia:** SQLite schema no tiene `project_id`, y existe `active_workflow` singleton. (`internal/adapters/state/migrations/001_initial_schema.sql:11`, `internal/adapters/state/migrations/001_initial_schema.sql:18`)

**Problema técnico:** un único DB para múltiples proyectos no puede separar workflows ni “activo”.

**Mitigaciones:**
- **DB por proyecto**: mantener el modelo actual y asignar una ruta de DB por root. *Trade‑off:* gestión de múltiples archivos.
- **ProjectID en esquema**: migraciones para añadir `project_id` a `workflows`, `tasks`, `checkpoints`, `active_workflow`. *Trade‑off:* migraciones complejas y cambios en consultas.

### D4) Acceso a archivos y adjuntos
**Evidencia:** `resolvePath` valida contra un único root; adjuntos viven bajo `<root>/.quorum/attachments`. (`internal/api/files.go:265`, `internal/api/files.go:285`, `internal/attachments/store.go:38`)

**Problema técnico:** multi‑tenant requiere múltiples raíces; el API actual sólo permite una.

**Mitigaciones:**
- Inyectar root por request (ProjectContext) y mantener validación de límites. *Trade‑off:* mayor complejidad de routing.
- Mantener server por proyecto (Alternativa A).

### D5) Estado global en WebUI
**Evidencia:** Zustand stores almacenan workflows/config globales y SSE único. (`frontend/src/stores/workflowStore.js:6`, `frontend/src/stores/configStore.js:67`, `frontend/src/hooks/useSSE.js:9`)

**Problema técnico:** al cambiar de proyecto, el store “mezcla” datos si no se resetea o segmenta.

**Mitigaciones:**
- Stores “por proyecto” (mapa `projectId -> state`) o reset al cambiar de tenant.
- API base dinámico y SSE por proyecto.

### D6) CLI/TUI y coherencia multi‑interfaz
**Evidencia:** TUI ancla root al cwd; CLI init/loader usa `.quorum` del cwd. (`internal/tui/chat/explorer.go:61`, `cmd/quorum/cmd/init.go:31`, `internal/config/loader.go:75`)

**Problema técnico:** multi‑proyecto desde una sola interfaz no se traslada automáticamente a CLI/TUI.

**Mitigaciones:**
- Agregar `--project-root` o `--project-id` para CLI/TUI y mantener compatibilidad con cwd.
- O limitar multi‑proyecto a WebUI inicialmente, documentando el alcance.

---

## 5) Alternativas arquitectónicas (mínimo 3)

### Alternativa A — Multi‑servidor + WebUI con “multi‑endpoint”
**Idea:** mantener “un servidor por proyecto” (como hoy) y permitir en WebUI elegir a qué servidor se conecta.

**Diagrama:**
```
[WebUI] --(Endpoint A)--> [Server A @ project A]
    |  --(Endpoint B)--> [Server B @ project B]
    |  --(Endpoint C)--> [Server C @ project C]
```

**Cambios en código:**
- WebUI: API_BASE y SSE_URL dinámicos por proyecto (modificar `frontend/src/lib/api.js`, `frontend/src/hooks/useSSE.js`).
- Stores: reset/segmentación por endpoint en `workflowStore` y `configStore`.
- UI: selector en Layout (slot ya existe). (`frontend/src/components/Layout.jsx:287`)

**Compatibilidad:**
- CLI/TUI: sin cambios.
- Backend: sin cambios.

**Pros:** mínimo refactor backend; menor riesgo. **Contras:** requiere múltiples procesos; no hay descubrimiento automático de proyectos.

### Alternativa B — Servidor multi‑proyecto con ProjectContext explícito (recomendada)
**Idea:** un único servidor gestiona múltiples roots con un `ProjectID` explícito en rutas o headers.

**Diagrama:**
```
           +------------------------+
           |  Project Registry      |
           |  {id -> root/path}     |
           +-----------+------------+
                       |
       /api/v1/projects/{id}/...
                       v
+--------------+   +--------------------+   +--------------------+
| API Router   |-->| ProjectContext     |-->| State/Config/Files  |
+--------------+   | root, stateMgr,    |   | .quorum per root    |
                   | eventBus, chat     |   +--------------------+
                   +--------------------+
```

**Cambios en código (alto impacto):**
- Backend: introducir `ProjectRegistry` y `ProjectContext` (root, config loader, state manager, event bus).
- Rutas: prefijar endpoints con `/projects/{projectID}`; SSE por proyecto.
- Eventos: añadir `project_id` o separar EventBus por proyecto.
- Persistencia: DB por proyecto o schema multi‑project.

**Compatibilidad:**
- WebUI: requiere adaptar API_BASE y stores a `projectID`.
- CLI/TUI: opcional (podría seguir funcionando con el proyecto “default”).

**Pros:** aislamiento correcto, escalable. **Contras:** refactor profundo y migraciones.

### Alternativa C — Servidor con “proyecto activo” global
**Idea:** API añade `/projects/active` para cambiar contexto global del servidor; las rutas actuales operan sobre el “activo”.

**Diagrama:**
```
[WebUI] -> POST /projects/active -> [Server cambia root global]
[WebUI] -> /api/v1/workflows (usa root activo)
```

**Pros:** cambios limitados en rutas existentes. **Contras:** incompatible con múltiples usuarios simultáneos; riesgo de datos cruzados si dos usuarios cambian el proyecto activo.

---

## 6) Recomendación
**Recomendación principal: Alternativa B (ProjectContext explícito).**

**Justificación técnica:**
- Es la única opción que elimina el acoplamiento a `cwd` y separa correctamente datos, eventos y archivos entre proyectos (ver D1–D4). (evidencias: `internal/api/server.go:132`, `internal/api/sse.go:33`, `internal/adapters/state/migrations/001_initial_schema.sql:11`)
- Evita el riesgo de contaminación de estado y eventos que tendría una “conmutación global” (Alternativa C).
- Permite evolucionar hacia autenticación/roles si en el futuro se requiere multi‑usuario.

**Consideración pragmática:** puede implementarse por fases (ver plan), manteniendo compatibilidad con rutas actuales mediante un `projectID` por defecto.

---

## 7) Plan de implementación conceptual (para Alternativa B)

### Fase 0 — Modelo de proyectos y registro
1) **Definir ProjectID** (hash del path o UUID). Guardar en un registro persistente (p.ej. `~/.config/quorum/projects.json`).
2) **API nueva**: `/api/v1/projects` (list/add/remove) + validación de que el root contiene `.quorum/`.
3) **Edge cases**: root inexistente, permisos insuficientes, `.quorum` corrupto.

### Fase 1 — ProjectContext en backend
1) Crear `ProjectContext` con: `root`, `configLoader`, `stateManager`, `chatStore`, `attachmentsStore`, `eventBus`.
2) Ajustar `internal/api/server.go` para resolver ProjectContext por request (p. ej. middleware que inyecta contexto).
3) Mantener compatibilidad: si no hay `projectID`, usar el cwd como “default”.

### Fase 2 — Persistencia y aislamiento
1) **Opción A (preferida): DB por proyecto**: usar rutas `.quorum/state/state.db` por root sin cambiar el esquema. (`internal/config/loader.go:224`)
2) **Opción B:** migraciones con `project_id` en tablas (más complejo, pero DB única).

### Fase 3 — SSE y eventos
1) **EventBus por proyecto** o añadir `project_id` a `BaseEvent`.
2) SSE endpoint por proyecto: `/api/v1/projects/{id}/sse/events`.
3) Ajustar `frontend/src/hooks/useSSE.js` a endpoint dinámico. (`frontend/src/hooks/useSSE.js:9`)

### Fase 4 — WebUI multi‑proyecto
1) **ProjectStore**: almacenar lista de proyectos y `currentProject`.
2) **API base dinámico**: construir URLs con `projectID`.
3) **Reset/segmentación**: limpiar `workflowStore` y `configStore` al cambiar de proyecto.
4) **UI**: selector en el top‑right del Layout (slot existente). (`frontend/src/components/Layout.jsx:287`)

### Fase 5 — CLI/TUI (opcional inicial)
1) Añadir flags `--project-root` o `--project-id` y mantener comportamiento actual por defecto. (`cmd/quorum/cmd/root.go:56`)
2) Para TUI, permitir seleccionar root inicial; mantener restricciones de seguridad. (`internal/tui/chat/explorer.go:61`)

### Riesgos de regresión y mitigaciones
- **Riesgo:** mezcla de datos entre proyectos al cambiar de contextos en stores. Mitigar con reset explícito y pruebas e2e.
- **Riesgo:** SSE cross‑tenant. Mitigar con eventos namespaced y tests de aislamiento.
- **Riesgo:** migraciones/paths. Mitigar con opción “DB por proyecto” en primera iteración.

---

## 8) Casos límite y restricciones
- **Proyecto corrupto**: falta `.quorum/config.yaml` o DB dañada → el registro debe marcar proyecto como “degradado” y mostrar diagnóstico. (ruta esperada: `internal/api/config.go:360`, `internal/config/loader.go:75`)
- **Permisos insuficientes**: file API y chat no deben permitir lectura fuera del root; mantener validaciones por proyecto. (`internal/api/files.go:285`, `internal/adapters/web/chat.go:813`)
- **Montajes remotos**: escaneos de proyectos deben ser explícitos y no recursivos por defecto para evitar latencia.
- **Concurrencia**: si dos usuarios usan el mismo servidor multi‑tenant, es indispensable separar event buses y estados por project.

---

## 9) Decisión rápida (árbol de decisión)

```
¿Quieres evitar cambios backend significativos?
 ├─ Sí → Alternativa A (multi‑servidor + selector de endpoint)
 └─ No → ¿Necesitas aislamiento fuerte y escalabilidad?
         ├─ Sí → Alternativa B (ProjectContext explícito)
         └─ No → Alternativa C (proyecto activo global) [no recomendado]
```

---

## 10) Conclusión
El diseño actual de Quorum AI está sólidamente orientado a **un proyecto por proceso** (root = cwd, `.quorum/` único, EventBus global). Con base en la evidencia del código, **la viabilidad de multi‑tenant en un único servidor requiere un rediseño explícito del contexto de proyecto**. La ruta más robusta es introducir `ProjectContext` y rutas con `projectID`, aunque conlleva una inversión considerable y cambios transversales. Una alternativa de bajo riesgo es una WebUI que gestione múltiples endpoints de servidores existentes.
