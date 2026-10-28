# Core rewrite — Copilot guide

## Files

| File | Role |
|---|---|
| [PLAN.md](PLAN.md) | Phases, checkpoints, status table, waves, open decisions. Source of truth. |
| [DECISIONS.md](DECISIONS.md) | Settled decisions. |
| [notes/](notes/) | One handoff note per finished checkpoint. Memory between sessions. |
| `.github/agents/director.agent.md` | Unattended director (Sonnet 4.6): runs every wave via nested subagents. |
| `.github/agents/core-rewrite.agent.md` | Orchestrator (Sonnet 4.6): wave or checkpoint mode; also what the director's subagents follow. |
| `.github/agents/scout.agent.md` | Read-only fact-finder (Haiku), used as a subagent. |
| `.github/agents/reviewer.agent.md` | Fresh-context reviewer (Sonnet 4.6), used as a subagent. |
| `.github/instructions/core-go.instructions.md` | Go rules, applied automatically to `core/**`. |

## One-time setup

1. Check out `core/c0-scaffold` (or `main` once C0 is merged) and open the repo in VS Code.
2. Check the `model:` lines in the three agent files match names in your Copilot model picker
   (`Claude Sonnet 4.6`, `Claude Haiku 4.5`). Fix them if your picker spells them differently.
3. Make sure subagents are enabled (the `runSubagent` tool, and custom agents as subagents if your version has
   that setting). If custom agents cannot be subagents, the orchestrator pastes the agent file's instructions
   into a plain subagent's prompt; that works too.
4. Go 1.27, Node 22 and `gh` on PATH.

## How work flows

- **Wave mode** (default way to make progress): one Copilot chat runs a whole wave. The orchestrator asks you all
  of the wave's decisions up front, delegates each checkpoint to a parallel subagent, has each one reviewed,
  commits one commit per checkpoint on `core/wave-<n>`, and opens one PR for the wave.
- **Checkpoint mode**: one checkpoint per chat. Used for join (`J`) checkpoints, which never run inside a wave,
  and for redoing a single checkpoint.
- A new chat for every wave or join checkpoint. The plan, decisions and notes carry over; chat history does not.
- You merge each PR. The next wave branches from the merged result.

## Prompts

### Run everything unattended (director)

Select the **director** agent (needs nested subagents), then:

```
Run the full core rewrite as director, unattended.

Settings:
- HEAVY_CHECKS = no
- STOP_AFTER = all

Follow .github/agents/director.agent.md exactly. Start from core/c0-scaffold (or resume from
documentation/core-rewrite/RUN-LOG.md if it exists). Answer open decisions with PLAN.md's recommendations as
provisional, run every wave and join checkpoint through C8.2 (never C9.x), integrate on core/rewrite with a draft PR
to main, and keep the run log current after every wave. Do not ask me anything until the final report.
```

To resume after the chat ends or runs out of context: same prompt in a new chat; the director reads `RUN-LOG.md`
and continues. Afterwards, the draft PR lists the provisional decisions to confirm, the checks that need you,
and the milestones that are ready to try.

### Start a wave (first one: wave 1)

```
Run the core rewrite in wave mode, wave 1.

Read documentation/core-rewrite/PLAN.md, DECISIONS.md and the latest notes first. Wave 1 is:
C1.1, C2.1, C2.2, C2.3, C2.4, P1, P2, C8.0. Branch core/wave-1 from core/c0-scaffold.

Order:
1. List every [DECISION] these checkpoints need, with your recommendation, and wait for my answers.
2. Do C1.1 yourself (git subtree add of https://github.com/rsxdalv/tts-webui-ignition into launcher/, plus
   its workflow and README fixes), commit it.
3. Delegate C2.1, C2.2, C2.3, C2.4, P1, P2 and C8.0 to parallel subagents, one each, with the brief format
   from your agent file. Use the scout subagent (Haiku) for any inventory a checkpoint needs before coding.
   C2.x touch only their own core/internal/<pkg>/ dir, cmd/tts-webui/cmd_<name>.go and a README section.
   P1 touches only installer_scripts/init_mamba.*. P2 touches only a new sdk/python/ dir plus re-exports in
   tts_webui/. C8.0 writes only documentation/core-rewrite/notes/C8.0.md.
4. Review each with the reviewer subagent, get findings fixed, commit each checkpoint separately.
5. End of wave: full test + cross-builds, PLAN.md statuses, one note per checkpoint, push, open the PR.

Do not stop between checkpoints to ask me anything except decisions and the stop conditions in your agent file.
```

### Later waves

```
Run the core rewrite in wave mode, wave <n>. Read PLAN.md, DECISIONS.md and the notes from wave <n-1> first.
Branch core/wave-<n> from main. Take the wave's checkpoints from PLAN.md "Parallel work", skipping any join (J)
checkpoint and anything whose dependencies are not done. Same procedure as your agent file's wave mode.
```

### A join checkpoint (C3.1, C3.2, C5.5, C7.3, C9.x)

```
Core rewrite, checkpoint mode: do <id> only, on branch core/<id>-<slug> from main. Read PLAN.md, DECISIONS.md
and every note this checkpoint depends on. Use scout subagents for inventories of legacy code and a reviewer
subagent at the end. Stop when its definition of done and parity check are met; open the PR.
```

### Resume after the chat ran out of context

```
Resume the core rewrite on branch <branch>. Read PLAN.md (in-progress rows), DECISIONS.md, the newest notes and
`git log --oneline main..HEAD`. Continue from where the last commit left off, same mode as before.
```

### Answer decisions

```
Decisions: D-open-1 = 7700. D-open-4 = merge into ui/ at C6.2. Record them in DECISIONS.md, update PLAN.md, continue.
```

## Where Haiku fits

Only through the `scout` agent: read-only inventories, call-site lists, step-by-step summaries of big legacy files,
fixture tables. Never Go code, never decisions, never reviews. PLAN.md's "Model use" lists the good fits per checkpoint.

## What you do between waves

- Answer the decision list at the start of each wave.
- Review and merge the wave PR (CI must be green).
- At milestones (M1–M4), try the new path yourself on a scratch install root before flipping any default.
