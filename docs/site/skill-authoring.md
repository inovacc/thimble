# Skill Authoring Guide

Skills are markdown files with YAML frontmatter that Claude Code can invoke as slash commands. When a user types `/plugin-name:skill-name` (or just `/skill-name` for built-in skills), Claude loads the skill's instructions and follows them to complete the task.

Skills let you package repeatable workflows, checklists, and domain-specific guidance into reusable prompts that any team member can invoke.

## Skill Format

Each skill lives in its own directory under `skills/` and contains a single `SKILL.md` file:

```
skills/
  my-skill/
    SKILL.md
```

A `SKILL.md` file has two parts:

1. **YAML frontmatter** -- metadata between `---` delimiters
2. **Markdown body** -- the instructions Claude follows when the skill is invoked

```markdown
---
name: my-skill
description: One-line summary of what the skill does and when to use it.
---

# My Skill

Instructions for Claude go here.
```

## Creating a Skill

### Step 1: Plan the workflow

Decide what the skill should accomplish. Good skills are focused on a single task or workflow — code review, deployment checks, documentation updates, etc.

### Step 2: Create the directory

```bash
mkdir -p skills/my-skill
```

### Step 3: Write the SKILL.md

Create `skills/my-skill/SKILL.md` with frontmatter and instructions:

```markdown
---
name: my-skill
description: Describe when Claude should use this skill.
---

# My Skill Title

Step-by-step instructions for Claude to follow.

## What to do

1. First, do this.
2. Then, do that.
3. Finally, report results.
```

### Step 4: Test the skill

Invoke the skill in Claude Code:

```
/thimble:my-skill
```

If the plugin is loaded locally via `claude --plugin-dir .`, changes to `SKILL.md` take effect on the next invocation without restarting.

### Step 5: Iterate

Refine the instructions based on how Claude interprets them. Add specificity where Claude makes wrong assumptions. Remove ambiguity.

## Frontmatter Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Skill identifier. Used in the slash command (`/plugin:name`). Must be lowercase, use hyphens for word separation. |
| `description` | string | Yes | Describes what the skill does and when to invoke it. Claude uses this to decide whether to suggest the skill. Should be a complete sentence or two. |
| `user-invocable` | boolean | No | When `true`, the skill appears in the slash command list for manual invocation. Defaults to `true` for most skills. |

### Description field

The `description` field is critical. It serves two purposes:

1. **Display** -- shown to the user in skill listings
2. **Routing** -- Claude reads it to decide if the skill matches a user request

Write descriptions that include trigger phrases. For example:

```yaml
description: |
  Run deployment preflight checks before pushing to production.
  Use when the user says "deploy", "release", or "push to prod".
```

### Multi-line descriptions

Use YAML block scalar syntax for longer descriptions:

```yaml
description: |
  Connect thimble to a cloud sync endpoint.
  Guides through API URL, token, and org ID configuration.
  Saves config to ~/.thimble/sync.json and tests the connection.
  Trigger: /thimble:cloud-setup
```

## Markdown Body

The body contains the instructions Claude follows. Write them as if you were giving a senior engineer a runbook:

- **Be specific** -- name the exact tools, commands, or MCP calls to use
- **Use numbered steps** -- Claude follows ordered lists more reliably than prose
- **Include conditionals** -- handle success and failure paths
- **Use `$ARGUMENTS`** -- this variable contains whatever the user typed after the slash command
- **Show expected output** -- use code blocks to show what success and failure look like

### Using $ARGUMENTS

When a user invokes `/thimble:my-skill some input here`, the text `some input here` is available as `$ARGUMENTS`. Reference it in your instructions:

```markdown
If `$ARGUMENTS` is provided, analyze that file path.
Otherwise, analyze the current project root.
```

### Referencing MCP tools

If your skill relies on MCP tools (thimble or otherwise), name them explicitly:

```markdown
Use `ctx_search` to find relevant code.
Use `ctx_execute` to run the linter.
```

## Best Practices

### Keep prompts focused

Each skill should do one thing well. If a workflow has multiple phases, consider splitting it into separate skills. A deploy-check skill and a deploy-execute skill are better than one mega-deploy skill.

### Use conditional logic

Handle different scenarios in the instructions:

```markdown
If the project has a `Dockerfile`, check for:
- Multi-stage builds
- Non-root user
- .dockerignore presence

If the project has no `Dockerfile`, skip container checks.
```

### Provide structure for output

Tell Claude what format to use for results:

```markdown
Display results as a markdown checklist:
- [x] Passing check
- [ ] Failing check — explanation
```

### Test iteratively

Invoke your skill with `/thimble:skill-name` and observe Claude's behavior. Common issues:

- **Too vague** -- Claude improvises instead of following your steps
- **Too rigid** -- Claude cannot adapt to project variations
- **Missing error handling** -- Claude gets stuck when a command fails

### Security considerations

- Never embed secrets, tokens, or credentials in skill files
- If a skill handles sensitive data, include masking instructions (e.g., "show only the last 4 characters")
- Set file permissions in instructions when writing config files (`chmod 600`)

## Example Skills

### Code Review Skill

```markdown
---
name: review
description: |
  Perform a structured code review on staged changes.
  Use when the user asks for a code review, CR, or review.
---

# Code Review

Review the staged git changes using a structured checklist.

## Steps

1. Run `git diff --cached --stat` to see which files are staged.
   If nothing is staged, run `git diff --stat` for unstaged changes instead.

2. For each changed file, evaluate:

   **Correctness**
   - Does the logic match the intent?
   - Are edge cases handled?
   - Are error paths covered?

   **Style**
   - Does the code follow project conventions?
   - Are names clear and consistent?
   - Is there unnecessary complexity?

   **Security**
   - Are inputs validated?
   - Are secrets handled safely?
   - Are there injection risks?

   **Testing**
   - Are new code paths tested?
   - Do existing tests still pass?
   - Are there missing test cases?

3. Display findings as a markdown report:

   ## Code Review Summary

   | File | Issues | Severity |
   |------|--------|----------|
   | ... | ... | ... |

   ### Findings

   For each issue:
   - File and line reference
   - What the issue is
   - Suggested fix

   ### Verdict

   One of: APPROVE, REQUEST CHANGES, or COMMENT.
```

### Deploy Checklist Skill

```markdown
---
name: deploy-check
description: |
  Run a preflight checklist before deploying to production.
  Use when the user says "deploy check", "preflight", or "ready to deploy".
---

# Deploy Preflight Checklist

Run through deployment prerequisites before pushing to production.

## Steps

1. **Branch check** -- verify the current branch:
   - Run `git branch --show-current`
   - If not on `main` or `release/*`, warn the user

2. **Clean working tree** -- check for uncommitted changes:
   - Run `git status --porcelain`
   - If output is non-empty, STOP and tell the user to commit or stash

3. **Tests pass** -- run the test suite:
   - Check for `Taskfile.yml` and run `task test`, OR
   - Check for `Makefile` and run `make test`, OR
   - Run `go test ./...` for Go projects
   - If tests fail, STOP and show failures

4. **Lint clean** -- run linters:
   - Run `golangci-lint run ./...` for Go, or the appropriate linter
   - If lint errors exist, list them

5. **No TODOs in diff** -- check for leftover TODOs:
   - Run `git diff main...HEAD` and search for `TODO`, `FIXME`, `HACK`
   - List any found with file and line

6. **Changelog updated** -- verify documentation:
   - Check if `CHANGELOG.md` or `docs/ROADMAP.md` has been modified in this branch
   - If not, warn that release notes may be missing

7. **Display results** as a checklist:

   ## Deploy Preflight

   - [x] Branch: main
   - [x] Working tree: clean
   - [x] Tests: 142 passed
   - [ ] Lint: 2 warnings (list them)
   - [x] No leftover TODOs
   - [ ] Changelog: not updated

   **Recommendation**: Fix lint warnings and update changelog before deploying.
```

### Documentation Audit Skill

```markdown
---
name: docs-audit
description: |
  Audit project documentation for completeness and accuracy.
  Use when the user asks to check docs, review documentation,
  or verify README accuracy.
---

# Documentation Audit

Evaluate the project's documentation for completeness, accuracy,
and consistency with the actual codebase.

## Steps

1. **Inventory existing docs** -- list all markdown files:
   - Check for: README.md, CLAUDE.md, CHANGELOG.md
   - Check `docs/` directory for additional documentation
   - Note any missing standard files

2. **README accuracy** -- if README.md exists:
   - Verify install commands actually work
   - Check that listed features match the codebase
   - Verify code examples are syntactically valid
   - Check that badge URLs resolve (if any)

3. **API documentation** -- if the project exposes an API:
   - Check that all public endpoints are documented
   - Verify request/response examples match the code
   - Look for undocumented parameters

4. **Code comments** -- sample 5 key files:
   - Check that exported functions have doc comments
   - Verify comments match current behavior (not stale)

5. **Display results**:

   ## Documentation Audit

   ### Coverage

   | Document | Status | Notes |
   |----------|--------|-------|
   | README.md | Present | Install section outdated |
   | CLAUDE.md | Present | OK |
   | CHANGELOG.md | Missing | Should be created |
   | docs/ARCHITECTURE.md | Present | Diagram needs update |

   ### Issues

   List each issue with:
   - File and section
   - What is wrong or missing
   - Suggested fix

   ### Score

   Rate documentation completeness: X/10 with a one-line justification.
```

## Publishing Skills in a Plugin

To include skills in a thimble plugin, add a `skills` field to your `plugin.json` manifest that points to a directory containing skill subdirectories:

```json
{
  "name": "my-plugin",
  "version": "1.0.0",
  "description": "My custom plugin with skills and tools",
  "skills": "./skills/",
  "tools": [...]
}
```

### Directory structure

```
my-plugin/
  plugin.json
  skills/
    my-skill/
      SKILL.md
    another-skill/
      SKILL.md
```

### Naming conventions

- Skill directory names should match the `name` field in frontmatter
- Use lowercase with hyphens: `deploy-check`, not `DeployCheck` or `deploy_check`
- When published as part of a plugin, users invoke skills as `/plugin-name:skill-name`

### Skills vs tools

Skills and tools serve different purposes in a plugin:

| | Skills | Tools |
|---|--------|-------|
| **Format** | Markdown with instructions | JSON with shell commands |
| **Execution** | Claude follows the instructions | MCP server runs the command |
| **Input** | Free-form `$ARGUMENTS` | Structured JSON schema |
| **Use case** | Workflows, checklists, guidance | Atomic operations, data retrieval |

Use skills for workflows that require judgment and adaptation. Use tools for deterministic operations that always run the same command.

### Marketplace submission

To submit a plugin with skills to the thimble marketplace:

1. Host your plugin on GitHub
2. Ensure `plugin.json` passes validation: `thimble plugin validate plugin.json`
3. Submit a PR to [inovacc/thimble-plugins](https://github.com/inovacc/thimble-plugins) with your plugin metadata
