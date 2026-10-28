# Core rewrite — how to run it with Copilot

Files:
- [PLAN.md](PLAN.md): phases, checkpoints, status table, open decisions. The source of truth.
- [DECISIONS.md](DECISIONS.md): settled decisions.
- [notes/](notes/): one handoff note per finished checkpoint.
- `.github/agents/core-rewrite.agent.md`: orchestrator (Sonnet). Pick it in the Copilot agent picker.
- `.github/agents/scout.agent.md`: read-only fact-finder (Haiku), used as a subagent.
- `.github/agents/reviewer.agent.md`: fresh-context checkpoint reviewer (Sonnet), used as a subagent.
- `.github/instructions/core-go.instructions.md`: Go rules, applied automatically to `core/**`.

Check the `model:` names in the three agent files against your Copilot model picker. If custom agents cannot be
used as subagents in your VS Code version, the orchestrator should run a plain subagent and paste the agent
file's instructions into its prompt.

## Starting a session

New chat, `core-rewrite` agent selected, then one of:

**Next checkpoint**
> Continue the core rewrite. Pick the next checkpoint per PLAN.md, do it end to end, and stop when its
> definition of done is met.

**A specific checkpoint (for parallel sessions)**
> Core rewrite: do checkpoint C2.2 only. Work on its own branch. Do not touch files outside what C2.2 lists.

**Resume after a context reset**
> Resume core-rewrite checkpoint C3.1 from its branch and the latest handoff note. Re-read PLAN.md first.

**Answer a decision**
> Decision D-open-1: use port 7700. Record it in DECISIONS.md, update PLAN.md, and continue.

## Parallel sessions

Run at most 2–3 at once, each on its own checkpoint from the same wave in PLAN.md's "Parallel work", each in its
own branch (a separate worktree avoids checkout clashes). Never run a `J` (join) checkpoint in parallel with
anything else. Merge finished checkpoint PRs before starting checkpoints that depend on them.
