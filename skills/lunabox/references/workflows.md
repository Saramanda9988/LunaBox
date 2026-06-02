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

## Install a Game from URL

When the user provides a download link and wants to install a game:

1. Confirm the URL and game title with the user before proceeding.
2. Run `lunacli install <url> -t "<game-title>"`.
3. If the user knows the archive format, add `-F <format>` (e.g., `-F rar`, `-F 7z`).
4. If the user knows the main executable, add `-s <relative-path>` (e.g., `-s Game.exe`).
5. If the user has metadata info, add `-m <source> -i <id>` (e.g., `-m bangumi -i 12345`).
6. Wait for the command to complete (may take a long time for large files).
7. On success, report the installed game name and path.
8. Optionally offer to launch with `lunacli start <game>`.

**Example — User says "帮我下载安装这个游戏 https://example.com/game.zip 叫星空的记忆"：**

```bash
lunacli install "https://example.com/game.zip" -t "星空的记忆"
```

**Example — User provides a .rar link with known Bangumi ID：**

```bash
lunacli install "https://example.com/game.rar" -t "星空的记忆" -F rar -m bangumi -i 12345
```

**Example — User provides a link and knows the startup exe：**

```bash
lunacli install "https://example.com/game.7z" -t "星空的记忆" -F 7z -s "StarMemories/Game.exe"
```

## Output Parsing Reference

| Command | Parse strategy |
|---|---|
| `list` | Split table rows by `│`, extract column 2 (ID) and column 3 (icon + name) |
| `detail` | Split lines by first `:`, treat as key-value pairs |
| `start` | Check for `Game started successfully!` in output |
| `backup` | Check for `✓` prefix, parse `File:`, `Size:`, `Path:` lines |
| `install` | Check for `✓ Game installed successfully!`, parse `Game:`, `ID:`, `Path:` lines |
