# AGENTS.md

## 1. Mission

You are working on **TX Carpool**, a carpool-management panel built around upstream **Remnawave** and **Emby** services.

Complete all assigned tasks **one by one** in a sensible dependency order.

For every task:

1. Understand the requirement and inspect the relevant existing implementation.
2. Check applicable skills, upstream APIs, references, and existing components.
3. Implement the task.
4. Audit the changed code.
5. Validate the affected frontend behavior when applicable.
6. Confirm the task is complete before proceeding to the next task.

Do not leave partially implemented tasks unless blocked by an external dependency.

---

## 2. Instruction Precedence

Follow instructions in this order:

1. Repository-specific instructions in this `AGENTS.md`
2. The following required skills, in this exact precedence order:
   1. `I have ADHD`
   2. `Minimalist skill`
   3. `taste skill`
   4. `Telegram mini app`
   5. `nuxt-ui`
   6. `vue-best-practices`
   7. `golang-pro`
3. Official upstream documentation under `./reference`
4. Existing project conventions
5. General framework/library best practices

When two instructions conflict, follow the instruction with the higher precedence.

Use the following MCP sources when framework documentation or implementation details need to be checked:

- `Nuxt UI`
- `Vue Docs`

Do not guess framework or component APIs when they can be verified through the available references.

---

## 3. Product and UI Direction

### 3.1 Design

The member Mini App must be mobile-first and premium-dark. The standalone browser
admin console must be **desktop-first with mobile compatibility**, independent of
Mini App styling, and support system light/dark appearance with a saved manual override.

Both interfaces must be:

- fully usable on desktop and mobile
- visually minimal
- low in visual noise
- consistent across pages
- optimized for clarity and fast interaction

Prefer restrained layouts, clear hierarchy, appropriate spacing, and purposeful animation.

Avoid:

- unnecessary decoration
- excessive gradients
- excessive borders
- excessive cards
- oversized headings
- redundant information
- unnecessary confirmation steps
- visual complexity without functional value

### 3.2 Component Preference

Always prefer implementations in this order:

1. Native platform/browser functionality
2. Existing project components
3. Nuxt / Vue native capabilities
4. Nuxt UI components
5. Existing approved libraries
6. Custom implementation only when none of the above sufficiently solves the problem

**Do not reinvent existing components or utilities.**

Potential frontend dependencies include:

- Nuxt UI v4
- Nuxt Icon
- Zod
- AutoAnimate
- TanStack Table

Do not add a dependency merely because it is listed here. Add or use it only when it provides a concrete benefit.

---

## 4. Architecture

Code must be modular, maintainable, and extensible.

### 4.1 General Structure

Separate:

- application/bootstrap entry points
- feature modules
- shared components
- shared utilities
- integrations
- API clients
- data models
- configuration

Keep business logic out of entry-point files.

Do not create large monolithic components or packages.

### 4.2 File Size

Keep source files at **200 lines or fewer whenever reasonably possible**.

If a file would exceed 200 lines, split it by responsibility into smaller modules.

Generated files, lockfiles, migrations, schemas, and files whose format inherently requires greater length are exempt when splitting would reduce maintainability.

### 4.3 Folder Structure

Organize implementation into meaningful folders.

Avoid creating folders containing only one implementation file when the contents can logically live in an existing module.

For newly introduced feature/module directories, prefer **at least two meaningful files** per directory.

Do not create filler files merely to satisfy directory structure requirements.

### 4.4 Module Documentation

After implementing or materially changing a module, document:

- module responsibility
- entry points
- major dependencies
- public interfaces
- data flow
- external API usage
- extension points
- important implementation constraints

Keep module documentation close to the module.

### 4.5 README Requirements

Each meaningful frontend or backend subsystem/module directory should contain or inherit clear documentation.

At minimum, maintain appropriate `README.md` documentation under major frontend and backend submodules.

Do not create redundant README files containing no useful information.

---

## 5. Frontend Rules

### 5.1 Internationalization

**Never hardcode user-facing text in frontend source code.**

This includes:

- labels
- buttons
- titles
- descriptions
- validation messages
- empty states
- errors
- success messages
- dialogs
- placeholders
- tooltips

Add user-facing strings to the appropriate language/i18n files and reference them through the project's localization system.

Technical identifiers, debug messages, protocol values, and data keys are not considered user-facing text.

### 5.2 Responsiveness

Design member Mini App views for mobile first. Design browser admin views for
desktop workflows first, then support mobile with accessible sidebar navigation,
stacked forms, and contained table scrolling.

Every affected view must then be checked for reasonable behavior on desktop layouts.

Keep the two experience roots, routers, initialization, and theme assets separate.
Browser admin views must not use Telegram Web App SDK controls, haptics, host
buttons, or WebView compatibility initialization. Existing backend Telegram
services remain available through browser controls.

The Mini App may expose only `/settings/admin-access` for admins: OAuth setup and
short-lived one-use console links. Other administrator pages and their resource
preloads belong exclusively to the browser console. Browser admin sessions must
remain independent of Mini App sessions and other browser devices.

### 5.3 Validation

Use existing validation mechanisms first.

Where appropriate, use **Zod** for structured frontend validation rather than creating another validation system.

### 5.4 Animation

Animations must:

- clarify state changes
- provide interaction feedback
- remain lightweight
- avoid delaying user actions

Use existing/native transitions first. Use AutoAnimate when it meaningfully simplifies list/layout transitions.

---

## 6. Backend Rules

### 6.1 Go

Follow established Go conventions and the `golang-pro` skill.

Prefer:

- small packages with clear responsibility
- explicit error handling
- contextual errors
- dependency injection where useful
- typed models
- context propagation
- bounded concurrency
- structured logging

Avoid unnecessary abstraction.

### 6.2 Data Storage

**Do not store unnecessary information in the database.**

Before adding a column, table, cache, or duplicated record:

1. Check whether the information already exists upstream.
2. Check whether it can be derived cheaply.
3. Check whether an existing relation/reference is sufficient.
4. Store it locally only when persistence is actually required.

Prefer references and upstream identifiers over duplicated upstream data.

Do not unnecessarily mirror Remnawave or Emby datasets locally.

---

## 7. Upstream Integrations

The primary upstream projects are:

- **Remnawave**
- **Emby**

Their API documentation is located under:

```text
./reference
```

Treat those documents as authoritative.

### 7.1 API Compliance

Follow upstream API documentation **strictly**.

Do not:

- invent endpoints
- assume undocumented fields
- assume undocumented status codes
- rely on undocumented request formats
- infer behavior when the documentation can be consulted

Reuse upstream capabilities whenever practical instead of reproducing them locally.

### 7.2 Backend API Queue

All outbound API calls performed by the **backend** to upstream/external services must pass through the project's request/job queue before execution.

Backend code must not directly invoke Remnawave, Emby, or other queued upstream integrations from arbitrary handlers or business modules.

The expected flow is conceptually:

```text
request / business action
        ↓
queue
        ↓
worker / executor
        ↓
upstream API
        ↓
result handling
```

Centralize:

- retry policy
- rate limiting
- concurrency limits
- failure handling
- observability

around the queue/executor layer whenever applicable.

Do not introduce a second competing queue mechanism if the repository already contains one.

---

## 8. Implementation Workflow

For a multi-task request:

1. Inventory all requested tasks.
2. Identify dependencies between them.
3. Arrange them into the safest implementation order.
4. Complete **one task at a time**.
5. Audit each task before moving on.
6. After all tasks are implemented, perform a final cross-task audit.

Do not jump randomly between unrelated tasks.

When modifying an existing feature, inspect its surrounding implementation before introducing a new pattern.

Prefer extending the existing architecture over creating parallel systems.

---

## 9. Testing and Validation

### 9.1 Local Test Restriction

**Do not run local automated test suites.**

Do not execute commands such as:

```bash
npm test
pnpm test
npm run test
pnpm vitest
go test ./...
pytest
```

or equivalent local automated test runners unless a later explicit instruction overrides this restriction.

Do not modify this rule by interpreting “audit” as permission to run tests.

### 9.2 Allowed Static Review

You must still audit changed code through methods that do not execute prohibited local test suites, including:

- code inspection
- type/interface reasoning
- import/dependency review
- control-flow review
- API contract review
- schema review
- i18n review
- error-path review
- security review
- lint/static tooling only when permitted by the repository instructions

### 9.3 Frontend Browser Validation

After frontend changes, validate the affected flows using **Chrome DevTools MCP**.

Use constructed/mock-compatible data when necessary to exercise the UI.

Check at least:

- initial render
- loading state
- populated state
- empty state
- relevant error states
- interaction behavior
- mobile layout
- desktop layout
- browser console errors
- obvious network/request failures

Chrome DevTools MCP validation is required and is **not** considered running a local automated test suite.

Do not claim frontend validation succeeded unless it was actually performed.

---

## 10. Code Audit Checklist

After editing code, inspect all changed areas for:

- correctness
- incomplete implementations
- dead code
- duplicated logic
- unnecessary dependencies
- oversized files
- misplaced responsibilities
- missing error handling
- race/concurrency issues
- security issues
- incorrect API assumptions
- direct backend API calls bypassing the queue
- unnecessary database persistence
- hardcoded frontend text
- missing translations
- broken responsive behavior
- inaccessible interaction patterns
- inconsistent naming
- missing module documentation

Also check that unrelated behavior was not unintentionally changed.

---

## 11. Completion Checklist

Before declaring the overall request complete, verify that:

- [ ] All requested tasks were identified.
- [ ] Tasks were completed in dependency-aware order.
- [ ] Every requested task is implemented.
- [ ] Required skills were followed.
- [ ] Relevant `./reference` API documentation was followed.
- [ ] Existing/native components were reused where possible.
- [ ] No unnecessary replacement components were created.
- [ ] Frontend user-facing text is localized.
- [ ] Mobile-first behavior is correct.
- [ ] Desktop behavior is supported.
- [ ] Backend upstream API calls go through the queue.
- [ ] No unnecessary information was added to the database.
- [ ] Changed modules remain reasonably modular.
- [ ] Source files are within the intended size limits or have a justified exemption.
- [ ] Module documentation is updated.
- [ ] Relevant README documentation is updated.
- [ ] Changed code was audited.
- [ ] No prohibited local automated tests were run.
- [ ] Frontend changes were validated with Chrome DevTools MCP when applicable.
- [ ] No obvious browser-console or request errors remain.
- [ ] Git diff contains no accidental/unrelated changes.

Do not mark the work complete while any applicable item remains unresolved.

---

## 12. Git and CI

After implementation and audit:

1. Review the complete diff.
2. Ensure unrelated files are not included.
3. Commit all intended changes with a clear commit message.
4. Push the completed work to the `main` branch.
5. Monitor the resulting CI / GitHub Actions workflows.
6. If a workflow fails:
   - inspect the failure,
   - identify the actual cause,
   - fix issues caused by or relevant to the submitted changes,
   - commit and push the fix,
   - monitor CI again.
7. Continue until the relevant workflow completes successfully or the failure is demonstrably caused by an external/unrelated condition.

Never use destructive Git operations merely to make CI pass.

Do not silently discard existing user changes.

---

## 13. General Engineering Principles

Throughout the repository:

- Prefer simple solutions.
- Reuse before creating.
- Extend before replacing.
- Keep responsibilities explicit.
- Avoid speculative abstractions.
- Avoid premature optimization.
- Avoid duplicated state.
- Avoid hidden side effects.
- Keep APIs typed and predictable.
- Preserve backward compatibility unless the task explicitly requires a breaking change.
- Follow existing conventions unless there is a concrete reason to improve them.
- Document non-obvious decisions.
- Make changes only as broad as required to complete the task correctly.
