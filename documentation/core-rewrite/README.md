# Core rewrite — how to run it

| File | Role |
|---|---|
| [PLAN.md](PLAN.md) | The rings, the current ring's state, verification rules. Source of truth. |
| [DECISIONS.md](DECISIONS.md) | Settled decisions. |
| [RUN-LOG.md](RUN-LOG.md) | A short entry per gate. |
| [notes/](notes/) | Handoff notes from earlier runs, by checkpoint. |
| [archive/](archive/) | The previous plan, log, guide and agents (runs 1–4). |
| `.github/agents/ring.agent.md` | Coordinator. Pick this one in the agent picker. |
| `.github/agents/worker.agent.md` | Owns a whole slice, including fix-ups. |
| `.github/agents/scout.agent.md` | Read-only questions, on Haiku. |
| `.github/agents/reviewer.agent.md` | Uses the result and tries to break it, before the demo gate. |

## One-time setup

Add to VS Code's user `settings.json`:

```json
"chat.subagents.showCreditUsage": true,
"github.copilot.chat.executionSubagent.enabled": false,
"github.copilot.chat.agentDebugLog.fileLogging.enabled": true
```

- `showCreditUsage` shows what each subagent cost.
- `executionSubagent.enabled` stops Copilot spawning an agent for small terminal jobs. These were 120 calls in runs 1–3.
- `fileLogging` records real per-request token counts, so usage can be measured instead of estimated.

The agent files list their tools, so MCP servers are not loaded for them. Open each agent file once: VS Code underlines
any tool name it doesn't recognise.

## The prompt

Start a new chat with the **ring** agent, and use this for every ring:

```
Continue the current ring in PLAN.md. Start with the plan gate.
```

To resume after a limit or a full context, use the same prompt in a new chat, or say "continue the approved plan" to
skip the plan gate. `PLAN.md` and `RUN-LOG.md` hold the state.

## What you will be asked

About three times per ring, through `#askQuestions`:

1. **Plan:** what will change, what won't, and the demo at the end.
2. **Design:** only for a real choice or a surprise.
3. **Demo:** steps to try yourself. Your answer is what makes a ring `done`.

Each gate shows turns, tool calls and subagents so far. Stop it if the numbers look wrong for what you're seeing.
