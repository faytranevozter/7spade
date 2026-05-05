# TASK

Review the code changes on branch `{{BRANCH}}` and improve code clarity, consistency, and maintainability while preserving exact functionality.

# CONTEXT

## Branch diff

!`git diff {{SOURCE_BRANCH}}...{{BRANCH}}`

## Commits on this branch

!`git log {{SOURCE_BRANCH}}..{{BRANCH}} --oneline`

# REVIEW PROCESS

1. **Understand the change**: Read the diff and commits above to understand the intent.

2. **Analyze for improvements**: Look for opportunities to:
   - Reduce unnecessary complexity and nesting
   - Eliminate redundant code and abstractions
   - Improve readability through clear variable and function names
   - Consolidate related logic
   - Remove unnecessary comments that describe obvious code
   - Avoid nested ternary operators - prefer switch statements or if/else chains
   - Choose clarity over brevity - explicit code is often better than overly compact code

3. **Check correctness**:
   - Does the implementation match the intent? Are edge cases handled?
   - Are new/changed behaviours covered by tests?
   - Are there unsafe casts, `any` types, or unchecked assumptions in Go or TypeScript?
   - Does the change introduce injection vulnerabilities, credential leaks, or other security issues?
   - Go: are all errors handled explicitly (no discarded `_` errors in production code)?
   - Go: does the Game Engine remain free of I/O side effects?
   - TypeScript: is `any` avoided? Are types explicit?

4. **Maintain balance**: Avoid over-simplification that could:
   - Reduce code clarity or maintainability
   - Create overly clever solutions that are hard to understand
   - Combine too many concerns into single functions or components
   - Remove helpful abstractions that improve code organization
   - Make the code harder to debug or extend

5. **Apply project standards**: Follow the coding standards defined in @.sandcastle/CODING_STANDARDS.md

6. **Preserve functionality**: Never change what the code does - only how it does it. All original features, outputs, and behaviors must remain intact.

# EXECUTION

If you find improvements to make:

1. Make the changes directly on this branch
2. Run the relevant tests:
   - **Go services**: `cd services/<service> && go test ./...`
   - **Frontend**: `cd web && npm run typecheck && npm test`
3. Commit describing the refinements (prefix with `RALPH:`)

# POST REVIEW TO PR

After completing the review (whether or not you made changes), find the open PR for this branch and post a review summary:

```
gh pr review --comment --body "## Review Summary

### What was reviewed
<brief description of the change>

### Findings
<list issues found, or 'No issues found — code is clean and well-structured.'>

### Changes made
<list any refinements applied, or 'None — no changes needed.'>

### Verdict
APPROVED / CHANGES REQUESTED"
```

Use `gh pr list --head {{BRANCH}} --json number --jq '.[0].number'` to find the PR number first.

If the code is already clean and well-structured, approve it in the review.

Once complete, output <promise>COMPLETE</promise>.
