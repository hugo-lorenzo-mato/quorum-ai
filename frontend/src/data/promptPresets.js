/**
 * Prompt presets for common software development tasks
 * Each preset provides a pre-configured prompt and execution strategy
 */

export const promptPresets = [
  // ===== CODE ANALYSIS =====
  {
    id: 'technical-debt-analysis',
    category: 'Code Analysis',
    name: 'Technical Debt Analysis',
    description: 'Identify technical debt hotspots: code complexity, duplications, antipatterns, and prioritize remediation',
    icon: 'analysis',
    prompt: `Analyze the codebase for technical debt:

1. **Code Complexity**: Identify functions/classes with high cyclomatic complexity
2. **Code Duplications**: Find duplicate or similar code blocks that could be refactored
3. **Antipatterns**: Detect common antipatterns (God objects, circular dependencies, etc.)
4. **Legacy Code**: Identify outdated patterns or deprecated API usage
5. **Priority Matrix**: Categorize findings by impact (high/medium/low) and effort (quick wins vs. long-term)

Provide actionable recommendations with specific file locations and suggested refactorings.`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['analysis', 'quality', 'refactoring']
  },
  {
    id: 'dead-code-detector',
    category: 'Code Analysis',
    name: 'Dead Code Detector',
    description: 'Find unused code: unreferenced functions, unused imports, orphaned files, and calculate cleanup impact',
    icon: 'trash',
    prompt: `Scan the codebase for dead code:

1. **Unreferenced Functions**: Find functions/methods that are never called
2. **Unused Imports**: Identify imports that are never used
3. **Orphaned Files**: Detect files that are not imported anywhere
4. **Dead Routes**: Find registered routes that are not linked from anywhere
5. **Unused Dependencies**: Check package.json/pom.xml for unused dependencies
6. **Impact Calculation**: Estimate LOC reduction and bundle size improvement

Provide a prioritized list with safe-to-remove candidates vs. needs-investigation.`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['analysis', 'cleanup', 'optimization']
  },
  {
    id: 'complexity-hotspots',
    category: 'Code Analysis',
    name: 'Complexity Hotspots',
    description: 'Analyze cyclomatic complexity, cognitive complexity, nesting depth and highlight functions needing refactoring',
    icon: 'flame',
    prompt: `Analyze code complexity metrics:

1. **Cyclomatic Complexity**: Calculate complexity score for all functions (target: <10)
2. **Cognitive Complexity**: Measure how difficult code is to understand
3. **Nesting Depth**: Identify deeply nested code (target: <4 levels)
4. **Function Length**: Find functions exceeding recommended length (target: <50 LOC)
5. **Parameter Count**: Detect functions with too many parameters (target: <5)

For each hotspot, provide:
- Specific location and complexity score
- Refactoring suggestions (extract method, simplify conditions, etc.)
- Priority based on change frequency and business criticality`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['analysis', 'complexity', 'refactoring']
  },
  {
    id: 'dependency-health-check',
    category: 'Code Analysis',
    name: 'Dependency Health Check',
    description: 'Audit dependencies: outdated packages, known CVEs, license issues, bundle size impact',
    icon: 'package',
    prompt: `Perform a comprehensive dependency audit:

1. **Outdated Packages**: List dependencies with available updates (minor, major, breaking)
2. **Security Vulnerabilities**: Identify known CVEs with severity scores
3. **License Issues**: Check for incompatible or restrictive licenses
4. **Bundle Size Impact**: Identify heavy dependencies contributing most to bundle size
5. **Duplicate Dependencies**: Find multiple versions of the same package
6. **Alternative Recommendations**: Suggest lighter or more maintained alternatives

Prioritize by risk level: Critical CVEs > Major version behind > High bundle impact.`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['analysis', 'security', 'dependencies']
  },
  {
    id: 'architecture-conformance',
    category: 'Code Analysis',
    name: 'Architecture Conformance',
    description: 'Validate if code follows intended architecture (hexagonal/clean/layered) and identify boundary violations',
    icon: 'architecture',
    prompt: `Analyze architectural conformance:

1. **Architecture Detection**: Identify the intended architecture pattern (layered, hexagonal, clean, etc.)
2. **Layer Violations**: Detect improper dependencies between layers (e.g., UI calling DB directly)
3. **Boundary Violations**: Find domain logic leaking into infrastructure or vice versa
4. **Dependency Direction**: Verify dependencies flow in the correct direction
5. **Separation of Concerns**: Check if responsibilities are properly separated
6. **Package Structure**: Validate if package/module structure reflects architecture

For each violation, explain why it's problematic and suggest the correct approach.`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['analysis', 'architecture', 'design']
  },

  // ===== PERFORMANCE ANALYSIS =====
  {
    id: 'database-query-analysis',
    category: 'Performance Analysis',
    name: 'Database Query Analysis',
    description: 'Detect N+1 queries, missing indexes, inefficient joins, and suggest optimizations',
    icon: 'database',
    prompt: `Analyze database query performance:

1. **N+1 Query Detection**: Find loops that trigger individual queries (use batch loading instead)
2. **Missing Indexes**: Identify frequently queried columns without indexes
3. **Inefficient Joins**: Spot complex joins that could be simplified or cached
4. **Unused Indexes**: Find indexes that are not used by any query
5. **Query Optimization**: Suggest query rewrites (e.g., EXISTS vs IN, proper pagination)
6. **Connection Pool Issues**: Check for connection leaks or pool exhaustion

For each finding, provide:
- Code location and problematic pattern
- Performance impact estimation
- Optimized alternative with code example`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['performance', 'database', 'optimization']
  },
  {
    id: 'bundle-size-analyzer',
    category: 'Performance Analysis',
    name: 'Bundle Size Analyzer',
    description: 'Analyze JS bundle: heavy dependencies, code splitting opportunities, tree-shaking issues',
    icon: 'bundle',
    prompt: `Analyze JavaScript bundle size:

1. **Heavy Dependencies**: Identify largest dependencies and their impact
2. **Code Splitting**: Find opportunities to split code by route or feature
3. **Tree-Shaking Issues**: Detect imports preventing tree-shaking (e.g., entire lodash)
4. **Lazy Loading**: Suggest components/modules that could be lazy loaded
5. **Duplicate Code**: Find code duplicated across chunks
6. **Moment.js/date-fns**: Check for heavy date libraries (suggest lighter alternatives)

Provide:
- Current bundle size breakdown
- Optimization recommendations with expected size reduction
- Implementation examples for code splitting`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['performance', 'frontend', 'optimization']
  },
  {
    id: 'memory-leak-detector',
    category: 'Performance Analysis',
    name: 'Memory Leak Detector',
    description: 'Scan for memory leak patterns: event listeners not cleaned, closures holding references, DOM leaks',
    icon: 'memory',
    prompt: `Scan for memory leak patterns:

1. **Event Listeners**: Find addEventListener without removeEventListener
2. **Interval/Timeout**: Detect setInterval/setTimeout not cleared in cleanup
3. **Closure Leaks**: Identify closures holding unnecessary references
4. **DOM Leaks**: Find detached DOM nodes still referenced
5. **Cache Without Limits**: Detect caches that grow unbounded
6. **React-Specific**: useEffect without cleanup, refs not cleared

For each potential leak:
- Code location and pattern
- Why it causes a leak
- Fixed code example with proper cleanup`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['performance', 'memory', 'bugs']
  },
  {
    id: 'react-performance-audit',
    category: 'Performance Analysis',
    name: 'React Performance Audit',
    description: 'Find unnecessary re-renders, missing memoization, expensive computations in render',
    icon: 'react',
    prompt: `Audit React performance:

1. **Unnecessary Re-renders**: Components re-rendering when props haven't changed
2. **Missing Memoization**: Expensive calculations that should use useMemo
3. **Callback Recreation**: Functions recreated on each render (need useCallback)
4. **Context Overuse**: Context causing too many re-renders (split context)
5. **Key Prop Issues**: Missing or incorrect keys in lists
6. **Inline Objects/Arrays**: Creating new objects in render causing child re-renders
7. **Large Component Trees**: Components that should be split smaller

For each issue:
- Component location and render frequency
- Performance impact
- Optimized code with React.memo, useMemo, or useCallback`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['performance', 'react', 'frontend']
  },

  // ===== SECURITY ANALYSIS =====
  {
    id: 'secrets-credentials-scan',
    category: 'Security Analysis',
    name: 'Secrets & Credentials Scan',
    description: 'Find hardcoded secrets, API keys, passwords, tokens, certificates',
    icon: 'lock',
    prompt: `Scan for exposed secrets and credentials:

1. **Hardcoded Secrets**: API keys, passwords, tokens in code
2. **Database Credentials**: Connection strings with embedded passwords
3. **Private Keys**: RSA/SSH keys, certificates in repository
4. **Environment Variables**: Check .env files are in .gitignore
5. **Git History**: Suggest scanning git history for past leaks
6. **Secret Management**: Recommend vault solutions (HashiCorp Vault, AWS Secrets Manager)

For each finding:
- File location and type of secret
- Severity (public repo = critical, private = high)
- Remediation steps (rotate key, use env vars, vault integration)`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['security', 'secrets', 'critical']
  },
  {
    id: 'input-validation-audit',
    category: 'Security Analysis',
    name: 'Input Validation Audit',
    description: 'Identify missing input validation, SQL injection risks, XSS vulnerabilities, command injection',
    icon: 'shield',
    prompt: `Audit input validation and injection vulnerabilities:

1. **SQL Injection**: Find string concatenation in SQL queries (use parameterized queries)
2. **XSS Vulnerabilities**: Identify unescaped user input rendered as HTML
3. **Command Injection**: Detect user input passed to system commands
4. **Path Traversal**: Check file operations with user-controlled paths
5. **LDAP/XML Injection**: Find unvalidated input in LDAP queries or XML parsing
6. **Validation Missing**: Input endpoints without validation (length, format, type)

For each vulnerability:
- Attack vector and example exploit
- Affected endpoints/functions
- Secure code example with proper validation/sanitization`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['security', 'validation', 'critical']
  },
  {
    id: 'auth-authorization-review',
    category: 'Security Analysis',
    name: 'Auth & Authorization Review',
    description: 'Analyze authentication flow, session management, JWT handling, RBAC implementation',
    icon: 'key',
    prompt: `Review authentication and authorization:

1. **Authentication Flow**: Analyze login/logout implementation, weak points
2. **Session Management**: Check session timeouts, secure flags, regeneration
3. **JWT Handling**: Verify signature, expiration, secure storage (not localStorage)
4. **Password Security**: Check hashing (bcrypt/argon2), salt, complexity requirements
5. **RBAC Implementation**: Verify role checks are enforced server-side
6. **Privilege Escalation**: Test if users can access unauthorized resources
7. **Multi-Factor Authentication**: Check if MFA is implemented for sensitive operations

For each issue:
- Security impact (authentication bypass, privilege escalation, etc.)
- Specific vulnerable code
- Secure implementation example`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['security', 'authentication', 'critical']
  },
  {
    id: 'api-security-checklist',
    category: 'Security Analysis',
    name: 'API Security Checklist',
    description: 'Review API endpoints: rate limiting, CORS, authentication, input validation, error disclosure',
    icon: 'globe',
    prompt: `Audit API security:

1. **Rate Limiting**: Check if endpoints have rate limiting to prevent abuse
2. **CORS Configuration**: Verify CORS is not set to * (wildcard)
3. **Authentication**: Ensure all sensitive endpoints require authentication
4. **Input Validation**: Validate all request parameters (query, body, headers)
5. **Error Disclosure**: Check errors don't leak sensitive info (stack traces, DB details)
6. **API Versioning**: Verify deprecated endpoints are properly sunset
7. **HTTPS Only**: Ensure API only works over HTTPS
8. **API Keys**: Check API key rotation and secure storage

For each endpoint, assess:
- Current security posture
- Missing protections
- Recommended fixes with code examples`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['security', 'api', 'web']
  },

  // ===== TESTING ANALYSIS =====
  {
    id: 'test-coverage-gap-analysis',
    category: 'Testing Analysis',
    name: 'Test Coverage Gap Analysis',
    description: 'Identify critical paths without tests, edge cases not covered, missing integration tests',
    icon: 'target',
    prompt: `Analyze test coverage gaps:

1. **Critical Paths**: Identify business-critical code without test coverage
2. **Edge Cases**: Find edge cases not covered (null, empty, boundary values)
3. **Error Paths**: Check if error handling is tested
4. **Integration Tests**: Identify missing integration tests for key user flows
5. **Risk Score**: Calculate risk = (code complexity × business criticality) / test coverage
6. **Mutation Testing**: Suggest areas where tests might pass but not catch bugs

For each gap:
- Code location and risk level
- Missing test scenarios
- Example test cases to add`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['testing', 'quality', 'coverage']
  },
  {
    id: 'flaky-test-detector',
    category: 'Testing Analysis',
    name: 'Flaky Test Detector',
    description: 'Analyze test failures: race conditions, timing issues, shared state, non-deterministic behavior',
    icon: 'dices',
    prompt: `Detect flaky test patterns:

1. **Race Conditions**: Tests depending on timing or async operations without proper awaits
2. **Shared State**: Tests that fail when run in parallel due to shared global state
3. **External Dependencies**: Tests depending on network, filesystem, or database state
4. **Non-Deterministic**: Tests using random values, current date/time without mocking
5. **Test Order Dependency**: Tests that fail when run in different order
6. **Improper Cleanup**: Tests not cleaning up resources (files, DB records, mocks)

For each flaky pattern:
- Test location and failure symptom
- Root cause explanation
- Fixed test with proper isolation/mocking`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['testing', 'quality', 'reliability']
  },
  {
    id: 'test-quality-assessment',
    category: 'Testing Analysis',
    name: 'Test Quality Assessment',
    description: 'Evaluate test quality: assertions effectiveness, test isolation, mocking patterns, maintainability',
    icon: 'check',
    prompt: `Assess test quality:

1. **Assertion Effectiveness**: Check if tests actually verify behavior (no assertions = red flag)
2. **Test Isolation**: Tests should not depend on each other or external state
3. **Mocking Patterns**: Verify mocks are used appropriately (not over-mocking)
4. **Test Readability**: Assess if tests follow Given-When-Then or Arrange-Act-Assert
5. **Test Maintainability**: Check for brittle tests that break with minor changes
6. **Test Data**: Use factories/builders instead of hardcoded test data
7. **Test Organization**: Proper describe/it structure, descriptive names

For each quality issue:
- Test location and problem
- Why it reduces test effectiveness
- Improved test example`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['testing', 'quality', 'best-practices']
  },

  // ===== INFRASTRUCTURE ANALYSIS =====
  {
    id: 'docker-image-optimizer',
    category: 'Infrastructure Analysis',
    name: 'Docker Image Optimizer',
    description: 'Analyze Dockerfile: multi-stage builds, layer caching, base image size, security scanning',
    icon: 'container',
    prompt: `Optimize Docker images:

1. **Multi-Stage Builds**: Check if using multi-stage to reduce final image size
2. **Layer Caching**: Optimize layer order (dependencies before source code)
3. **Base Image Size**: Suggest alpine or distroless alternatives
4. **Security Scanning**: Check for vulnerabilities in base image
5. **Build Time**: Identify slow build steps that could be parallelized or cached
6. **.dockerignore**: Verify .dockerignore excludes unnecessary files
7. **Layer Count**: Combine RUN commands to reduce layers

Provide:
- Current image size and build time
- Optimized Dockerfile with explanations
- Expected improvements (size reduction, build time)`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['infrastructure', 'docker', 'optimization']
  },
  {
    id: 'cicd-pipeline-audit',
    category: 'Infrastructure Analysis',
    name: 'CI/CD Pipeline Audit',
    description: 'Review GitHub Actions/CI: workflow optimization, caching strategies, parallelization, secret management',
    icon: 'refresh',
    prompt: `Audit CI/CD pipeline:

1. **Caching Strategies**: Check if dependencies are cached (npm, Maven, Docker layers)
2. **Parallelization**: Identify jobs that could run in parallel
3. **Workflow Triggers**: Verify workflows trigger on appropriate events
4. **Secret Management**: Ensure secrets use GitHub Secrets, not hardcoded
5. **Job Duration**: Find slow jobs that could be optimized
6. **Matrix Builds**: Use matrix strategy for testing multiple versions
7. **Cost Optimization**: Identify redundant jobs or inefficient runners

For each optimization:
- Current bottleneck or issue
- Recommended change with workflow YAML
- Expected time/cost savings`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['infrastructure', 'ci-cd', 'optimization']
  },

  // ===== OBSERVABILITY ANALYSIS =====
  {
    id: 'logging-coverage-analysis',
    category: 'Observability Analysis',
    name: 'Logging Coverage Analysis',
    description: 'Identify missing logs in critical paths, inconsistent log levels, PII in logs, structured logging',
    icon: 'logging',
    prompt: `Analyze logging coverage:

1. **Missing Logs**: Identify critical operations without logging (errors, business events)
2. **Log Levels**: Check for incorrect log levels (debug as info, errors as warnings)
3. **PII in Logs**: Find logs containing personal identifiable information
4. **Structured Logging**: Verify logs use structured format (JSON) not plain strings
5. **Correlation IDs**: Check if requests have correlation IDs for tracing
6. **Sensitive Data**: Ensure passwords, tokens, credit cards are not logged
7. **Log Volume**: Identify overly verbose logging that could impact performance

For each finding:
- Code location and issue
- Privacy/compliance risk
- Corrected logging example`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['observability', 'logging', 'compliance']
  },
  {
    id: 'error-handling-assessment',
    category: 'Observability Analysis',
    name: 'Error Handling Assessment',
    description: 'Find uncaught exceptions, missing error boundaries, poor error messages, error recovery gaps',
    icon: 'warning',
    prompt: `Assess error handling:

1. **Uncaught Exceptions**: Find try-catch blocks missing or errors not handled
2. **Error Boundaries**: React apps should have error boundaries at strategic levels
3. **Error Messages**: Check if errors provide actionable information to users
4. **Error Propagation**: Verify errors bubble up correctly without being swallowed
5. **Retry Logic**: Identify operations that should retry on transient failures
6. **Graceful Degradation**: Check if system degrades gracefully vs. hard failures
7. **Error Monitoring**: Verify errors are sent to monitoring (Sentry, Rollbar)

For each gap:
- Code location and risk
- User/developer impact
- Proper error handling example`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['observability', 'reliability', 'errors']
  },
  {
    id: 'monitoring-gaps-detector',
    category: 'Observability Analysis',
    name: 'Monitoring Gaps Detector',
    description: 'Identify missing metrics, alerting opportunities, SLO/SLI coverage, trace missing in critical flows',
    icon: 'monitoring',
    prompt: `Detect monitoring gaps:

1. **Missing Metrics**: Identify critical operations without metrics (latency, throughput, errors)
2. **Alerting**: Suggest alerts for SLO violations, error spikes, performance degradation
3. **SLO/SLI Coverage**: Check if key user journeys have defined SLOs
4. **Trace Coverage**: Verify distributed tracing covers critical flows
5. **Business Metrics**: Ensure business KPIs are tracked (sign-ups, conversions, revenue)
6. **Dashboard Gaps**: Identify missing dashboards for system health
7. **Anomaly Detection**: Suggest metrics that would benefit from anomaly detection

For each gap:
- What should be monitored and why
- Recommended metrics and thresholds
- Example implementation (Prometheus, DataDog, etc.)`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['observability', 'monitoring', 'sre']
  },

  // ===== MIGRATION ANALYSIS =====
  {
    id: 'framework-migration-planner',
    category: 'Migration Analysis',
    name: 'Framework Migration Planner',
    description: 'Analyze codebase for migration (React, Vue, Angular): breaking changes, deprecations, effort estimation',
    icon: 'refresh',
    prompt: `Plan framework migration:

1. **Breaking Changes**: Identify code affected by breaking changes in new version
2. **Deprecated APIs**: Find usage of deprecated APIs that need replacement
3. **New Features**: Suggest new features that could simplify existing code
4. **Migration Phases**: Break migration into phases (codemods, manual changes, testing)
5. **Effort Estimation**: Estimate person-days per phase
6. **Risk Assessment**: Identify high-risk areas that need extra testing
7. **Rollback Plan**: Suggest incremental migration strategy with rollback points

Provide:
- Comprehensive migration checklist
- Code examples for common patterns (before/after)
- Recommended migration order`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['migration', 'planning', 'framework']
  },
  {
    id: 'javascript-to-typescript',
    category: 'Migration Analysis',
    name: 'JavaScript to TypeScript',
    description: 'Analyze JS codebase: type inference suggestions, any type candidates, interface extraction',
    icon: 'typescript',
    prompt: `Plan JavaScript to TypeScript migration:

1. **Type Inference**: Suggest explicit types where inference fails
2. **Any Type Candidates**: Find complex types that might need 'any' initially
3. **Interface Extraction**: Generate interfaces from object shapes
4. **Utility Types**: Suggest utility types (Partial, Pick, Omit) for common patterns
5. **Strict Mode**: Assess readiness for strict mode flags
6. **Third-Party Types**: Check @types availability for dependencies
7. **Migration Strategy**: Suggest file-by-file vs. all-at-once approach

Provide:
- Priority order for migrating files (leaf nodes first)
- Type definitions for core data structures
- tsconfig.json with appropriate strictness`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['migration', 'typescript', 'types']
  },
  {
    id: 'change-impact-analysis',
    category: 'Migration Analysis',
    name: 'Change Impact Analysis',
    description: 'Analyze proposed change impact: affected modules, breaking changes, required updates, rollback strategy',
    icon: 'target',
    prompt: `Analyze impact of proposed change:

1. **Affected Modules**: Identify all modules that depend on changed code
2. **Breaking Changes**: Detect if change breaks existing contracts/APIs
3. **Required Updates**: List all locations that need updates
4. **Test Coverage**: Check if affected code has tests
5. **Rollback Strategy**: Assess how easily change can be rolled back
6. **Performance Impact**: Estimate performance implications
7. **Migration Path**: For breaking changes, suggest migration steps

Provide:
- Blast radius (how many files/modules affected)
- Risk level (low/medium/high)
- Recommended rollout strategy (feature flag, canary, etc.)`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['analysis', 'impact', 'planning']
  },

  // ===== UX/ACCESSIBILITY ANALYSIS =====
  {
    id: 'accessibility-audit-wcag',
    category: 'UX/Accessibility Analysis',
    name: 'Accessibility Audit (WCAG)',
    description: 'Review for WCAG 2.1 AA: ARIA labels, keyboard navigation, color contrast, screen reader support',
    icon: 'accessibility',
    prompt: `Audit accessibility compliance (WCAG 2.1 AA):

1. **ARIA Labels**: Check interactive elements have proper labels
2. **Keyboard Navigation**: Verify all functionality accessible via keyboard
3. **Color Contrast**: Test text/background contrast meets 4.5:1 ratio
4. **Screen Reader Support**: Ensure semantic HTML and ARIA for screen readers
5. **Focus Management**: Check visible focus indicators and logical tab order
6. **Form Accessibility**: Labels, error messages, required field indicators
7. **Image Alt Text**: Verify images have descriptive alt text

For each violation:
- WCAG criterion (e.g., 1.1.1, 2.1.1)
- User impact (blind, low vision, keyboard-only users)
- Remediation with code example`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['accessibility', 'ux', 'compliance']
  },
  {
    id: 'mobile-responsiveness-check',
    category: 'UX/Accessibility Analysis',
    name: 'Mobile Responsiveness Check',
    description: 'Analyze responsive design: breakpoint coverage, touch targets, viewport config, mobile-specific UX',
    icon: 'mobile',
    prompt: `Audit mobile responsiveness:

1. **Breakpoint Coverage**: Check layout works at common breakpoints (320px, 375px, 768px, 1024px)
2. **Touch Targets**: Verify buttons/links are at least 44×44px
3. **Viewport Meta Tag**: Ensure proper viewport configuration
4. **Mobile-Specific UX**: Check for mobile hamburger menu, swipe gestures
5. **Font Sizing**: Text should scale appropriately, avoid fixed pixel sizes
6. **Horizontal Scrolling**: Ensure no horizontal scroll on small screens
7. **Performance**: Check if images are responsive, lazy loaded

For each issue:
- Breakpoint and specific problem
- User experience impact
- Responsive CSS fix`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['mobile', 'responsive', 'ux']
  },
  {
    id: 'user-journey-analysis',
    category: 'UX/Accessibility Analysis',
    name: 'User Journey Analysis',
    description: 'Map user flows, identify friction points, dead ends, confusing navigation, conversion blockers',
    icon: 'map',
    prompt: `Analyze user journeys:

1. **Flow Mapping**: Map out key user journeys (sign-up, purchase, onboarding)
2. **Friction Points**: Identify steps that cause user confusion or drop-off
3. **Dead Ends**: Find pages with no clear next action
4. **Navigation Issues**: Detect confusing or inconsistent navigation
5. **Form Complexity**: Assess if forms are too long or complex
6. **Error Prevention**: Check if system prevents errors vs. just showing them
7. **Conversion Blockers**: Identify UX issues preventing goal completion

For each journey:
- Current flow diagram
- Identified friction points with severity
- Recommended improvements`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['ux', 'conversion', 'user-flow']
  },

  // ===== CONSISTENCY ANALYSIS =====
  {
    id: 'api-design-consistency',
    category: 'Consistency Analysis',
    name: 'API Design Consistency',
    description: 'Review REST/GraphQL consistency: naming conventions, HTTP verbs, response structures, error formats',
    icon: 'link',
    prompt: `Audit API design consistency:

1. **Naming Conventions**: Check consistent resource naming (plural/singular, camelCase/snake_case)
2. **HTTP Verbs**: Verify proper use of GET/POST/PUT/PATCH/DELETE
3. **Response Structures**: Ensure consistent shape ({ data, error, meta })
4. **Error Formats**: Check error responses follow same format (code, message, details)
5. **Status Codes**: Verify appropriate status codes (200, 201, 400, 404, 500)
6. **Pagination**: Consistent pagination (cursor vs. offset, parameter names)
7. **Versioning**: Check API versioning strategy (URL vs. header)

For each inconsistency:
- Endpoints affected
- Current vs. recommended pattern
- Migration guide if breaking change needed`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['api', 'consistency', 'design']
  },
  {
    id: 'code-style-inconsistencies',
    category: 'Consistency Analysis',
    name: 'Code Style Inconsistencies',
    description: 'Find pattern violations: naming conventions, file structure, import order, component patterns',
    icon: 'palette',
    prompt: `Detect code style inconsistencies:

1. **Naming Conventions**: Find violations (camelCase, PascalCase, SCREAMING_SNAKE_CASE)
2. **File Structure**: Check if files are organized consistently
3. **Import Order**: Verify consistent import sorting (stdlib, external, internal)
4. **Component Patterns**: Check consistent component structure (hooks order, prop types)
5. **State Management**: Ensure consistent state management approach
6. **Error Handling**: Verify consistent error handling pattern
7. **Code Formatting**: Check for formatting inconsistencies

For each inconsistency:
- File location and pattern
- Correct pattern based on project conventions
- Autofix suggestions (eslint, prettier rules)`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['consistency', 'style', 'quality']
  },

  // ===== JAVA/SPRING BOOT SPECIFIC =====
  {
    id: 'spring-boot-best-practices',
    category: 'Java/Spring Boot',
    name: 'Spring Boot Best Practices',
    description: 'Audit Spring Boot app: proper annotations, dependency injection, configuration, exception handling',
    icon: 'leaf',
    prompt: `Audit Spring Boot best practices:

1. **Annotation Usage**: Check proper use of @Service, @Repository, @Component, @Controller
2. **Dependency Injection**: Prefer constructor injection over field injection
3. **Configuration Properties**: Use @ConfigurationProperties instead of @Value for complex config
4. **Exception Handling**: Verify @ControllerAdvice for global exception handling
5. **Transaction Management**: Check @Transactional usage and propagation settings
6. **Bean Scope**: Verify appropriate bean scopes (singleton, prototype, request)
7. **Spring Data**: Check proper repository method naming, custom queries, pagination

For each issue:
- Anti-pattern detected
- Why it's problematic (testability, maintainability, performance)
- Best practice implementation`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['java', 'spring-boot', 'best-practices']
  },
  {
    id: 'jpa-hibernate-optimization',
    category: 'Java/Spring Boot',
    name: 'JPA/Hibernate Optimization',
    description: 'Analyze JPA usage: N+1 queries, lazy loading, fetch strategies, entity relationships, caching',
    icon: 'save',
    prompt: `Optimize JPA/Hibernate usage:

1. **N+1 Query Detection**: Find @OneToMany/@ManyToOne causing N+1 queries (use JOIN FETCH)
2. **Lazy Loading Issues**: Check LazyInitializationException risks (use @Transactional correctly)
3. **Fetch Strategies**: Verify EAGER vs LAZY fetch is appropriate
4. **Entity Relationships**: Review bidirectional relationships, orphan removal, cascade types
5. **Query Optimization**: Check for SELECT * (use DTO projections), pagination
6. **Second-Level Cache**: Suggest entities that would benefit from caching (@Cacheable)
7. **Batch Processing**: Find loops saving entities individually (use batch inserts)

For each issue:
- Entity/repository location
- Performance impact (query count, load time)
- Optimized code with @EntityGraph, @Query, or DTO projection`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['java', 'jpa', 'hibernate', 'performance']
  },
  {
    id: 'java-concurrency-review',
    category: 'Java/Spring Boot',
    name: 'Java Concurrency Review',
    description: 'Audit thread safety: shared mutable state, synchronization issues, race conditions, deadlocks',
    icon: 'shuffle',
    prompt: `Review Java concurrency and thread safety:

1. **Shared Mutable State**: Find fields accessed by multiple threads without synchronization
2. **Synchronization Issues**: Check improper use of synchronized blocks/methods
3. **Race Conditions**: Identify check-then-act patterns without atomicity
4. **Deadlock Potential**: Find circular lock dependencies
5. **Volatile Usage**: Check if volatile is needed for visibility guarantees
6. **Thread-Safe Collections**: Verify use of ConcurrentHashMap vs HashMap in multi-threaded context
7. **CompletableFuture**: Review async code for proper error handling and composition

For each issue:
- Thread safety violation type
- Potential consequences (data corruption, deadlock, etc.)
- Thread-safe alternative (synchronized, locks, atomic classes, concurrent collections)`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['java', 'concurrency', 'thread-safety']
  },
  {
    id: 'java-exception-handling',
    category: 'Java/Spring Boot',
    name: 'Java Exception Handling',
    description: 'Review exception handling: checked vs unchecked, exception swallowing, resource cleanup, custom exceptions',
    icon: 'zap',
    prompt: `Audit Java exception handling:

1. **Exception Swallowing**: Find empty catch blocks or catching Exception/Throwable
2. **Checked Exception Overuse**: Suggest converting to unchecked for runtime errors
3. **Resource Cleanup**: Check try-with-resources usage vs. manual finally blocks
4. **Custom Exceptions**: Review if custom exceptions add value or just noise
5. **Exception Chaining**: Ensure exceptions preserve cause (don't lose stack traces)
6. **Logging**: Check exceptions are logged with proper context before rethrowing
7. **Spring @ControllerAdvice**: Verify REST exception handling returns proper status codes

For each issue:
- Code location and problem
- Impact (lost information, resource leaks, unclear errors)
- Improved exception handling example`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['java', 'exceptions', 'error-handling']
  },
  {
    id: 'spring-security-audit',
    category: 'Java/Spring Boot',
    name: 'Spring Security Audit',
    description: 'Review Spring Security config: authentication, authorization, CSRF, CORS, session management, JWT',
    icon: 'lock',
    prompt: `Audit Spring Security configuration:

1. **Authentication**: Review UserDetailsService, password encoding (BCrypt), authentication providers
2. **Authorization**: Check @PreAuthorize/@Secured usage, role hierarchy, method security
3. **CSRF Protection**: Verify CSRF enabled for state-changing operations
4. **CORS Configuration**: Check CORS is not overly permissive (not allowAll)
5. **Session Management**: Review session fixation protection, concurrent session control
6. **JWT Implementation**: Check signature verification, expiration, refresh tokens, secure storage
7. **Password Policy**: Verify password complexity, expiration, history

For each security issue:
- Vulnerability type (auth bypass, CSRF, etc.)
- Exploitation scenario
- Secure configuration example`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['java', 'spring-security', 'security', 'critical']
  },
  {
    id: 'java-stream-api-best-practices',
    category: 'Java/Spring Boot',
    name: 'Java Stream API Best Practices',
    description: 'Review Stream usage: performance pitfalls, proper collectors, parallel streams, primitive streams',
    icon: 'activity',
    prompt: `Audit Java Stream API usage:

1. **Performance Pitfalls**: Find boxed streams where primitive streams should be used (IntStream)
2. **Collector Usage**: Check proper use of Collectors (toList, groupingBy, partitioningBy)
3. **Parallel Streams**: Verify parallel() is used appropriately (not for small datasets or blocking I/O)
4. **Stream Reuse**: Find attempts to reuse stream (should create new stream)
5. **Unnecessary Boxing**: Detect unnecessary autoboxing (use mapToInt instead of map)
6. **Readability**: Check overly complex stream chains that should be refactored
7. **Side Effects**: Find side effects in stream operations (should be avoided)

For each issue:
- Stream location and problem
- Performance or correctness impact
- Optimized stream code`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['java', 'streams', 'performance']
  },
  {
    id: 'spring-boot-actuator-setup',
    category: 'Java/Spring Boot',
    name: 'Spring Boot Actuator Setup',
    description: 'Review Actuator config: enabled endpoints, security, health indicators, custom metrics, monitoring integration',
    icon: 'analysis',
    prompt: `Audit Spring Boot Actuator configuration:

1. **Enabled Endpoints**: Check which endpoints are exposed (health, metrics, info) via management.endpoints
2. **Security**: Verify sensitive endpoints (env, beans, heapdump) are secured
3. **Health Indicators**: Review custom health indicators for dependencies (DB, Redis, external APIs)
4. **Custom Metrics**: Identify business metrics that should be exposed via Micrometer
5. **Monitoring Integration**: Check integration with Prometheus, Grafana, DataDog, etc.
6. **Info Endpoint**: Verify build info, git info are exposed for version tracking
7. **Management Port**: Check if using separate management port for actuator (production best practice)

For each finding:
- Configuration issue or missing metric
- Security or observability impact
- Proper configuration example`,
    executionStrategy: 'multi-agent-consensus',
    tags: ['java', 'spring-boot', 'actuator', 'monitoring']
  }
];

export const promptCategories = [
  'All',
  'Code Analysis',
  'Performance Analysis',
  'Security Analysis',
  'Testing Analysis',
  'Infrastructure Analysis',
  'Observability Analysis',
  'Migration Analysis',
  'UX/Accessibility Analysis',
  'Consistency Analysis',
  'Java/Spring Boot'
];
