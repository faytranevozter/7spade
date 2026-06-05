---
name: fullstack-dev
description: Use this skill when developing, debugging, fixing, testing, or verifying a local full-stack application with a Golang backend (running via `air`) and a React frontend (running via `pnpm dev`). Covers bug fixes, new features, UI/UX changes, REST API changes, WebSocket behavior, and end-to-end verification using Playwright, Chrome DevTools MCP, or other available tools. Trigger on any Go + React local dev task.
---

# Local Full-Stack Development & Verification

## Purpose

Use this skill for any work on a local Go + React full-stack application:

- Reproducing and fixing bugs
- Building or updating features
- Improving UI or UX
- Modifying API or WebSocket behavior
- Refactoring a focused area
- Verifying changes in the running app

Deliver working, verified changes with minimal disruption to the local dev environment.

## Local Environment Assumptions

The application is already running locally:

- **Golang backend** — running via `air` (hot reload active)
- **WebSocket service** — running via `air` (hot reload active)
- **React frontend** — running via `pnpm dev` (hot reload active)

Do not restart any service unless the user explicitly asks.

## Core Rules

1. Understand the request before touching code.
2. Inspect the existing implementation and follow project patterns.
3. Reproduce the issue or validate baseline behavior when applicable.
4. Make the smallest safe change that satisfies the request.
5. Preserve architecture, naming conventions, style, and API contracts.
6. Avoid unrelated refactors, formatting churn, or broad rewrites.
7. Update related types, interfaces, schemas, validation, or tests when needed.
8. Verify behavior after every change.

## Bug Fix Workflow

1. Reproduce the issue using available tools.
2. Capture evidence: browser behavior, console errors, network requests, API responses, WebSocket frames, backend logs.
3. Identify the root cause before editing code.
4. Fix the root cause, not just the symptom.
5. Re-test the exact flow that reproduced the bug.
6. Check nearby behavior for regressions.

If the issue cannot be reproduced, clearly state what was tested and observed.

## New Feature Workflow

1. Inspect the existing related implementation.
2. Locate relevant components, hooks, services, routes, handlers, types, API endpoints, WebSocket events, or database logic.
3. Follow project conventions throughout.
4. Implement the smallest coherent set of changes.
5. Reuse existing utilities, components, and patterns when appropriate.
6. Add validation, loading states, empty states, and error handling where relevant.
7. Keep UX consistent with the existing application.
8. Verify through the real UI, API flow, WebSocket flow, or targeted tests.
9. Confirm existing related behavior still works.

## Tool Usage

Prefer:

- **Playwright** — browser reproduction, feature testing, regression checks
- **Chrome DevTools MCP** — DOM inspection, console logs, network requests, WebSocket frames (Network → WS tab), storage, cookies
- **Terminal** — targeted tests, type checks, lint checks, backend logs
- **File search / code nav** — locate actual source of behavior

For UI work, always verify in the browser — code inspection alone is not enough.
For WebSocket work, capture actual frames as evidence before and after changes.

## Verification Requirements

After making changes, verify using one or more of:

- Playwright browser flow
- Chrome DevTools MCP validation
- API request/response validation
- WebSocket event capture
- Type check / lint check
- Backend or frontend logs
- Targeted unit or integration tests

**For bugs:** confirm the original issue no longer occurs.  
**For features:** confirm the feature works through the expected user flow.

The task is not complete until the changed behavior is verified.

## Final Response Format

### Summary
What was implemented, fixed, or changed.

### Reproduction or Baseline
How the bug was reproduced, or what baseline behavior was inspected for new features.

### Root Cause or Implementation Approach
Root cause for bugs. Approach taken for features.

### Changes Made
Files or areas changed, with brief summaries.

### Verification
How the result was tested — tools used, flows exercised, evidence captured.

### Result
Pass or fail. If failed, what remains.

### Notes *(optional)*
Remaining risks, assumptions, or follow-up items when relevant.