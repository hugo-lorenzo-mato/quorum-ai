# ADR-0001: Adopt Hexagonal Architecture

## Status

Accepted

## Context

The project needs a testable and maintainable architecture with a clear
separation between business logic and external systems. The system integrates
CLI, TUI, and multiple external agent adapters, which will evolve over time.
A strict separation between core logic and IO concerns reduces coupling and
makes it easier to validate behavior with tests.

## Decision

Adopt a hexagonal (ports and adapters) architecture. Core domain logic lives in
internal/core, orchestration in internal/service, and all external interactions
are implemented as adapters.

## Consequences

### Positive
- Core logic can be tested without external dependencies
- Adapters can be swapped or expanded without changing core logic
- Clear boundaries improve maintainability as the system grows

### Negative
- Additional boilerplate for ports and adapters
- Requires more up-front design discipline

### Neutral
- Contributors need to learn and follow the layering rules

## References

- https://alistair.cockburn.us/hexagonal-architecture/
- ../ARCHITECTURE.md
