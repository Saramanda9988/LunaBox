---
name: lunabox
description: "Operate the LunaBox visual novel / galgame library via the `lunacli` CLI tool. This skill should be used when the user asks to browse their game library, get game details or recommendations, launch a game, backup saves, or check play status. Trigger on keywords: game, galgame, visual novel, VN, play, launch, start, backup, save, library, LunaBox, lunacli."
---

# LunaBox CLI Skill

LunaBox is a Windows visual novel / galgame library manager. The `lunacli` binary provides CLI access to the full library — listing, querying, launching, and backing up games. This skill enables AI agents to operate LunaBox on behalf of users.

## Prerequisites

- The `lunacli` binary must be in the system PATH or at a known absolute path.
- LunaBox must have games already added via the GUI application.
- Output is UTF-8 and may contain CJK characters (Japanese game titles) and Unicode symbols.

## Commands

### List Games

Show every game in the user's library.

```bash
lunacli list
```

Output is an ASCII table with columns: **Short ID** (8-char prefix), **Status Icon**, **Name**.

Status icons:
- `·` Not Started
- `▶` Playing
- `✓` Completed
- `○` On Hold
- `✗` Dropped

**Parsing:** Each data row follows `│ <8-char-id>  │ <icon> <name> │`. Extract the ID and name from each row.

### Game Details

Show comprehensive metadata for a single game.

```bash
lunacli detail <game>
```

Output is key-value pairs (one per line, split on first `:`): Name, ID, Status, Source, Company, Launch Path, Save Path, Process Name, Use Locale Emulator, Use Magpie, Created At, Cached At, Summary.

### Start a Game

Launch a game from the library.

```bash
lunacli start <game>
lunacli start <game> --le          # With Locale Emulator (Japanese locale)
lunacli start <game> --magpie      # With Magpie (resolution upscaling)
lunacli start <game> --le --magpie # Both
```

Flags:
- `--le` / `-l`: Force Locale Emulator. Requires configured LE path.
- `--magpie` / `-m`: Force Magpie upscaling. Requires configured Magpie path.

On success: `Game started successfully!` followed by `Recording play duration...`.

**Important:** If multiple games match, the command lists candidates and fails. Retry with a more specific ID or full name.

### Backup Saves

Create a local backup of a game's save files.

```bash
lunacli backup --game <game>
lunacli backup -g <game>
```

On success, output includes: `✓ Game save backup created successfully!`, Game name, File name, Size, Path.

### Version

```bash
lunacli version
```

Returns version, git commit, build time, build mode.

### Protocol Management

```bash
lunacli protocol register [--exe <path>]
lunacli protocol unregister
```

Register or unregister the `lunabox://` URL scheme. Typically not needed for agent use.

## Game Resolution

All `<game>` arguments resolve in this order:

1. **Exact ID** — full UUID
2. **ID prefix** — first N characters (recommend 8+)
3. **Exact name** — case-insensitive
4. **Fuzzy name** — substring match, case-insensitive

Multiple matches produce a candidate list and an error. Retry with a more specific query.

## Workflow Patterns

Refer to `references/workflows.md` for detailed workflow examples covering: game recommendation, launching the currently-playing game, pre-route backup, and play-status queries.

## Error Handling

Common errors (non-zero exit code, message on stderr):

| Error message | Cause |
|---|---|
| `no game found matching: <query>` | No match at all |
| `please use a longer ID prefix to match exactly one game` | Ambiguous ID prefix |
| `please use the exact game ID or refine your search` | Ambiguous name match |
| `Locale Emulator path is not configured` | LE not set up in LunaBox settings |
| `Magpie path is not configured` | Magpie not set up in LunaBox settings |

## Safety Notes

- Only `start` and `backup` have side effects. `list`, `detail`, and `version` are read-only.
- Do not run multiple `start` commands simultaneously — one game at a time.
- Always confirm with the user before running `start` (launches a program) or `backup` (writes to disk).
- When recommending games, run `lunacli list` first, then `lunacli detail` on candidates to read summaries before making recommendations.

## System Prompt Snippet

To integrate LunaBox into a bot framework, add the following to the bot's system prompt:

```
You have access to the user's LunaBox game library via the `lunacli` CLI tool.

Available actions:
- `lunacli list` — Show all games (ID, status, name)
- `lunacli detail <game>` — Show game metadata and synopsis
- `lunacli start <game> [--le] [--magpie]` — Launch a game
- `lunacli backup -g <game>` — Backup game saves

Game queries accept: full ID, 8-char ID prefix, or game name (fuzzy match).
Status: · not started, ▶ playing, ✓ completed, ○ on hold, ✗ dropped.

Use these tools to help the user manage their visual novel library: browse games,
get recommendations, check play status, launch games, and backup saves.
Always run `lunacli list` first if you need to know what games the user has.
```
