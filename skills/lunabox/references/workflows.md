# LunaBox Workflow Examples

Detailed workflow patterns for common AI agent scenarios with LunaBox.

## Recommend a Game

When the user asks for a game recommendation:

1. Run `lunacli list` to get the full library.
2. Filter games with status `·` (not started) — these are unplayed candidates.
3. For promising candidates, run `lunacli detail <id>` to read the summary and metadata.
4. Recommend based on user preferences (genre, company, engine) and game summaries.
5. If the user accepts, offer to launch with `lunacli start <id>`.

## Launch the Currently-Playing Game

When the user says "continue my game" or "launch what I was playing":

1. Run `lunacli list` to get the library.
2. Find the game with status `▶` (playing).
3. If exactly one match, run `lunacli start <id>`.
4. If multiple games are marked as playing, present the list and ask the user to choose.

## Backup Before Starting a New Route

When the user wants to save progress before branching:

1. Run `lunacli backup -g <game>` to save current progress.
2. Confirm backup success (check for `✓` in output).
3. Report the backup file name and path to the user.
4. Optionally run `lunacli start <game>` to relaunch.

## Check Play Status

When the user asks "have I played X?" or "what's my status on X?":

1. Run `lunacli detail <game-name>`.
2. Check the Status field in the output.
3. Report the status in natural language:
   - `Not Started` → "You haven't played this yet."
   - `Playing` → "You're currently playing this."
   - `Completed` → "You've finished this one."
   - `On Hold` → "You put this on hold."
   - `Dropped` → "You dropped this one."

## Browse by Company or Engine

When the user asks about games from a specific developer:

1. Run `lunacli list` to get all game IDs.
2. Run `lunacli detail <id>` for each game (or a batch of candidates).
3. Filter by the Company or Source field.
4. Present the filtered results to the user.

## Output Parsing Reference

| Command | Parse strategy |
|---|---|
| `list` | Split table rows by `│`, extract column 2 (ID) and column 3 (icon + name) |
| `detail` | Split lines by first `:`, treat as key-value pairs |
| `start` | Check for `Game started successfully!` in output |
| `backup` | Check for `✓` prefix, parse `File:`, `Size:`, `Path:` lines |
