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
- In a new worktree, run `npm --prefix web ci` first. `make build` and `make test` build the frontend and need these dependencies.
- When a ticket depends on another unmerged ticket, branch from that ticket's branch as `al-<number>-<short-name>-on-<base number>`, and retarget the pull request to `main` after the base merges.

## Language

- Pull request titles are `AL-<number> <Chinese title>`. Pull request bodies, ticket titles and bodies, and ticket comments are in Chinese.
- Commit messages, code, and code comments are in English.

## Commits and pull requests

- Commit messages are `AL-<number> <imperative sentence>`, with a first line of at most 72 characters. Stage files by name; do not use `git add -A` or `git add .`.
- Run `go vet ./...` and `make test` before pushing. When every file listed by `git diff --name-only origin/main...HEAD` ends in `.md`, skip both and run the documentation checks instead: relative links and paths exist, and commands match the Makefile, package manifest, and source. When you changed a skill under `.agents/skills/`, also run the skill validation. Both are described in [validation](../../../docs/VALIDATION.md).
- Push a new branch with `git push -u origin <branch>`. A branch created from `origin/main` tracks `origin/main`; `-u` makes it track its own remote branch.
- The pull request title is `AL-<number> <Chinese title>` and its base is `main`. The body states what was delivered, risks, and the verification actually run with exit codes. Do not paste the whole ticket or raw logs.
- Merge with `gh pr merge --merge`, never squash or rebase, and only when the user authorized the merge and checks pass.

## GitHub sync

The instance reads the repository's pull requests every minute. A pull request whose branch name or title contains `AL-<number>` is recorded on that ticket. Opening it moves the ticket to `in_review`; merging it moves the ticket to `done`. A draft moves nothing.

- Do not record pull requests with `add_link`, and do not set `in_review` or `done` by hand.
- About a minute after pushing and opening the pull request, run `get_issue` and check that `pull_requests` lists it and the status is `in_review`. After a merge, check for `done` the same way.
- Only when the sync has not taken effect, record the pull request or set the status yourself, and comment on the ticket with the reason.

## After merge

1. Confirm the branch head is in `origin/main` (`git merge-base --is-ancestor <head> origin/main`) and that nothing else on the branch needs keeping.
2. Remove the worktree with `git worktree remove <path>`. Never force-remove a dirty worktree.
3. Delete the local branch with `git branch -d`. Delete the remote branch only if its tip is the head you checked.
4. Run `git fetch --prune`.
5. When the server changed, update the instance as described in [development](../../../docs/DEVELOPMENT.md).
