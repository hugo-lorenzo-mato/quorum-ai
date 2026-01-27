# ADR-0009: SQLite as Default State Backend

## Status

Accepted (supersedes ADR-0003 as default)

## Context

ADR-0003 established JSON as the state persistence backend for the POC, citing
simplicity and minimal dependencies. As quorum-ai matures beyond POC, several
limitations of JSON have become apparent:

1. **Concurrency**: JSON requires explicit file locking to prevent corruption
   during concurrent writes. This adds complexity and risk.
2. **Performance**: Loading and saving entire state files becomes slow as
   workflow history grows.
3. **Queries**: JSON requires loading all data into memory to query specific
   workflows or tasks.
4. **Atomicity**: Atomic writes in JSON require write-to-temp-then-rename
   patterns, which are error-prone across platforms.

SQLite addresses these issues while remaining a single-file, zero-configuration
database that fits quorum-ai's local-first model.

## Decision

Adopt SQLite as the default state backend. JSON remains available as an
alternative for debugging, migration, or environments where SQLite is
problematic.

### Per-Project Database

State is stored in `.quorum/state/state.db` within each project directory,
rather than a global database. Rationale:

- **Isolation**: Each project maintains independent workflow history without
  risk of cross-project interference.
- **Portability**: Projects can be moved, copied, or archived with their
  complete state intact.
- **Simplicity**: No need for multi-tenancy logic or project identifiers in
  the schema.
- **Git-friendliness**: The `.quorum/` directory can be selectively included
  or excluded from version control per project needs.

### Backward Compatibility

The JSON backend is retained for:

- Existing installations that already use JSON state files
- Debugging scenarios where human-readable state is valuable
- Environments with constraints on native dependencies

Users can switch backends via configuration:

```yaml
state:
  backend: json  # or sqlite (default)
  path: .quorum/state/state.json
```

Path extensions are automatically adjusted when switching backends.

### Future Considerations

JSON backend may be deprecated in a future major version (v2+) if:

- Maintenance burden of dual backends becomes significant
- SQLite proves universally reliable across target platforms
- Migration tooling matures sufficiently

Any deprecation would include a migration path and advance notice.

## Consequences

### Positive

- **ACID transactions**: Guaranteed consistency even during crashes or power
  loss.
- **Concurrent access**: Multiple processes can safely read/write without
  explicit locking.
- **Query performance**: Indexed queries for workflow lookup, task status,
  and history without loading entire state.
- **Incremental writes**: Only changed data is written, improving performance
  for large workflow histories.
- **Battle-tested**: SQLite is the most deployed database engine, with
  extensive testing and platform support.

### Negative

- **Binary format**: State files are not human-readable without tooling
  (mitigated by `quorum state` commands and JSON export).
- **Dependency size**: SQLite adds to binary size (~1-2MB with pure-Go
  implementation).
- **Schema migrations**: Future schema changes require migration logic
  (though SQLite's flexibility minimizes this).

### Neutral

- JSON backend remains available, so this is additive rather than breaking.
- ADR-0003's decision was correct for POC scope; this extends rather than
  invalidates it.
- Per-project databases prevent future options like cross-project queries,
  but this aligns with quorum-ai's project-centric model.

## References

- [ADR-0003: JSON State Persistence for POC](0003-state-persistence-json.md)
- docs/CONFIGURATION.md#state
- docs/ARCHITECTURE.md
