# ISSUES

Here are the open issues in the repo:

<issues-json>

!`gh issue list --state open --label Sandcastle --json number,title,body,labels,comments --jq '[.[] | {number, title, body, labels: [.labels[].name], comments: [.comments[].body]}]'`

</issues-json>

# TASK

Analyze the open issues and build a dependency graph. For each issue, determine whether it **blocks** or **is blocked by** any other open issue.

An issue B is **blocked by** issue A if:

- B requires code or infrastructure that A introduces
- B and A modify overlapping files or modules, making concurrent work likely to produce merge conflicts
- B's requirements depend on a decision or API shape that A will establish

An issue is **unblocked** if it has zero blocking dependencies on other open issues.

For each unblocked issue, assign a branch name using the format `sandcastle/issue-{id}-{slug}`.

For frontend issues, account for the shared design dependency: work in `web/` must follow @design/design_system.html and use Tailwind CSS v4.2. Treat a Tailwind/design-system foundation issue as blocking other frontend UI issues if it establishes shared tokens, layout primitives, or package setup.

# OUTPUT

Output your plan as a JSON object wrapped in `<plan>` tags:

<plan>
{"issues": [{"id": "42", "title": "Fix auth bug", "branch": "sandcastle/issue-42-fix-auth-bug"}]}
</plan>

Include only unblocked issues. If every issue is blocked, include the single highest-priority candidate (the one with the fewest or weakest dependencies).
