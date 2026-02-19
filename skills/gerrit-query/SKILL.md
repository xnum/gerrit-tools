# Gerrit Query Tool (`gerrit-cli`) Usage Skill

## Overview

You have access to a CLI tool called `gerrit-cli` that allows you to query and interact with the Gerrit code review system. Use this tool to fetch information about changes, patchsets, files, and comments.

**Tool Location**: `gerrit-cli`

---

## Key Principles

1. **Use `gerrit-cli` from PATH**: avoid host-specific absolute paths
2. **Output is JSON by default**: All commands return structured JSON
3. **Quote search queries**: Use double quotes for queries with spaces
4. **Check success field**: Always check `"success": true` in the response

---

## Command Structure

```bash
gerrit-cli <resource> <action> [flags] [args]
```

**Resources**: `change`, `patchset`, `comment`, `draft`, `review`

---

## Common Commands

### 1. List Changes (Query)

**Search for open changes:**
```bash
gerrit-cli change list "status:open"
```

**Search by project:**
```bash
gerrit-cli change list "status:open project:gobe"
```

**Search by owner:**
```bash
gerrit-cli change list "owner:john status:open"
```

**Limit results:**
```bash
gerrit-cli change list "status:open" --limit 5
```

**Common query operators:**
- `status:open` - Open changes
- `status:merged` - Merged changes
- `project:NAME` - Specific project
- `owner:USER` - By owner username
- `branch:master` - Specific branch
- `topic:TOPIC` - By topic

**Output example:**
```json
{
  "success": true,
  "data": [
    {
      "id": "gobe~10661",
      "project": "gobe",
      "branch": "master",
      "subject": "Add new feature",
      "_number": 10661,
      "status": "NEW"
    }
  ]
}
```

### 2. Get Change Details

**Fetch detailed information about a specific change:**
```bash
gerrit-cli change get 10661
```

**Output includes:**
- Full change metadata
- Current revision
- Labels (Code-Review votes)
- Owner information
- Commit message

**Output example:**
```json
{
  "success": true,
  "data": {
    "id": "gobe~10661",
    "project": "gobe",
    "branch": "master",
    "subject": "Add new feature",
    "_number": 10661,
    "status": "NEW",
    "owner": {
      "name": "John Doe",
      "email": "john@example.com"
    },
    "current_revision": "abc123...",
    "labels": {
      "Code-Review": {
        "all": [
          {"name": "Reviewer", "value": 1}
        ]
      }
    }
  }
}
```

### 3. List Files in a Patchset

**List all changed files:**
```bash
gerrit-cli patchset diff 10661 --list-files
```

**Output example:**
```json
{
  "success": true,
  "data": {
    "src/main.go": {
      "status": "M",
      "lines_inserted": 10,
      "lines_deleted": 2
    },
    "README.md": {
      "status": "M",
      "lines_inserted": 5,
      "lines_deleted": 1
    }
  }
}
```

**File status codes:**
- `M` - Modified
- `A` - Added
- `D` - Deleted
- `R` - Renamed

### 4. Get Diff for a Specific File

**Fetch the diff for a specific file:**
```bash
gerrit-cli patchset diff 10661 --file src/main.go
```

**Output includes:**
- Line-by-line diff content
- Added lines (in `b` field)
- Removed lines (in `a` field)
- Context lines (in `ab` field)

### 5. List Comments

**Get all comments on a change:**
```bash
gerrit-cli comment list 10661
```

**Filter by file:**
```bash
gerrit-cli comment list 10661 --file src/main.go
```

**Only unresolved comments:**
```bash
gerrit-cli comment list 10661 --unresolved
```

**Output example:**
```json
{
  "success": true,
  "data": {
    "src/main.go": [
      {
        "id": "comment-id",
        "line": 42,
        "message": "Consider adding error handling here",
        "author": {
          "name": "Reviewer"
        },
        "unresolved": true
      }
    ]
  }
}
```

### 6. Post a Review

**Post a review with a message and vote:**
```bash
gerrit-cli review post 10661 \
  --message "Looks good to me!" \
  --vote 1
```

**Post review with inline comments:**
```bash
gerrit-cli review post 10661 \
  --message "Please address the comments" \
  --vote 0 \
  --comment "src/main.go:42:Add error handling here"
```

**Vote values:**
- `-1` - I would prefer this is not submitted as is
- `0` - No score (informational only)
- `+1` - Looks good to me, but someone else must approve
- `+2` - Looks good to me, approved (if you have permission)

---

## 🎯 Draft Comments (Build Review Incrementally)

### What are Draft Comments?

Draft comments are **private comments** that only you can see until you publish them. This allows you to:
- Build up a review incrementally as you analyze files
- Modify or delete comments before publishing
- Organize your thoughts without committing to them immediately

### Priority Levels and Auto-Unresolved

When creating drafts, use **priority prefixes** to categorize comments:

**Format**: `[PRIORITY] [TYPE] Message`

| Priority | Meaning | Unresolved | Use Case |
|----------|---------|------------|----------|
| `[P0] 👎` | Critical | ✅ Yes (blocks merge) | Security vulnerabilities, data loss, crashes |
| `[P1] 👎` | High | ✅ Yes (blocks merge) | Bugs, incorrect logic, missing error handling |
| `[P2] 👎` | Medium | ❌ No (doesn't block) | Code quality, maintainability suggestions |
| `[P3] 👍` | Low | ❌ No (doesn't block) | Style, minor optimizations, positive feedback |

**The tool automatically sets `unresolved: true/false` based on the priority prefix!**

### 1. Create Draft Comment

**Basic usage:**
```bash
gerrit-cli draft create <change-id> <file> <line> "<message>"
```

**Examples:**
```bash
# Critical issue (auto-unresolved)
gerrit-cli draft create 10661 src/main.go 42 "[P0] 👎 SQL injection vulnerability - use parameterized queries"

# High priority issue (auto-unresolved)
gerrit-cli draft create 10661 src/auth.go 15 "[P1] 👎 Missing error handling for database connection"

# Medium priority suggestion (auto-resolved)
gerrit-cli draft create 10661 src/utils.go 50 "[P2] 👎 Consider extracting this logic to a separate function"

# Positive feedback (auto-resolved)
gerrit-cli draft create 10661 src/utils.go 88 "[P3] 👍 Good use of defer for resource cleanup"

# Create on specific patchset (not current)
gerrit-cli draft create 10661 src/main.go 42 "[P1] 👎 Issue here" 3

# Override auto-detection (force unresolved even for P3)
gerrit-cli draft create 10661 src/main.go 50 "[P3] 👎 Minor but important" --unresolved
```

**Output:**
```json
{
  "success": true,
  "data": {
    "id": "abc123",
    "path": "src/main.go",
    "line": 42,
    "message": "[P0] 👎 SQL injection vulnerability - use parameterized queries",
    "unresolved": true,
    "updated": "2026-02-11T10:30:00Z"
  }
}
```

### 2. List Draft Comments

**List all drafts:**
```bash
gerrit-cli draft list 10661
```

**Filter by file:**
```bash
gerrit-cli draft list 10661 --file src/main.go
```

**Only unresolved drafts:**
```bash
gerrit-cli draft list 10661 --unresolved
```

**Output:**
```json
{
  "success": true,
  "data": {
    "src/main.go": [
      {
        "id": "abc123",
        "line": 42,
        "message": "[P0] 👎 SQL injection vulnerability",
        "unresolved": true
      },
      {
        "id": "def456",
        "line": 88,
        "message": "[P3] 👍 Good defer usage",
        "unresolved": false
      }
    ],
    "src/auth.go": [
      {
        "id": "ghi789",
        "line": 15,
        "message": "[P1] 👎 Missing error handling",
        "unresolved": true
      }
    ]
  }
}
```

### 3. Update Draft Comment

**Update message:**
```bash
gerrit-cli draft update 10661 abc123 "[P1] 👎 Updated: Use prepared statements to prevent SQL injection"
```

**Mark as resolved:**
```bash
gerrit-cli draft update 10661 abc123 --resolved
```

**Mark as unresolved:**
```bash
gerrit-cli draft update 10661 abc123 --unresolved
```

### 4. Delete Draft Comment

**Delete a draft:**
```bash
gerrit-cli draft delete 10661 abc123
```

**Output:**
```json
{
  "success": true,
  "data": {
    "deleted": true,
    "draft_id": "abc123"
  }
}
```

### 5. Publish Drafts (via review post)

**When you're ready to publish all drafts, use the standard review post command:**
```bash
gerrit-cli review post 10661 \
  --message "Found 2 critical issues that must be fixed before merge" \
  --vote -1
```

**Note**: The `gerrit-cli review post` command will automatically publish any draft comments you've created. You don't need a separate "publish drafts" command.

---

## Workflow Examples

### Example 1: Complete Code Review with Draft Comments (Recommended for LLM)

**This is the preferred workflow for AI-powered code reviews:**

```bash
# Step 1: Get change details
gerrit-cli change get 10661

# Step 2: List all changed files
gerrit-cli patchset diff 10661 --list-files

# Step 3: Review each file and create draft comments
# File 1: src/main.go
gerrit-cli patchset diff 10661 --file src/main.go

# Create drafts as you find issues
gerrit-cli draft create 10661 src/main.go 42 "[P0] 👎 SQL injection vulnerability - use parameterized queries"
gerrit-cli draft create 10661 src/main.go 88 "[P3] 👍 Good use of defer for cleanup"

# File 2: src/auth.go
gerrit-cli patchset diff 10661 --file src/auth.go

gerrit-cli draft create 10661 src/auth.go 15 "[P1] 👎 Missing error handling for database connection"
gerrit-cli draft create 10661 src/auth.go 50 "[P2] 👎 Consider using a constant for timeout value"

# Step 4: Review your drafts before publishing
gerrit-cli draft list 10661

# Step 5: Make corrections if needed
# If you want to change a comment:
gerrit-cli draft update 10661 abc123 "[P1] 👎 Updated: Missing error handling and logging"

# If you want to remove a comment:
gerrit-cli draft delete 10661 def456

# Step 6: Publish all drafts with summary and vote
gerrit-cli review post 10661 \
  --message "Found 1 critical security issue and 1 high-priority bug that must be fixed before merge. Also provided 1 suggestion and 1 positive feedback." \
  --vote -1
```

### Example 2: Find and Review a Specific Change (Simple One-Shot)

```bash
# 1. Search for open changes in a project
gerrit-cli change list "status:open project:gobe" --limit 5

# 2. Get details of a specific change
gerrit-cli change get 10661

# 3. List files changed in the patchset
gerrit-cli patchset diff 10661 --list-files

# 4. Get diff for a specific file
gerrit-cli patchset diff 10661 --file src/main.go

# 5. Check existing comments
gerrit-cli comment list 10661

# 6. Post your review
gerrit-cli review post 10661 \
  --message "Code looks good, minor suggestions" \
  --vote 1
```

### Example 2: Check Status of Your Changes

```bash
# Find your open changes
gerrit-cli change list "owner:self status:open"

# Check if there are any new comments
gerrit-cli comment list 12345 --unresolved
```

### Example 3: Compare Multiple Patchsets

```bash
# List files in the latest patchset
gerrit-cli patchset diff 10661 --list-files

# Get diff for each changed file
for file in $(jq -r '.data | keys[]' response.json); do
  gerrit-cli patchset diff 10661 --file "$file"
done
```

---

## Parsing JSON Output

All `gerrit-cli` commands output JSON. You can parse it with `jq`:

**Extract change numbers:**
```bash
gerrit-cli change list "status:open" | \
  jq -r '.data[]._number'
```

**Check if successful:**
```bash
gerrit-cli change get 10661 | \
  jq -r '.success'
```

**Get project name:**
```bash
gerrit-cli change get 10661 | \
  jq -r '.data.project'
```

---

## Error Handling

Always check the `success` field:

```json
{
  "success": false,
  "error": {
    "message": "Change not found",
    "code": "NOT_FOUND"
  }
}
```

**Common error codes:**
- `NOT_FOUND` - Change/file doesn't exist
- `AUTH_ERROR` - Authentication failed
- `CONNECTION_ERROR` - Cannot connect to Gerrit

---

## Tips for LLM Usage

1. **Always quote queries**: Use `"status:open project:gobe"` not `status:open project:gobe`

2. **Parse JSON programmatically**: Use `jq` or parse JSON in your language

3. **Check success before processing**:
   ```bash
   if [ "$(jq -r '.success')" = "true" ]; then
     # Process data
   fi
   ```

4. **Use --limit for exploration**: Don't fetch thousands of results at once

5. **Combine commands**: Use change list → change get → patchset diff workflow

6. **Handle errors gracefully**: Check `success` field and show error message

---

## Quick Reference Card

| Task | Command |
|------|---------|
| List open changes | `gerrit-cli change list "status:open"` |
| Search by project | `gerrit-cli change list "project:NAME"` |
| Get change details | `gerrit-cli change get CHANGE_NUM` |
| List files | `gerrit-cli patchset diff CHANGE_NUM --list-files` |
| Get file diff | `gerrit-cli patchset diff CHANGE_NUM --file PATH` |
| List comments | `gerrit-cli comment list CHANGE_NUM` |
| **Create draft comment** | `gerrit-cli draft create CHANGE_NUM FILE LINE "MESSAGE"` |
| **List drafts** | `gerrit-cli draft list CHANGE_NUM` |
| **Update draft** | `gerrit-cli draft update CHANGE_NUM DRAFT_ID "MESSAGE"` |
| **Delete draft** | `gerrit-cli draft delete CHANGE_NUM DRAFT_ID` |
| Post review | `gerrit-cli review post CHANGE_NUM --message "..." --vote N` |

---

## Environment

- **Tool Path**: `gerrit-cli`
- **Config**: Prefer environment variables; if using `config.yaml`, keep it local and gitignored
- **Output Format**: JSON (default)
- **Authentication**: Loaded from environment variables or local `config.yaml` (never commit credentials)

---

## When to Use `gerrit-cli`

Use `gerrit-cli` when you need to:
- ✅ Query Gerrit for changes
- ✅ Get information about patchsets
- ✅ Read file diffs programmatically
- ✅ Check comments on changes
- ✅ **Build reviews incrementally with draft comments**
- ✅ **Perform LLM-driven code reviews step-by-step**
- ✅ Post reviews with priority-based categorization

The `gerrit-cli` tool is now powerful enough for complete LLM-driven code reviews!

---

## Examples of Good LLM Usage

**Good ✅**:
```
Human: "Find all open changes in the gobe project"
LLM: I'll use gerrit-cli to search for open changes:
     gerrit-cli change list "status:open project:gobe"
```

**Good ✅**:
```
Human: "What files changed in change 10661?"
LLM: Let me list the files:
     gerrit-cli patchset diff 10661 --list-files
```

**Bad ❌**:
```
Human: "Review change 10661"
LLM: gerrit-cli review post 10661 --message "LGTM" --vote 1
     (This should use gerrit-reviewer for proper AI review)
```

---

## Summary

The `gerrit-cli` tool is your complete interface to Gerrit for:
- 🔍 Querying changes
- 📄 Fetching diffs
- 💬 Reading comments
- ✏️ **Creating and managing draft comments**
- 🎯 **Priority-based review (P0-P3)**
- 👍👎 **Good/bad evaluation**
- 📝 Publishing complete reviews

**Best Practice for LLM**: Use draft comments to build reviews incrementally:
1. Get change details
2. List files
3. Review each file and create drafts with priority prefixes
4. Review drafts
5. Publish with summary and vote

Use `gerrit-cli` from PATH, check JSON success field, and quote your queries.
