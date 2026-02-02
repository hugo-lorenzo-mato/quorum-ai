# Analisis de viabilidad multi-tenant/multi-proyecto en Quorum AI (basado en codigo)

## 0. Alcance y metodologia
- Hecho: El codigo actual asume un unico proyecto activo por proceso mediante rutas relativas a `.quorum` y uso de `os.Getwd()` como raiz de proyecto en multiples capas (config, API, TUI, chat, adjuntos). Evidencia en `internal/config/loader.go:54-90`, `internal/api/server.go:130-138`, `internal/api/files.go:265-290`, `internal/adapters/web/chat.go:222-241`, `cmd/quorum/cmd/run.go:170-176`, `internal/tui/chat/explorer.go:61-66`.
- Hecho: El analisis se basa unicamente en el repositorio local; no se asume ningun comportamiento externo. Las conclusiones marcadas como Inferencia se derivan de la combinacion de evidencias. (ver secciones por evidencia)
- Hipotesis (marcada): donde se proponen impactos o beneficios, se indican como Hipotesis y no como hechos.

## 1. Arquitectura actual (hechos con evidencia)

### 1.1 Vista general

Diagrama de alto nivel (CLI/TUI y WebUI comparten el mismo backend y state manager por proyecto):

```
+-----------+     +------------------+     +----------------------+
| CLI/TUI   | --> | workflow.Runner  | --> | StateManager (db/json)|
+-----------+     +------------------+     +----------------------+
     |                     |                       |
     v                     v                       v
 config.Loader        Output/EventBus          .quorum/state/*
```
Evidencia de componentes y wiring: `cmd/quorum/cmd/common.go:67-117`, `internal/service/workflow/runner.go:134-233`, `internal/core/ports.go:140-240`, `internal/adapters/state/factory.go:19-49`.

Diagrama de WebUI:

```
Browser (React) -> /api/v1 -> api.Server -> RunnerFactory -> Runner
        ^                           |
        |                           v
        +----- SSE /api/v1/sse ---- EventBus
```
Evidencia: `frontend/src/App.jsx:1-39`, `frontend/src/lib/api.js:1-33`, `internal/api/server.go:184-208`, `internal/api/runner_factory.go:52-124`, `internal/api/sse.go:18-61`.

### 1.2 Configuracion y estado del proyecto
- Hecho: La configuracion se carga por defecto desde `.quorum/config.yaml` en el directorio actual; si no existe, usa `.quorum.yaml` legado y luego `~/.config/quorum`. Esto fija el concepto de proyecto al cwd. (`internal/config/loader.go:54-90`)
- Hecho: La ruta por defecto del estado es relativa: `.quorum/state/state.db` y backup `.quorum/state/state.db.bak`. (`internal/config/loader.go:223-226`)
- Hecho: `quorum init` crea `.quorum/` y subdirectorios en el directorio actual (state, logs, runs). Esto fija el proyecto a la carpeta actual. (`cmd/quorum/cmd/init.go:30-73`)
- Hecho: En la API, el path de configuracion esta hardcodeado a `.quorum/config.yaml` (no parametrizable por request). (`internal/api/config.go:360-363`)
- Hecho: El servidor web inicializa el state manager con la ruta del config (o fallback), y crea el chat store bajo `.quorum/`. (`cmd/quorum/cmd/serve.go:97-130`)

### 1.3 Inicializacion y persistencia de un workflow (quorum)
- Hecho: En WebUI/API, un workflow se crea en `handleCreateWorkflow`, se construye `WorkflowState` y se persiste via `stateManager.Save`. (`internal/api/workflows.go:258-375`)
- Hecho: En CLI, la inicializacion de estado para un workflow nuevo ocurre en `InitializeWorkflowState`, con `WorkflowState` y `WorkflowID` nuevo. (`cmd/quorum/cmd/common.go:399-415`)
- Hecho: En JSON backend, el `Save` setea el workflow activo via `SetActiveWorkflowID` y guarda `active.json` en `.quorum/state/`. (`internal/adapters/state/json.go:79-135`, `internal/adapters/state/json.go:198-240`)
- Hecho: En SQLite, el workflow activo se guarda en tabla singleton `active_workflow` con `id = 1`. (`internal/adapters/state/migrations/001_initial_schema.sql:11-16`, `internal/adapters/state/sqlite.go:847-1001`)
- Hecho: El esquema de workflows y tasks no tiene columna de tenant/proyecto. (`internal/adapters/state/migrations/001_initial_schema.sql:18-60`)

### 1.4 WebUI (React) y gestion de estado
- Hecho: El routing no incluye ningun parametro de proyecto (rutas planas). (`frontend/src/App.jsx:1-39`)
- Hecho: El store principal de workflows (Zustand) guarda `activeWorkflow` y `selectedWorkflowId` sin namespace de proyecto. (`frontend/src/stores/workflowStore.js:6-110`)
- Hecho: El store de configuracion es global y no esta aislado por proyecto. (`frontend/src/stores/configStore.js:26-120`)
- Hecho: SSE se conecta a un endpoint fijo `/api/v1/sse/events` y no acepta selector de proyecto. (`frontend/src/hooks/useSSE.js:9-40`)
- Hecho: La base API esta fija a `/api/v1`. (`frontend/src/lib/api.js:1-33`)
- Hecho: El layout tiene un placeholder de acciones en la barra superior (posible punto para selector de proyecto), pero no existe UI para ello. (`frontend/src/components/Layout.jsx:266-289`)

### 1.5 CLI y TUI
- Hecho: CLI usa config loader con Viper y lectura por defecto de `.quorum` en el cwd. (`cmd/quorum/cmd/root.go:56-83`, `cmd/quorum/cmd/common.go:67-117`)
- Hecho: CLI crea state manager desde la ruta de config (por defecto `.quorum/state/state.json`). (`cmd/quorum/cmd/run.go:195-213`, `cmd/quorum/cmd/common.go:88-117`)
- Hecho: CLI/TUI usa `os.Getwd()` para crear worktrees y detectar repo root (acople al proyecto actual). (`cmd/quorum/cmd/common.go:237-258`)
- Hecho: TUI persiste estado UI en `.quorum/ui-state.json` via `NewWithStateManager`. (`cmd/quorum/cmd/run.go:170-176`, `internal/tui/state.go:50-121`)
- Hecho: El explorador de archivos del TUI fija `initialRoot` al cwd y no permite navegar fuera. (`internal/tui/chat/explorer.go:61-66`, `internal/tui/chat/explorer.go:196-200`)
- Hecho: TUI autoload del workflow activo usa `ListWorkflows` y `IsActive`, por lo que asume un solo proyecto por proceso. (`internal/tui/chat/model.go:535-569`)

### 1.6 Backend/API, archivos y adjuntos
- Hecho: `api.Server` fija `root` a `os.Getwd()` si no se especifica, y esa raiz se usa para operaciones de archivos. (`internal/api/server.go:130-138`, `internal/api/files.go:265-290`)
- Hecho: El API de archivos restringe paths al directorio raiz (`resolvePath`), sin posibilidad de elegir otro root por request. (`internal/api/files.go:265-290`)
- Hecho: El store de adjuntos usa `.quorum/attachments` bajo la raiz del servidor. (`internal/attachments/store.go:38-41`)
- Hecho: El chat web guarda `projectRoot` al crear sesion usando `os.Getwd()`. (`internal/adapters/web/chat.go:222-241`)
- Hecho: El esquema de chat incluye `project_root`, pero no existe un selector de proyecto en rutas. (`internal/adapters/chat/migrations/001_initial_schema.sql:2-9`, `internal/api/server.go:185-253`)
- Hecho: SSE suscribe a *todos* los eventos del EventBus, sin filtro por proyecto. (`internal/api/sse.go:33-61`)
- Hecho: El EventBus es uno por proceso en `serve`. (`cmd/quorum/cmd/serve.go:157-159`)

## 2. Aislamiento de datos: estado actual

### 2.1 Hechos
- Hecho: Los paths por defecto de estado, reportes, adjuntos y trazas son relativos a `.quorum/` y por tanto al cwd. (`internal/config/loader.go:151-256`, `internal/service/report/writer.go:15-28`, `internal/attachments/store.go:38-41`)
- Hecho: El workflow activo es un singleton por state store (archivo `active.json` o tabla `active_workflow` con id=1). (`internal/adapters/state/json.go:198-240`, `internal/adapters/state/migrations/001_initial_schema.sql:11-16`, `internal/adapters/state/sqlite.go:847-1001`)
- Hecho: No hay columna `project_id`/`tenant_id` en el esquema de `workflows`/`tasks` de SQLite. (`internal/adapters/state/migrations/001_initial_schema.sql:18-60`)
- Hecho: La gestion de archivos y adjuntos se restringe a una sola raiz (`s.root` o cwd). (`internal/api/files.go:265-290`, `internal/attachments/store.go:38-41`)

### 2.2 Inferencias (marcadas)
- Inferencia: Si se intenta servir multiples proyectos desde un solo proceso sin separar state managers por raiz, el `active_workflow` y los locks se colisionaran (singleton global). Evidencia base del singleton: `internal/adapters/state/migrations/001_initial_schema.sql:11-16`, `internal/adapters/state/sqlite.go:847-1001`.
- Inferencia: SSE mezclaria eventos de proyectos distintos al compartir un solo EventBus. Evidencia base: `internal/api/sse.go:33-61`, `cmd/quorum/cmd/serve.go:157-159`.
- Inferencia: El rate limiting (registry global) aplicaria limites cruzados entre proyectos si conviven en el mismo proceso. Evidencia base: `internal/service/ratelimit.go:149-163`.

## 3. Viabilidad tecnica de multi-tenant desde una unica interfaz

### 3.1 Viable, pero con refactor transversal
- Hecho: La arquitectura actual esta fuertemente acoplada a un solo proyecto por proceso mediante cwd y rutas relativas (`.quorum`). (`internal/config/loader.go:54-90`, `internal/api/server.go:130-138`, `internal/tui/chat/explorer.go:61-66`)
- Inferencia: Para multi-tenant real en un solo backend, se requiere introducir un `ProjectContext` (root, config, state, chat, attachments, event bus) y propagarlo por API, Runner, y UI. Esto implica cambios en varias capas. Evidencia base del acople por cwd y rutas fijas: `internal/api/files.go:265-290`, `internal/adapters/web/chat.go:222-241`, `cmd/quorum/cmd/run.go:170-176`.
- Hecho: Ya existe soporte de multiples workflows dentro de un mismo proyecto (tabla `running_workflows`, locks por workflow). (`internal/adapters/state/migrations/006_workflow_isolation.sql:26-49`, `internal/core/ports.go:172-197`)
- Inferencia: Ese soporte no resuelve multi-proyecto, pero reduce el cambio necesario en el nivel workflow; el mayor reto esta en el nivel proyecto (paths, config, state, EventBus). Evidencia base: ausencia de tenant en schema y uso de cwd (`internal/adapters/state/migrations/001_initial_schema.sql:18-60`, `internal/api/server.go:130-138`).

## 4. Matriz de pros y contras (con probabilidad/impacto)

**Nota**: Beneficios son Hipotesis de producto; Riesgos se apoyan en hechos del codigo.

| Aspecto | Beneficio (Hipotesis) | Riesgo tecnico (Hecho/Inferencia) | Prob. | Impacto | Evidencia |
|---|---|---|---|---|---|
| UX unificada | Menos friccion al cambiar de proyecto desde un solo UI | Mezcla de eventos SSE y estado si no se separa EventBus/State | Alta | Alta | `internal/api/sse.go:33-61`, `internal/adapters/state/sqlite.go:847-1001` |
| Productividad | Comparar workflows entre proyectos en una sola vista | Stores UI no estan namespaced, riesgo de contaminar estado UI | Alta | Media | `frontend/src/stores/workflowStore.js:6-110`, `frontend/src/stores/configStore.js:26-120` |
| Observabilidad | Ver historiales globales | DB schema no tiene `project_id`; migracion necesaria si se centraliza | Media | Alta | `internal/adapters/state/migrations/001_initial_schema.sql:18-60` |
| Operacion | Un solo proceso servidor | Muchos componentes usan cwd/root fijo; refactor transversal | Alta | Alta | `internal/api/server.go:130-138`, `internal/adapters/web/chat.go:222-241` |
| Seguridad | Control de accesos por proyecto (Hipotesis) | API de archivos y adjuntos solo valida una raiz global | Alta | Alta | `internal/api/files.go:265-290`, `internal/attachments/store.go:38-41` |

## 5. Dificultades principales (detalle tecnico)

### D1. Raiz de proyecto acoplada a cwd en multiples capas
- Descripcion: Muchas rutas y validaciones usan `os.Getwd()` y `.quorum` relativo, lo que implica un unico proyecto por proceso. Esto aparece en API (root), chat, TUI, y worktrees. (`internal/api/server.go:130-138`, `internal/adapters/web/chat.go:222-241`, `internal/tui/chat/explorer.go:61-66`, `cmd/quorum/cmd/common.go:237-258`)
- Archivos afectados (no exhaustivo): `internal/api/server.go`, `internal/api/files.go`, `internal/adapters/web/chat.go`, `internal/attachments/store.go`, `internal/service/workflow/attachments_context.go`, `internal/tui/chat/explorer.go`, `cmd/quorum/cmd/common.go`.
- Posibles soluciones:
  1) Introducir `ProjectContext{Root, ConfigPath, StatePath}` y pasarla por API/Runner/Stores. Trade-off: refactor amplio pero mantiene datos por proyecto en `.quorum`. (Evidencia de dependencias actuales por cwd en refs anteriores)
  2) Mantener un proceso por proyecto y un UI agregador que seleccione backend (menos refactor). Trade-off: operacion mas compleja (multiple procesos).

### D2. Singleton de workflow activo y locks globales
- Descripcion: El estado activo es singleton (`active_workflow` con id=1 o `active.json`) y los locks son globales por state store. Esto rompe multi-proyecto en un solo store. (`internal/adapters/state/migrations/001_initial_schema.sql:11-16`, `internal/adapters/state/json.go:198-240`, `internal/adapters/state/sqlite.go:847-1001`)
- Archivos afectados: `internal/adapters/state/*`, `internal/core/ports.go`.
- Posibles soluciones:
  1) Un state manager por proyecto (separar DB/JSON). Trade-off: mas instancias, pero sin migracion de schema.
  2) Migrar schema para incluir `project_id` y eliminar singleton `active_workflow`. Trade-off: migracion compleja y cambios en queries.

### D3. API y SSE sin namespace de proyecto
- Descripcion: Las rutas no incluyen proyecto y SSE suscribe a todos los eventos del bus. (`internal/api/server.go:184-208`, `internal/api/sse.go:33-61`)
- Archivos afectados: `internal/api/server.go`, `internal/api/sse.go`, `frontend/src/hooks/useSSE.js`.
- Posibles soluciones:
  1) Rutas con `/api/v1/projects/{id}/...` y SSE por proyecto. Trade-off: cambios en frontend y backend.
  2) Header `X-Quorum-Project` y bus por proyecto. Trade-off: mas dificil de depurar y cachear en frontend.

### D4. Stores de frontend sin aislamiento
- Descripcion: Zustand guarda estado global (workflows, config, chat) sin key de proyecto. (`frontend/src/stores/workflowStore.js:6-110`, `frontend/src/stores/configStore.js:26-120`, `frontend/src/stores/chatStore.js:5-120`)
- Archivos afectados: `frontend/src/stores/*`, `frontend/src/hooks/useSSE.js`.
- Posibles soluciones:
  1) Namespacing por `projectId` en stores, con cache por proyecto y reset al cambiar. Trade-off: mas memoria y complejidad.
  2) Reiniciar stores al cambiar proyecto (simpler). Trade-off: pierde cache y UX.

### D5. API de archivos y adjuntos con raiz unica
- Descripcion: `resolvePath` valida contra una sola raiz (s.root), y el store de adjuntos usa `.quorum/attachments` bajo esa raiz. (`internal/api/files.go:265-290`, `internal/attachments/store.go:38-41`)
- Archivos afectados: `internal/api/files.go`, `internal/attachments/store.go`, `internal/adapters/web/chat.go`.
- Posibles soluciones:
  1) Resolver root por proyecto y validar paths por request. Trade-off: requiere context middleware por proyecto.
  2) Mantener un servidor por proyecto. Trade-off: mas procesos.

### D6. Configuracion fija en la API
- Descripcion: `getConfigPath()` devuelve `.quorum/config.yaml` fijo; no usa `--config` ni projectId. (`internal/api/config.go:360-363`)
- Archivos afectados: `internal/api/config.go`.
- Posibles soluciones:
  1) `getConfigPath(projectId)` basado en registry. Trade-off: cambios en API y frontend.

### D7. Recursos compartidos globales (rate limiter)
- Descripcion: `GetGlobalRateLimiter` retorna un singleton global en el proceso. (`internal/service/ratelimit.go:149-163`)
- Archivos afectados: `internal/service/ratelimit.go`, `cmd/quorum/cmd/common.go:231-235`.
- Posibles soluciones:
  1) RateLimiter por proyecto (instancia por ProjectContext). Trade-off: mas memoria, configuracion por proyecto.

## 6. Alternativas arquitectonicas (min 3)

### Alternativa A: Multi-backend por proyecto + UI agregador
**Descripcion**: Cada proyecto corre su propio `quorum serve` en su carpeta. El frontend agrega una lista de endpoints y cambia la base API/SSE segun seleccion. No requiere refactor grande del backend.

Diagrama:
```
UI (unica) -> [Proyecto A API]  (process A, root A)
           -> [Proyecto B API]  (process B, root B)
```

Cambios requeridos:
- Frontend: agregar store de proyectos (lista de endpoints), selector en top bar, y reconexion SSE por endpoint. (`frontend/src/components/Layout.jsx:266-289`, `frontend/src/hooks/useSSE.js:9-40`, `frontend/src/lib/api.js:1-33`)
- Backend: opcional, endpoint de metadata `/api/v1/project` para mostrar nombre/root (no existe hoy). (Inferencia)

Compatibilidad:
- CLI/TUI: sin cambios (cada proyecto se maneja en su cwd). (`cmd/quorum/cmd/root.go:56-83`)
- WebUI: requiere cambios moderados.

Pros:
- Menor impacto en backend (Hecho: acople fuerte al cwd). (`internal/api/server.go:130-138`)
- Aislamiento fuerte por proceso/FS.

Contras:
- Operacion mas compleja (muchos procesos/puertos). (Inferencia)
- Sin descubrimiento automatico; requiere registro manual en UI. (Inferencia)

### Alternativa B: Un solo backend multi-proyecto con ProjectRegistry
**Descripcion**: Un proceso `quorum serve` gestiona multiples `ProjectContext` con raiz, config, state, chat y EventBus por proyecto. Las rutas API incluyen `projectId` o header para seleccionar contexto.

Diagrama:
```
UI -> /api/v1/projects/{id}/... -> ProjectContext(id)
                                  |-> StateManager(.quorum de ese root)
                                  |-> ChatStore(.quorum de ese root)
                                  |-> EventBus (por proyecto)
```

Cambios requeridos (codigo):
- API router y handlers con `projectId` (hoy no existe). (`internal/api/server.go:184-208`)
- Resolver root por proyecto en `files.go` y `attachments` store. (`internal/api/files.go:265-290`, `internal/attachments/store.go:38-41`)
- Config path por proyecto (hoy fijo). (`internal/api/config.go:360-363`)
- EventBus por proyecto o eventos con `projectId` + filtro SSE. (`internal/api/sse.go:33-61`)
- Frontend stores namespaced y SSE segun proyecto. (`frontend/src/stores/workflowStore.js:6-110`, `frontend/src/hooks/useSSE.js:9-40`)

Compatibilidad:
- CLI/TUI: podria mantenerse single-proyecto; opcional `--project` para usar registry. (`cmd/quorum/cmd/root.go:56-83`)
- WebUI: requiere cambios altos.

Pros:
- Un solo servidor, UX coherente.
- Reutiliza `.quorum` por proyecto sin migrar schema (si se instancia StateManager por proyecto).

Contras:
- Refactor transversal y riesgo alto.
- Necesita registry central (archivo en `~/.config/quorum/` o similar). (Inferencia)

### Alternativa C: Centralizar todo en un solo SQLite multi-tenant
**Descripcion**: Un unico state store (por ejemplo `~/.config/quorum/state.db`) con columnas `project_id` en todas las tablas. Files/adjuntos se guardan en una raiz global con subcarpetas por proyecto.

Diagrama:
```
UI -> /api/v1/projects/{id} -> StateDB(global, project_id)
                             -> Storage ~/.config/quorum/projects/{id}/...
```

Cambios requeridos:
- Migraciones DB con `project_id` en workflows, tasks, checkpoints, active_workflow. (`internal/adapters/state/migrations/001_initial_schema.sql:18-60`)
- Reescritura de queries y de `StateManager` en SQLite y JSON. (`internal/adapters/state/sqlite.go`, `internal/adapters/state/json.go`)
- Cambios amplios en API y frontend (similar a Alternativa B).

Compatibilidad:
- CLI/TUI: alto impacto si se quiere compartir state global.

Pros:
- Busqueda y reportes globales mas faciles.

Contras:
- Migracion compleja y alto riesgo de regresion.

## 7. Recomendacion

**Recomendacion principal**: Alternativa B con ProjectRegistry + StateManager por proyecto, manteniendo `.quorum` por proyecto. Justificacion:
- Hecho: el diseno actual ya usa `.quorum` por proyecto y no hay `project_id` en schema; instanciar state manager por proyecto evita migrar el schema. (`internal/config/loader.go:223-226`, `internal/adapters/state/migrations/001_initial_schema.sql:18-60`)
- Inferencia: Aunque es un refactor amplio, mantiene compatibilidad con datos existentes y evita migraciones destructivas.

**Estrategia incremental** (opcional): comenzar con Alternativa A como puente si se necesita valor rapido, y luego migrar hacia B.

## 8. Plan de implementacion conceptual (para Alternativa B)

### Fase 1: ProjectRegistry y contexto
1) Crear `ProjectRegistry` persistente (ej. `~/.config/quorum/projects.json`) con `id`, `name`, `root`. (Inferencia)
2) Definir `ProjectContext` (root, configPath, statePath, chatPath, eventBus).
3) Crear factory `ProjectServices` que instancie `StateManager`, `ChatStore`, `AttachmentsStore` usando el root del proyecto. Evidencia de necesidad: rutas actuales dependen de cwd. (`internal/api/files.go:265-290`, `internal/attachments/store.go:38-41`)

### Fase 2: Backend API con namespace de proyecto
1) Agregar endpoints `/api/v1/projects` (list/create/select) y `/api/v1/projects/{id}/...`.
2) Actualizar `api.Server` para resolver `ProjectContext` por request (middleware). (`internal/api/server.go:184-208`)
3) Cambiar `getConfigPath` a resolver por proyecto. (`internal/api/config.go:360-363`)
4) Instanciar EventBus por proyecto o incluir `projectId` en eventos + filtro SSE. (`internal/api/sse.go:33-61`)

### Fase 3: Frontend multi-proyecto
1) Nuevo store `projectStore` con `activeProjectId` y cache de workflows/config/chat por proyecto. Evidencia de stores actuales globales: `frontend/src/stores/workflowStore.js:6-110`.
2) Actualizar API base para incluir `projectId` en path o header. (`frontend/src/lib/api.js:1-33`)
3) Reconectar SSE al cambiar de proyecto. (`frontend/src/hooks/useSSE.js:9-40`)
4) UI selector de proyecto en top-right (Layout). (`frontend/src/components/Layout.jsx:266-289`)

### Fase 4: CLI/TUI (opcional)
1) Agregar flag `--project` o `--root` para usar registry, sin romper el modo actual por cwd. (`cmd/quorum/cmd/root.go:56-83`)
2) Mantener compatibilidad: si no hay flag, se usa cwd (comportamiento actual). (`internal/config/loader.go:54-90`)

### Fase 5: Hardening y mitigacion de regresiones
- Tests para routing con proyecto, SSE filtrado, y aislamiento de archivos/adjuntos. (`internal/api/sse.go:33-61`, `internal/api/files.go:265-290`)
- Validaciones de permisos para evitar lectura/escritura fuera del root del proyecto. (`internal/api/files.go:265-290`)
- Mecanismo de fallback: si `projectId` no existe, retornar error explicito.

## 9. Riesgos de regresion y mitigaciones
- Riesgo: mezclas de eventos entre proyectos si se usa un solo EventBus. Mitigacion: EventBus por proyecto o filtro SSE por projectId. (`internal/api/sse.go:33-61`)
- Riesgo: cache de frontend contaminada al cambiar proyecto. Mitigacion: namespacing por projectId o reset de stores. (`frontend/src/stores/workflowStore.js:6-110`)
- Riesgo: acceso a archivos fuera del proyecto si no se valida root por request. Mitigacion: usar `resolvePath` con root por proyecto. (`internal/api/files.go:265-290`)
- Riesgo: conflicto en active workflow y locks. Mitigacion: un StateManager por proyecto. (`internal/adapters/state/sqlite.go:847-1001`)

## 10. Edge cases a cubrir (checklist)
- Proyecto con `.quorum` corrupto o state.db bloqueado -> manejo de lock TTL y restore. (`internal/adapters/state/sqlite.go:1005-1038`, `internal/adapters/state/json.go:206-240`)
- Ruta invalida o sin permisos -> `resolvePath` debe fallar correctamente. (`internal/api/files.go:265-290`)
- Proyectos en montajes de red (latencias) -> potencial timeouts en SSE o DB. (Inferencia)
- Proyectos con diferentes versiones de schema -> necesidad de migracion por proyecto. (`internal/adapters/state/sqlite.go:206-240`)
- Cambio rapido de proyecto en UI con SSE activo -> evitar eventos cruzados. (`frontend/src/hooks/useSSE.js:9-40`)

## 11. Puntos de extension utiles en el codigo
- Port `core.StateManager` permite multiples implementaciones por proyecto. (`internal/core/ports.go:140-240`)
- Factory `state.NewStateManager` permite instancias separadas por path. (`internal/adapters/state/factory.go:19-49`)
- `workflow.RunnerBuilder` centraliza construccion de runners (punto de injection de ProjectContext). (`internal/service/workflow/builder.go:64-152`)
- `ChatStore` abstrae persistencia de chat, reusable por proyecto. (`internal/adapters/chat/factory.go:11-29`)

## 12. Conclusiones claras (hechos vs inferencias)
- Hecho: El sistema actual esta disenado para un unico proyecto por proceso, con rutas `.quorum` relativas y cwd como raiz. (`internal/config/loader.go:54-90`, `internal/api/server.go:130-138`)
- Hecho: No existe namespace de proyecto en API ni en stores del frontend. (`internal/api/server.go:184-208`, `frontend/src/App.jsx:1-39`, `frontend/src/stores/workflowStore.js:6-110`)
- Inferencia: Multi-tenant en un solo backend es viable pero requiere refactor transversal para aislar root, state, SSE y caches.
- Inferencia: La alternativa mas segura a corto plazo es UI agregador con multiples backends; la alternativa mas coherente a largo plazo es ProjectRegistry con instancias por proyecto.
