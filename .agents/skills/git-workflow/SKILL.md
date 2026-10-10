---
name: git-workflow
description: "Branches, worktrees, commits, pull requests, merging, and cleanup for Agent Ledger work."
---

# Git workflow

## Before changing anything

- Check the current branch, worktrees, and uncommitted or unpushed changes. Fetch, then compare `main` with `origin/main`.
- Leave other work as it is: never stash, overwrite, or mix in changes you did not make. When a checkout is dirty or diverged and you cannot tell whose work it is, stop and report.

## Branches and worktrees

- One ticket, one branch, created from the latest `origin/main`: `al-<number>-<short-name>`, lowercase and hyphenated. Work without a ticket uses a short descriptive name.
- Work in a dedicated worktree beside the main checkout, such as `../agent-ledger-al-<number>`. Keep the main checkout on `main`; the instance is built from it.
- When a ticket depends on another unmerged ticket, branch from that ticket's branch as `al-<number>-<short-name>-on-<base number>`, and retarget the pull request to `main` after the base merges.

## Commits and pull requests

- Commit messages are `AL-<number> <imperative sentence>`, with a first line of at most 72 characters. Stage files by name; do not use `git add -A` or `git add .`.
- Run `go vet ./...` and `make test` before pushing.
- The pull request title is `AL-<number> <title>` and its base is `main`. The body states what was delivered, risks, and the verification actually run with exit codes. Do not paste the whole ticket or raw logs.
- After opening it, record it on the ticket with `add_link` and set the ticket to `in_review`.
- Merge with `gh pr merge --merge`, never squash or rebase, and only when the user authorized the merge and checks pass. Then set the ticket to `done`.

## After merge

1. Confirm the branch head is in `origin/main` (`git merge-base --is-ancestor <head> origin/main`) and that nothing else on the branch needs keeping.
2. Remove the worktree with `git worktree remove <path>`. Never force-remove a dirty worktree.
3. Delete the local branch with `git branch -d`. Delete the remote branch only if its tip is the head you checked.
4. Run `git fetch --prune`.
5. When the server changed, update the instance as described in `AGENTS.md`.
