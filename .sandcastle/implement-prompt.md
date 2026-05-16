# TASK

Fix issue {{TASK_ID}}: {{ISSUE_TITLE}}

Pull in the issue using `gh issue view <ID> --comments`. If it has a parent PRD, pull that in with comments too.

Issue comments may contain newer acceptance criteria, design constraints, triage notes, blockers, or maintainer decisions. Treat relevant comments as part of the task context unless they clearly conflict with a newer maintainer instruction.

Only work on the issue specified.

Work on branch {{BRANCH}}. Make commits and run tests.

# CONTEXT

First, ensure you have the latest `main`:

!`git fetch origin main && git checkout main && git pull origin main`

Then create and switch to branch {{BRANCH}} off the latest `main`.
If the branch already exists, switch to it and rebase onto the latest `main` to stay up to date:

!`git checkout {{BRANCH}} 2>/dev/null && git rebase main || git checkout -b {{BRANCH}}`

Here are the last 10 commits:

<recent-commits>

!`git log -n 10 --format="%H%n%ad%n%B---" --date=short`

</recent-commits>

# EXPLORATION

Explore the repo and fill your context window with relevant information that will allow you to complete the task.

Pay extra attention to test files that touch the relevant parts of the code.

For frontend work in `web/`, read @design/design_system.html before implementation. The UI must follow the Seven Spade design system and use Tailwind CSS v4.2 via the Vite plugin (`tailwindcss` + `@tailwindcss/vite`) with `@import "tailwindcss";` in the CSS entry. Preserve the dark compact game-table visual language, DM Sans/DM Mono typography, card states, status badges, room cards, and board layout tokens from the design system.

# EXECUTION

Before starting implementation, ensure your branch is based on the latest `main` — fetch the remote and rebase onto `origin/main` (or merge if rebase conflicts are too complex). Do not start implementing against a stale base.

If applicable, use RGR to complete the task.

1. RED: write one test
2. GREEN: write the implementation to pass that test
3. REPEAT until done
4. REFACTOR the code

# FEEDBACK LOOPS

Before committing, run the appropriate tests for the services you changed:

- **Go services** (`services/api` or `services/ws`): `make -C services/<service> test`
- **Frontend** (`web/`): `make -C web check`
- **Frontend tests**: run `make -C web test` when a test script exists or frontend tests are added

# COMMIT

Make a git commit. The commit message must:

1. Start with `RALPH:` prefix
2. Include task completed + issue reference (e.g. `Closes #{{TASK_ID}}`)
3. Key decisions made
4. Files changed
5. Blockers or notes for next iteration

Keep it concise.

# PUSH & PULL REQUEST

After committing:

1. Push the branch to remote origin:
   ```
   git push -u origin {{BRANCH}}
   ```

2. Create a pull request:
   ```
   gh pr create \
     --title "{{ISSUE_TITLE}}" \
     --body "Closes #{{TASK_ID}}

   ## Summary
   <describe what was implemented and why>

   ## Changes
   <list key files changed>

   ## Decisions
   <note any non-obvious decisions>" \
     --base main \
     --head {{BRANCH}}
   ```

3. Output the PR URL.

# THE ISSUE

If the task is not complete, leave a comment on the issue with what was done.

Do not close the issue - this will be done later via the PR merge.

Once complete, output <promise>COMPLETE</promise>.

# FINAL RULES

ONLY WORK ON A SINGLE TASK.
