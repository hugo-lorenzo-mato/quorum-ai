> **Status: HISTORICAL -- IMPLEMENTED**
>
> This proposal has been fully implemented and exceeded. The WebUI configuration editor includes ETag-based concurrency control, config schema API for dynamic UI generation, enum endpoints, multi-project config scoping, and 6 settings tabs. See [CONFIGURATION.md](CONFIGURATION.md) for the current reference. The implementation lives in `internal/api/config.go`, `internal/api/config_enums.go`, and `frontend/src/pages/GlobalSettings.jsx`.

# Analysis: Configuration Management via Web UI

## 1. Executive Summary
Currently, `quorum-ai` configuration is managed via a `config.yaml` file. The goal is to enable full configuration management through the Web UI. This requires expanding the Backend API to expose all configuration options and creating a robust, sectioned Frontend interface for editing them. The YAML file will remain the source of truth, but the API will serve as the gateway for reading and writing these values.

## 2. Backend Analysis & Required Changes

### 2.1. Current State
- **File:** `internal/api/config.go`
- **Endpoints:** `GET /config/`, `PATCH /config/`
- **Limitations:**
  - The DTOs (`ConfigResponse`, `ConfigUpdateRequest`) are incomplete. They only cover a subset of the configuration (Workflow timeout, basic Agent settings, Git mode).
  - Missing critical sections: `Phases` (detailed), `Diagnostics`, `State`, `Report`, and comprehensive `Agent` settings (phase-specific models, reasoning effort).

### 2.2. Proposed Changes

#### A. Expand Data Transfer Objects (DTOs)
The `internal/api/config.go` file must be updated to mirror the structure of `internal/config/config.go` more closely.

**New/Updated Structs needed:**

```go
// internal/api/config.go updates

type ConfigResponse struct {
    // ... existing fields
    Phases      PhasesConfigResponse      `json:"phases"`
    Diagnostics DiagnosticsConfigResponse `json:"diagnostics"`
    State       StateConfigResponse       `json:"state"`
    Report      ReportConfigResponse      `json:"report"`
    // ... expand Agents and Git
}

type PhasesConfigResponse struct {
    Analyze AnalyzePhaseConfigResponse `json:"analyze"`
    Plan    PlanPhaseConfigResponse    `json:"plan"`
    Execute ExecutePhaseConfigResponse `json:"execute"`
}

// ... corresponding Update structs (pointer-based for partial updates)
```

#### B. Update Handlers
- **`handleGetConfig`**: Map the full `internal/config.Config` struct to the expanded `ConfigResponse`.
- **`handleUpdateConfig`**:
  - Implement deep merging logic. Since the request contains pointers, only update fields that are non-nil.
  - **Crucial:** Invoke `config.Validate(cfg)` *before* saving. If validation fails, return `400 Bad Request` with the validation error message.

#### C. Synchronization Strategy
- **File Writing:** Continue using `yaml.Marshal` to write to `.quorum/config.yaml`.
- **Concurrency:** For a local tool, simple file writing is acceptable. To prevent partial writes, use a generic atomic write approach (write to temp file, fsync, rename).

## 3. Frontend Analysis & Required Changes

### 3.1. Architecture
- **State Management:** Create a `useConfigStore` (Zustand) to manage:
  - `config`: The current configuration object.
  - `isDirty`: Whether changes have been made.
  - `isLoading`: Fetching state.
  - `isSaving`: Saving state.
  - `fetchConfig()`: Async action to load from API.
  - `saveConfig()`: Async action to push to API.
  - `reset()`: Revert to last saved state.

### 3.2. UX/UI Design
The configuration is complex. A flat list is overwhelming.

**Proposed Layout:**
- **Sidebar (Vertical Tabs):**
  1.  **General:** Logging, Report settings, Theme (local).
  2.  **Workflow:** Global timeouts, Deny Tools.
  3.  **Agents:** A detailed matrix or list view.
  4.  **Phases:** Deep dive into Analyze, Plan, Execute settings.
  5.  **Git & State:** Repo settings, Worktree modes, Persistence backend.
  6.  **Diagnostics:** Monitoring thresholds, Crash dumps.

**Key Components:**

1.  **`SettingsLayout`**: Wraps the page, handles the sidebar navigation.
2.  **`SectionHeader`**: Title and description for each panel.
3.  **`FormField`**: Reusable wrapper for Label + Input + HelpTooltip.
    - *Tip:* Use "Info" icons with tooltips to explain complex fields (e.g., "Reasoning Effort").
4.  **`AgentCard`**: A component to configure a single agent.
    - Toggle for "Enabled".
    - Dropdown for "Model".
    - Matrix checkboxes for "Phases" (Refine, Analyze, etc.).
    - Advanced section for "Phase Models" overrides.

### 3.3. Implementation Details

#### Agent Configuration Experience
This is the most complex part.
- **Problem:** Users might not know which model corresponds to which agent alias.
- **Solution:**
  - Group by Agent Alias (Claude, Gemini, etc.).
  - **Defaults:** Show what the default model is if not explicitly set (greyed out placeholder text).
  - **Capabilities:** Visual indicators for what an agent is used for (e.g., "Currently used for: Refiner, Moderator").

#### Validation Feedback
- Frontend validation (e.g., required fields).
- Backend validation display: If the API returns a 400 error (e.g., "SingleAgent and Moderator cannot both be enabled"), display this as a toast notification or a banner at the top of the settings page.

## 4. Migration & Compatibility

- **Storage Location:** Keep `.quorum/config.yaml` as the primary storage. This allows the CLI to work seamlessly without needing a running server database.
- **Defaults:** The frontend should handle "undefined" values by showing the system defaults (which can be hardcoded in frontend or sent via a separate `/config/defaults` endpoint - *Recommendation: Add a `defaults` field to the GET response or a separate endpoint so the frontend doesn't need to hardcode values*).

## 5. Action Plan

1.  **Backend:**
    - Update `internal/api/config.go` with full DTOs.
    - Implement the mapping logic in `configToResponse` and `applyConfigUpdates`.
    - Add validation call in `handleUpdateConfig`.
2.  **Frontend:**
    - Create `src/stores/configStore.js`.
    - Refactor `Settings.jsx` to use the Sidebar layout.
    - Create sub-components for each section.
    - Integrate with `configStore`.

## 6. Visual Mockup (ASCII)

```
+----------------+----------------------------------------------------+
|  SETTINGS      |  AGENTS CONFIGURATION                              |
|                |                                                    |
|  General       |  [ Claude ]                                        |
|  Workflow      |    Enabled: [x]                                    |
| >Agents        |    Model: [ claude-3-opus v]                       |
|  Phases        |    Roles: [x] Refine  [x] Analyze  [x] Plan        |
|  Git           |           [ ] Moderate (assigned to Copilot)       |
|  Diagnostics   |    > Advanced (Phase Models)                       |
|                |                                                    |
|                |  --------------------------------------------------|
|                |                                                    |
|                |  [ Gemini ]                                        |
|                |    Enabled: [x]                                    |
|                |    ...                                             |
|                |                                                    |
+----------------+----------------------------------------------------+
|                |           [ Reset ]                 [ Save Changes]|
+----------------+----------------------------------------------------+
```
