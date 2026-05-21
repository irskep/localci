# Git Discipline

- Never run state-changing Git commands in parallel.
- Treat `git add`, `git commit`, `git merge`, `git rebase`, `git cherry-pick`, and any command that writes `.git/index` or refs as strictly sequential.
- Do not batch `git add` and `git commit` into parallel tool calls.
- If a Git command fails with an `index.lock` error, stop and verify whether another Git process is active before retrying.
- Avoid stash
