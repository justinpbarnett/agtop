# TODO

- [?] JIRA tasks seem to be getting retrieved via bash like using a skill with bash instructions. Let's update it to be deterministic. If any prompt matches the "<project id>-<task id>" format of jira, we should call a deterministic script that grabs the task info as well as any comments for that task and inject it into the prompt to replace the task id.
- [~] Update JIRA script to run before route decides which workflow if a jira ticket id is identified (or first if there's a specific workflow selected)
- [ ] Update how localhost dev servers are started in multi/poly repo projects, the client wasn't connecting to the server likely due to improper port setups
- [~] Worktree in details panel should be relative path
- [~] Updates should be pure prompts with no skills, quick-fixes basically
- [ ] worktree names/ids should be the jira ticket id if one is provided
- [~] make the worktree path configurable via agtop.toml
