# TASK

Merge the following branches into the current branch:

{{BRANCHES}}

For each branch:

1. Find the open PR for that branch:
   ```
   gh pr list --head <branch> --json number,title --jq '.[0]'
   ```

2. Push any local changes to the target branch:
   ```
   git push origin HEAD
   ```

3. Check the PR for merge conflicts:
   ```
   gh pr view <PR_NUMBER> --json mergeable
   ```

4. If there are conflicts, resolve them locally:
   ```
   git merge <branch>
   # Resolve conflicts in your editor
   git add .
   git commit -m "Resolve merge conflicts"
   git push origin HEAD
   ```
   The PR will automatically update on GitHub.

5. Merge via the PR on GitHub (do NOT merge locally):
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

6. After each merge, pull the updated main branch locally:
   ```
   git pull origin HEAD
   ```

7. Run the relevant tests to verify everything works:
   - **Go services**: `cd services/<service> && go test ./...`
   - **Frontend**: `cd web && npm run build && npm run lint`
   - **Frontend tests**: `cd web && npm test` when a test script exists or frontend tests are part of the merged branch
   - If tests fail, report the issue and do NOT continue merging

# CLOSE ISSUES

For each branch that was merged, close its issue:

```
gh issue close <ID> --comment "Completed by Sandcastle — merged via PR #<PR_NUMBER>"
```

Here are all the issues:

{{ISSUES}}

Once you've merged everything you can, output <promise>COMPLETE</promise>.
