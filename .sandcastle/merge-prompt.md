# TASK

Merge the following branches into the current branch:

{{BRANCHES}}

For each branch:

1. Find the open PR for that branch:
   ```
   gh pr list --head <branch> --json number,title --jq '.[0]'
   ```

2. Merge via the PR with a summary comment:
   ```
   gh pr merge <PR_NUMBER> --merge --subject "Merge: <PR title>" --body "## Merge Summary

   **Issue**: #<id>
   **Branch**: <branch>

   ### What was merged
   <brief description of changes>

   ### Key decisions
   <any notable decisions or trade-offs>

   ### Test status
   All tests passing."
   ```

3. If there is no open PR (e.g. branch was pushed without one), fall back to:
   ```
   git merge <branch> --no-edit
   ```
   Then resolve any conflicts intelligently by reading both sides and choosing the correct resolution.

4. After each merge, run the relevant tests to verify everything works:
   - **Go services**: `cd services/<service> && go test ./...`
   - **Frontend**: `cd web && npm run build && npm run lint`
   - **Frontend tests**: `cd web && npm test` when a test script exists or frontend tests are part of the merged branch
   - If tests fail, fix the issues before proceeding to the next branch.

# CLOSE ISSUES

For each branch that was merged, close its issue:

```
gh issue close <ID> --comment "Completed by Sandcastle — merged via PR #<PR_NUMBER>"
```

Here are all the issues:

{{ISSUES}}

Once you've merged everything you can, output <promise>COMPLETE</promise>.
