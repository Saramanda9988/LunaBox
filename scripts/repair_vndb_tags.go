//go:build ignore

// repair_vndb_tags refreshes LunaBox VNDB tags from a VNDB database dump.
//
// It supports two target shapes:
//  1. A LunaBox DuckDB file (*.db): updates game_tags in a transaction.
//  2. A LunaBox CSV database export directory: rewrites database/game_tags.csv
//     and removes exported temp/orphan table COPY entries from load.sql.
//
// Usage:
//
//	go run scripts/repair_vndb_tags.go --target build/bin/lunabox.db --dump build/bin/vndb-db-2026-06-21.tar.zst --dry-run
//	go run scripts/repair_vndb_tags.go --target build/bin/lunabox_2026-06-25T22-10-40 --dump build/bin/vndb-db-2026-06-21.tar.zst --apply
package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/google/uuid"
)

const (
	vndbTagSource      = "vndb"
	defaultTagLimit    = -1
	defaultMinPositive = 1
)

var (
	vndbIDPattern                  = regexp.MustCompile(`^v[0-9]+$`)
	duckDBExportCreateTablePattern = regexp.MustCompile(`(?i)^CREATE\s+(?:TEMP\s+)?TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?"?([A-Za-z_][A-Za-z0-9_]*)"?\b`)
	duckDBExportCopyPattern        = regexp.MustCompile(`(?i)^COPY\s+"?([A-Za-z_][A-Za-z0-9_]*)"?\s+FROM\s+'`)
)

type targetKind int

const (
	targetKindDuckDB targetKind = iota + 1
	targetKindCSVExport
)

type targetInfo struct {
	kind        targetKind
	inputPath   string
	dbPath      string
	databaseDir string
	gamesCSV    string
	gameTagsCSV string
}

type gameRef struct {
	GameID   string
	Name     string
	SourceID string
}

type existingTag struct {
	ID        string
	GameID    string
	Name      string
	Source    string
	Weight    string
	IsSpoiler string
	CreatedAt string
	UpdatedAt string
}

type tagInfo struct {
	Name       string
	Searchable bool
	Applicable bool
}

type tagAccumulator struct {
	VNID       string
	TagID      string
	Positive   int
	VoteSum    int
	SpoilerSum int
	SpoilerCnt int
}

type computedTag struct {
	VNID      string
	TagID     string
	Name      string
	Rating    float64
	Weight    float64
	IsSpoiler bool
	Positive  int
}

type repairStats struct {
	TargetKind        targetKind
	GamesScanned      int
	VNDBGames         int
	UniqueVNIDs       int
	ExistingTags      int
	KeptTags          int
	RemovedVNDBTags   int
	GeneratedTags     int
	GamesWithTags     int
	GamesWithoutTags  int
	UnknownSourceIDs  int
	DuplicateTagNames int
}

type archiveFormat int

const (
	archiveTarZst archiveFormat = iota + 1
	archiveTarGz
	archiveTarPlain
)

func main() {
	targetPath := flag.String("target", "", "LunaBox DuckDB file or LunaBox CSV export directory")
	dumpPath := flag.String("dump", "", "VNDB near-complete dump, for example vndb-db-2026-06-21.tar.zst")
	apply := flag.Bool("apply", false, "write changes; without this flag the script only prints a dry-run summary")
	dryRun := flag.Bool("dry-run", false, "force dry-run mode")
	tagLimit := flag.Int("tag-limit", defaultTagLimit, "maximum VNDB tags per game; -1 keeps all, 0 writes none")
	minPositive := flag.Int("min-positive-votes", defaultMinPositive, "minimum positive VNDB tag votes required before writing a tag")
	includeMeta := flag.Bool("include-meta-tags", false, "include VNDB tags that are not applicable/searchable")
	backup := flag.Bool("backup", true, "create a timestamped backup before --apply")
	flag.Parse()

	if *targetPath == "" || *dumpPath == "" {
		exitErr(errors.New("--target and --dump are required"))
	}
	if *apply && *dryRun {
		exitErr(errors.New("--apply and --dry-run cannot be used together"))
	}
	if *tagLimit < -1 {
		exitErr(errors.New("--tag-limit must be -1, 0, or a positive integer"))
	}
	if *minPositive < 1 {
		exitErr(errors.New("--min-positive-votes must be at least 1"))
	}

	target, err := resolveTarget(*targetPath)
	if err != nil {
		exitErr(err)
	}

	fmt.Printf("Target: %s\n", target.inputPath)
	fmt.Printf("Mode: %s\n", targetModeName(target.kind))
	fmt.Printf("VNDB dump: %s\n", *dumpPath)
	if !*apply {
		fmt.Println("Dry-run: no files will be changed. Pass --apply to write changes.")
	}

	ctx := context.Background()
	games, existingTags, err := loadTargetData(ctx, target)
	if err != nil {
		exitErr(err)
	}
	sourceIDs := uniqueSourceIDs(games)
	if len(sourceIDs) == 0 {
		fmt.Println("No VNDB-backed games with source_id were found; nothing to repair.")
		return
	}

	fmt.Printf("VNDB-backed games: %d (%d unique VN IDs)\n", len(games), len(sourceIDs))
	fmt.Println("Reading VNDB dump tables: db/tags, db/tags_vn")

	tagsByVN, err := buildVNDBTagsFromDump(*dumpPath, sourceIDs, *tagLimit, *minPositive, *includeMeta)
	if err != nil {
		exitErr(err)
	}

	stats := buildStats(target.kind, games, existingTags, tagsByVN)
	printStats(stats)

	var exportTempTables []string
	if target.kind == targetKindCSVExport {
		exportTempTables, err = findDuckDBExportTablesAbsentFromSchema(target.databaseDir)
		if err != nil {
			exitErr(err)
		}
		if len(exportTempTables) > 0 {
			fmt.Printf("  CSV export temp/orphan tables: %d (%s)\n", len(exportTempTables), strings.Join(exportTempTables, ", "))
			if !*apply {
				fmt.Println("Dry-run: matching load.sql COPY lines and CSV files would be removed.")
			}
		}
	}

	if !*apply {
		return
	}

	if *backup {
		backupPath, err := createBackup(target)
		if err != nil {
			exitErr(err)
		}
		fmt.Printf("Backup created: %s\n", backupPath)
	}

	now := time.Now()
	switch target.kind {
	case targetKindCSVExport:
		removedTables, sanitizeErr := sanitizeDuckDBExportDir(target.databaseDir)
		if sanitizeErr != nil {
			err = fmt.Errorf("sanitize CSV export temp tables: %w", sanitizeErr)
			break
		}
		if len(removedTables) > 0 {
			fmt.Printf("Removed CSV export temp/orphan tables: %s\n", strings.Join(removedTables, ", "))
		}
		err = applyCSVRepair(target, games, existingTags, tagsByVN, now)
	case targetKindDuckDB:
		err = applyDuckDBRepair(ctx, target, games, tagsByVN, now)
	default:
		err = fmt.Errorf("unsupported target kind: %d", target.kind)
	}
	if err != nil {
		exitErr(err)
	}

	fmt.Println("VNDB tag repair completed.")
}

func resolveTarget(rawPath string) (targetInfo, error) {
	abs, err := filepath.Abs(rawPath)
	if err != nil {
		return targetInfo{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return targetInfo{}, fmt.Errorf("target not found: %w", err)
	}
	if !info.IsDir() {
		return targetInfo{kind: targetKindDuckDB, inputPath: abs, dbPath: abs}, nil
	}

	candidates := []string{
		filepath.Join(abs, "database"),
		abs,
	}
	for _, dir := range candidates {
		gamesCSV := filepath.Join(dir, "games.csv")
		gameTagsCSV := filepath.Join(dir, "game_tags.csv")
		if fileExists(gamesCSV) && fileExists(gameTagsCSV) {
			return targetInfo{
				kind:        targetKindCSVExport,
				inputPath:   abs,
				databaseDir: dir,
				gamesCSV:    gamesCSV,
				gameTagsCSV: gameTagsCSV,
			}, nil
		}
	}

	dbCandidates, _ := filepath.Glob(filepath.Join(abs, "*.db"))
	if len(dbCandidates) == 1 {
		return targetInfo{kind: targetKindDuckDB, inputPath: abs, dbPath: dbCandidates[0]}, nil
	}
	return targetInfo{}, fmt.Errorf("target directory is neither a LunaBox CSV export nor a directory with exactly one .db file: %s", abs)
}

func loadTargetData(ctx context.Context, target targetInfo) ([]gameRef, []existingTag, error) {
	switch target.kind {
	case targetKindCSVExport:
		games, err := loadGamesFromCSV(target.gamesCSV)
		if err != nil {
			return nil, nil, err
		}
		tags, err := loadTagsFromCSV(target.gameTagsCSV)
		if err != nil {
			return nil, nil, err
		}
		return games, tags, nil
	case targetKindDuckDB:
		db, err := sql.Open("duckdb", target.dbPath)
		if err != nil {
			return nil, nil, err
		}
		defer db.Close()
		games, err := loadGamesFromDuckDB(ctx, db)
		if err != nil {
			return nil, nil, err
		}
		tags, err := loadTagsFromDuckDB(ctx, db)
		if err != nil {
			return nil, nil, err
		}
		return games, tags, nil
	default:
		return nil, nil, fmt.Errorf("unsupported target kind: %d", target.kind)
	}
}

func loadGamesFromCSV(path string) ([]gameRef, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open games csv: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read games csv header: %w", err)
	}
	index, err := headerIndex(header, "id", "name", "source_type", "source_id")
	if err != nil {
		return nil, err
	}

	var games []gameRef
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read games csv: %w", err)
		}
		sourceType := strings.ToLower(strings.TrimSpace(csvValue(record, index["source_type"])))
		sourceID := normalizeVNID(csvValue(record, index["source_id"]))
		if sourceType != vndbTagSource || sourceID == "" {
			continue
		}
		games = append(games, gameRef{
			GameID:   strings.TrimSpace(csvValue(record, index["id"])),
			Name:     strings.TrimSpace(csvValue(record, index["name"])),
			SourceID: sourceID,
		})
	}
	return games, nil
}

func loadTagsFromCSV(path string) ([]existingTag, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open game_tags csv: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read game_tags csv header: %w", err)
	}
	index, err := headerIndex(header, "id", "game_id", "name", "source", "weight", "is_spoiler", "created_at", "updated_at")
	if err != nil {
		return nil, err
	}

	var tags []existingTag
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read game_tags csv: %w", err)
		}
		tags = append(tags, existingTag{
			ID:        csvValue(record, index["id"]),
			GameID:    csvValue(record, index["game_id"]),
			Name:      csvValue(record, index["name"]),
			Source:    csvValue(record, index["source"]),
			Weight:    csvValue(record, index["weight"]),
			IsSpoiler: csvValue(record, index["is_spoiler"]),
			CreatedAt: csvValue(record, index["created_at"]),
			UpdatedAt: csvValue(record, index["updated_at"]),
		})
	}
	return tags, nil
}

func loadGamesFromDuckDB(ctx context.Context, db *sql.DB) ([]gameRef, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, COALESCE(name, ''), COALESCE(source_id, '')
		FROM games
		WHERE LOWER(COALESCE(source_type, '')) = 'vndb'
			AND COALESCE(source_id, '') <> ''
	`)
	if err != nil {
		return nil, fmt.Errorf("query VNDB games: %w", err)
	}
	defer rows.Close()

	var games []gameRef
	for rows.Next() {
		var game gameRef
		if err := rows.Scan(&game.GameID, &game.Name, &game.SourceID); err != nil {
			return nil, fmt.Errorf("scan VNDB game: %w", err)
		}
		game.SourceID = normalizeVNID(game.SourceID)
		if game.SourceID == "" {
			continue
		}
		games = append(games, game)
	}
	return games, rows.Err()
}

func loadTagsFromDuckDB(ctx context.Context, db *sql.DB) ([]existingTag, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, game_id, name, source, COALESCE(weight, 1.0), COALESCE(is_spoiler, FALSE),
		       COALESCE(CAST(created_at AS VARCHAR), ''), COALESCE(CAST(updated_at AS VARCHAR), '')
		FROM game_tags
	`)
	if err != nil {
		return nil, fmt.Errorf("query existing game_tags: %w", err)
	}
	defer rows.Close()

	var tags []existingTag
	for rows.Next() {
		var tag existingTag
		var weight float64
		var spoiler bool
		if err := rows.Scan(&tag.ID, &tag.GameID, &tag.Name, &tag.Source, &weight, &spoiler, &tag.CreatedAt, &tag.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan game_tag: %w", err)
		}
		tag.Weight = formatFloat(weight)
		tag.IsSpoiler = strconv.FormatBool(spoiler)
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func buildVNDBTagsFromDump(dumpPath string, sourceIDs map[string]struct{}, tagLimit int, minPositive int, includeMeta bool) (map[string][]computedTag, error) {
	tags, err := readTagInfo(dumpPath, includeMeta)
	if err != nil {
		return nil, err
	}
	accs, err := readTagVotes(dumpPath, sourceIDs)
	if err != nil {
		return nil, err
	}

	byVN := make(map[string][]computedTag, len(sourceIDs))
	seenName := make(map[string]map[string]struct{})
	for _, acc := range accs {
		if acc.Positive < minPositive {
			continue
		}
		info, ok := tags[acc.TagID]
		if !ok || strings.TrimSpace(info.Name) == "" {
			continue
		}

		rating := float64(acc.VoteSum) / float64(acc.Positive)
		if rating <= 0 {
			continue
		}
		if rating > 3 {
			rating = 3
		}
		spoiler := 0.0
		if acc.SpoilerCnt > 0 {
			spoiler = float64(acc.SpoilerSum) / float64(acc.SpoilerCnt)
		}

		nameKey := strings.ToLower(info.Name)
		if seenName[acc.VNID] == nil {
			seenName[acc.VNID] = make(map[string]struct{})
		}
		if _, exists := seenName[acc.VNID][nameKey]; exists {
			continue
		}
		seenName[acc.VNID][nameKey] = struct{}{}

		byVN[acc.VNID] = append(byVN[acc.VNID], computedTag{
			VNID:      acc.VNID,
			TagID:     acc.TagID,
			Name:      info.Name,
			Rating:    rating,
			Weight:    rating / 3.0,
			IsSpoiler: spoiler >= 1.5,
			Positive:  acc.Positive,
		})
	}

	for vnid, tags := range byVN {
		sort.SliceStable(tags, func(i, j int) bool {
			if tags[i].Rating == tags[j].Rating {
				if tags[i].Positive == tags[j].Positive {
					return tags[i].Name < tags[j].Name
				}
				return tags[i].Positive > tags[j].Positive
			}
			return tags[i].Rating > tags[j].Rating
		})
		if tagLimit >= 0 && len(tags) > tagLimit {
			tags = tags[:tagLimit]
		}
		byVN[vnid] = tags
	}
	return byVN, nil
}

func readTagInfo(dumpPath string, includeMeta bool) (map[string]tagInfo, error) {
	reader, cleanup, err := openDumpTable(dumpPath, "db/tags")
	if err != nil {
		return nil, err
	}
	defer cleanup()

	result := make(map[string]tagInfo)
	scanner := newCopyScanner(reader)
	for scanner.Scan() {
		fields := scanner.Fields()
		if len(fields) < 8 {
			return nil, fmt.Errorf("db/tags row has %d fields, expected at least 8", len(fields))
		}
		id := fields[0]
		searchable := parseCopyBool(fields[3])
		applicable := parseCopyBool(fields[4])
		if !includeMeta && (!searchable || !applicable) {
			continue
		}
		name := strings.TrimSpace(fields[5])
		if id == "" || name == "" {
			continue
		}
		result[id] = tagInfo{Name: name, Searchable: searchable, Applicable: applicable}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read db/tags: %w", err)
	}
	return result, nil
}

func readTagVotes(dumpPath string, sourceIDs map[string]struct{}) (map[string]tagAccumulator, error) {
	reader, cleanup, err := openDumpTable(dumpPath, "db/tags_vn")
	if err != nil {
		return nil, err
	}
	defer cleanup()

	accs := make(map[string]tagAccumulator)
	scanner := newCopyScanner(reader)
	for scanner.Scan() {
		fields := scanner.Fields()
		if len(fields) < 8 {
			return nil, fmt.Errorf("db/tags_vn row has %d fields, expected at least 8", len(fields))
		}
		tagID := fields[1]
		vnID := normalizeVNID(fields[2])
		if _, wanted := sourceIDs[vnID]; !wanted {
			continue
		}
		if parseCopyBool(fields[6]) {
			continue
		}
		if parseCopyBool(fields[7]) {
			continue
		}

		vote, err := strconv.Atoi(nullToZero(fields[4]))
		if err != nil {
			return nil, fmt.Errorf("parse tag vote %q for %s/%s: %w", fields[4], vnID, tagID, err)
		}
		if vote <= 0 {
			continue
		}

		key := vnID + "\x00" + tagID
		acc := accs[key]
		acc.VNID = vnID
		acc.TagID = tagID
		acc.Positive++
		acc.VoteSum += vote
		if fields[5] != "" && fields[5] != `\N` {
			spoiler, err := strconv.Atoi(fields[5])
			if err != nil {
				return nil, fmt.Errorf("parse spoiler vote %q for %s/%s: %w", fields[5], vnID, tagID, err)
			}
			acc.SpoilerSum += spoiler
			acc.SpoilerCnt++
		}
		accs[key] = acc
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read db/tags_vn: %w", err)
	}
	return accs, nil
}

func applyCSVRepair(target targetInfo, games []gameRef, existing []existingTag, tagsByVN map[string][]computedTag, now time.Time) error {
	gameByID := make(map[string]gameRef, len(games))
	for _, game := range games {
		gameByID[game.GameID] = game
	}

	tempPath := target.gameTagsCSV + ".repair-tmp"
	file, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("create temp game_tags csv: %w", err)
	}
	writer := csv.NewWriter(file)
	if err := writer.Write([]string{"id", "game_id", "name", "source", "weight", "is_spoiler", "created_at", "updated_at"}); err != nil {
		file.Close()
		return fmt.Errorf("write game_tags csv header: %w", err)
	}

	for _, tag := range existing {
		if isTargetVNDBTag(tag, gameByID) {
			continue
		}
		if err := writer.Write([]string{tag.ID, tag.GameID, tag.Name, tag.Source, tag.Weight, tag.IsSpoiler, tag.CreatedAt, tag.UpdatedAt}); err != nil {
			file.Close()
			return fmt.Errorf("write existing tag: %w", err)
		}
	}

	nowText := formatCSVTime(now)
	for _, game := range games {
		for _, tag := range tagsByVN[game.SourceID] {
			row := []string{
				uuid.NewString(),
				game.GameID,
				tag.Name,
				vndbTagSource,
				formatFloat(tag.Weight),
				strconv.FormatBool(tag.IsSpoiler),
				nowText,
				nowText,
			}
			if err := writer.Write(row); err != nil {
				file.Close()
				return fmt.Errorf("write repaired tag for %s: %w", game.GameID, err)
			}
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		file.Close()
		return fmt.Errorf("flush game_tags csv: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temp game_tags csv: %w", err)
	}

	if err := os.Rename(tempPath, target.gameTagsCSV); err != nil {
		return fmt.Errorf("replace game_tags csv: %w", err)
	}
	return nil
}

func findDuckDBExportTablesAbsentFromSchema(exportDir string) ([]string, error) {
	schemaPath := filepath.Join(exportDir, "schema.sql")
	loadPath := filepath.Join(exportDir, "load.sql")
	if _, err := os.Stat(schemaPath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if _, err := os.Stat(loadPath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	formalTables, err := readDuckDBExportSchemaTables(schemaPath)
	if err != nil {
		return nil, err
	}
	if len(formalTables) == 0 {
		return nil, nil
	}

	file, err := os.Open(loadPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	seen := make(map[string]struct{})
	var tables []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		tableName := duckDBExportCopyTableName(scanner.Text())
		if tableName == "" {
			continue
		}
		key := strings.ToLower(tableName)
		if _, ok := formalTables[key]; ok {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		tables = append(tables, tableName)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.Strings(tables)
	return tables, nil
}

func sanitizeDuckDBExportDir(exportDir string) ([]string, error) {
	removedTables, err := findDuckDBExportTablesAbsentFromSchema(exportDir)
	if err != nil || len(removedTables) == 0 {
		return removedTables, err
	}

	removeSet := make(map[string]struct{}, len(removedTables))
	for _, tableName := range removedTables {
		removeSet[strings.ToLower(tableName)] = struct{}{}
	}

	loadPath := filepath.Join(exportDir, "load.sql")
	loadFile, err := os.Open(loadPath)
	if err != nil {
		return nil, err
	}

	tmpPath := loadPath + ".repair-tmp"
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		loadFile.Close()
		return nil, err
	}

	scanner := bufio.NewScanner(loadFile)
	for scanner.Scan() {
		line := scanner.Text()
		tableName := duckDBExportCopyTableName(line)
		if tableName != "" {
			if _, remove := removeSet[strings.ToLower(tableName)]; remove {
				continue
			}
		}
		if _, err := fmt.Fprintln(tmpFile, line); err != nil {
			tmpFile.Close()
			loadFile.Close()
			os.Remove(tmpPath)
			return nil, err
		}
	}
	if err := scanner.Err(); err != nil {
		tmpFile.Close()
		loadFile.Close()
		os.Remove(tmpPath)
		return nil, err
	}
	if err := loadFile.Close(); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return nil, err
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return nil, err
	}
	if err := os.Rename(tmpPath, loadPath); err != nil {
		os.Remove(tmpPath)
		return nil, err
	}

	for _, tableName := range removedTables {
		if err := os.Remove(filepath.Join(exportDir, tableName+".csv")); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	return removedTables, nil
}

func readDuckDBExportSchemaTables(schemaPath string) (map[string]struct{}, error) {
	file, err := os.Open(schemaPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	tables := make(map[string]struct{})
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if tableName := duckDBExportCreateTableName(scanner.Text()); tableName != "" {
			tables[strings.ToLower(tableName)] = struct{}{}
		}
	}
	return tables, scanner.Err()
}

func duckDBExportCreateTableName(line string) string {
	matches := duckDBExportCreateTablePattern.FindStringSubmatch(strings.TrimSpace(line))
	if len(matches) != 2 {
		return ""
	}
	return matches[1]
}

func duckDBExportCopyTableName(line string) string {
	matches := duckDBExportCopyPattern.FindStringSubmatch(strings.TrimSpace(line))
	if len(matches) != 2 {
		return ""
	}
	return matches[1]
}

func applyDuckDBRepair(ctx context.Context, target targetInfo, games []gameRef, tagsByVN map[string][]computedTag, now time.Time) error {
	db, err := sql.Open("duckdb", target.dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM game_tags
		WHERE source = 'vndb'
		  AND game_id IN (
			SELECT id FROM games
			WHERE LOWER(COALESCE(source_type, '')) = 'vndb'
			  AND COALESCE(source_id, '') <> ''
		  )
	`); err != nil {
		return fmt.Errorf("delete old VNDB tags: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO game_tags (id, game_id, name, source, weight, is_spoiler, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (game_id, name, source) DO UPDATE SET
			id = EXCLUDED.id,
			weight = EXCLUDED.weight,
			is_spoiler = EXCLUDED.is_spoiler,
			updated_at = EXCLUDED.updated_at
	`)
	if err != nil {
		return fmt.Errorf("prepare repaired tag insert: %w", err)
	}
	defer stmt.Close()

	for _, game := range games {
		for _, tag := range tagsByVN[game.SourceID] {
			if _, err := stmt.ExecContext(ctx, uuid.NewString(), game.GameID, tag.Name, vndbTagSource, tag.Weight, tag.IsSpoiler, now, now); err != nil {
				return fmt.Errorf("insert tag %s for %s: %w", tag.Name, game.GameID, err)
			}
		}
	}

	return tx.Commit()
}

func buildStats(kind targetKind, games []gameRef, existing []existingTag, tagsByVN map[string][]computedTag) repairStats {
	gameByID := make(map[string]gameRef, len(games))
	sourceIDs := make(map[string]struct{})
	for _, game := range games {
		gameByID[game.GameID] = game
		sourceIDs[game.SourceID] = struct{}{}
	}

	stats := repairStats{
		TargetKind:   kind,
		GamesScanned: len(games),
		VNDBGames:    len(games),
		UniqueVNIDs:  len(sourceIDs),
		ExistingTags: len(existing),
	}
	for _, tag := range existing {
		if isTargetVNDBTag(tag, gameByID) {
			stats.RemovedVNDBTags++
			continue
		}
		stats.KeptTags++
	}
	for _, game := range games {
		tags := tagsByVN[game.SourceID]
		if len(tags) == 0 {
			stats.GamesWithoutTags++
			if _, known := tagsByVN[game.SourceID]; !known {
				stats.UnknownSourceIDs++
			}
			continue
		}
		stats.GamesWithTags++
		stats.GeneratedTags += len(tags)
	}
	return stats
}

func printStats(stats repairStats) {
	fmt.Println()
	fmt.Println("Summary")
	fmt.Printf("  VNDB games scanned:        %d\n", stats.VNDBGames)
	fmt.Printf("  Unique VNDB source IDs:    %d\n", stats.UniqueVNIDs)
	fmt.Printf("  Existing game_tags rows:   %d\n", stats.ExistingTags)
	fmt.Printf("  Existing VNDB rows removed:%d\n", stats.RemovedVNDBTags)
	fmt.Printf("  Existing rows kept:        %d\n", stats.KeptTags)
	fmt.Printf("  New VNDB rows generated:   %d\n", stats.GeneratedTags)
	fmt.Printf("  Games with VNDB tags:      %d\n", stats.GamesWithTags)
	fmt.Printf("  Games without VNDB tags:   %d\n", stats.GamesWithoutTags)
}

func createBackup(target targetInfo) (string, error) {
	stamp := time.Now().Format("20060102-150405")
	switch target.kind {
	case targetKindCSVExport:
		backupPath := target.databaseDir + ".before-vndb-tag-repair-" + stamp
		if err := copyDir(target.databaseDir, backupPath); err != nil {
			return "", err
		}
		return backupPath, nil
	case targetKindDuckDB:
		backupPath := target.dbPath + ".before-vndb-tag-repair-" + stamp
		if err := copyFile(target.dbPath, backupPath); err != nil {
			return "", err
		}
		return backupPath, nil
	default:
		return "", fmt.Errorf("unsupported target kind: %d", target.kind)
	}
}

func openDumpTable(dumpPath string, tableName string) (io.Reader, func(), error) {
	format, err := detectArchiveFormat(dumpPath)
	if err != nil {
		return nil, nil, err
	}

	var source io.Reader
	var cleanup func()
	file, err := os.Open(dumpPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open dump: %w", err)
	}
	cleanup = func() { file.Close() }

	switch format {
	case archiveTarPlain:
		source = file
	case archiveTarGz:
		gz, err := gzip.NewReader(file)
		if err != nil {
			file.Close()
			return nil, nil, fmt.Errorf("open gzip dump: %w", err)
		}
		source = gz
		cleanup = func() {
			gz.Close()
			file.Close()
		}
	case archiveTarZst:
		cmd := exec.Command("zstd", "-dc", dumpPath)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			file.Close()
			return nil, nil, fmt.Errorf("open zstd stdout: %w", err)
		}
		if err := cmd.Start(); err != nil {
			file.Close()
			return nil, nil, fmt.Errorf("start zstd -dc: %w", err)
		}
		file.Close()
		source = stdout
		cleanup = func() {
			stdout.Close()
			if err := cmd.Wait(); err != nil {
				_ = err
			}
		}
	default:
		file.Close()
		return nil, nil, fmt.Errorf("unsupported archive format")
	}

	tarReader := tar.NewReader(source)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			cleanup()
			return nil, nil, fmt.Errorf("table %s not found in dump", tableName)
		}
		if err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("read tar header: %w", err)
		}
		if filepath.ToSlash(header.Name) == tableName {
			return tarReader, cleanup, nil
		}
	}
}

func detectArchiveFormat(path string) (archiveFormat, error) {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".tar.zst") || strings.HasSuffix(lower, ".tzst"):
		if _, err := exec.LookPath("zstd"); err != nil {
			return 0, errors.New("zstd executable is required to read .tar.zst dumps")
		}
		return archiveTarZst, nil
	case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
		return archiveTarGz, nil
	case strings.HasSuffix(lower, ".tar"):
		return archiveTarPlain, nil
	default:
		return 0, fmt.Errorf("unsupported dump format: %s", path)
	}
}

type copyScanner struct {
	reader *csv.Reader
	fields []string
	err    error
}

func newCopyScanner(r io.Reader) *copyScanner {
	reader := csv.NewReader(r)
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	return &copyScanner{reader: reader}
}

func (s *copyScanner) Scan() bool {
	if s.err != nil {
		return false
	}
	record, err := s.reader.Read()
	if errors.Is(err, io.EOF) {
		return false
	}
	if err != nil {
		s.err = err
		return false
	}
	for i := range record {
		record[i] = unescapeCopyValue(record[i])
	}
	s.fields = record
	return true
}

func (s *copyScanner) Fields() []string {
	return s.fields
}

func (s *copyScanner) Err() error {
	return s.err
}

func unescapeCopyValue(value string) string {
	if value == `\N` {
		return value
	}
	var b strings.Builder
	b.Grow(len(value))
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if ch != '\\' || i+1 >= len(value) {
			b.WriteByte(ch)
			continue
		}
		i++
		switch value[i] {
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'v':
			b.WriteByte('\v')
		case '\\':
			b.WriteByte('\\')
		default:
			b.WriteByte(value[i])
		}
	}
	return b.String()
}

func uniqueSourceIDs(games []gameRef) map[string]struct{} {
	ids := make(map[string]struct{}, len(games))
	for _, game := range games {
		if game.SourceID != "" {
			ids[game.SourceID] = struct{}{}
		}
	}
	return ids
}

func normalizeVNID(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return ""
	}
	value = strings.TrimPrefix(value, "https://vndb.org/")
	value = strings.TrimPrefix(value, "http://vndb.org/")
	value = strings.TrimPrefix(value, "vndb.org/")
	value = strings.Trim(value, "/")
	if strings.HasPrefix(value, "v") {
		parts := strings.Split(value, "/")
		value = parts[0]
	}
	if !vndbIDPattern.MatchString(value) {
		return ""
	}
	return value
}

func isTargetVNDBTag(tag existingTag, gameByID map[string]gameRef) bool {
	if strings.ToLower(strings.TrimSpace(tag.Source)) != vndbTagSource {
		return false
	}
	_, ok := gameByID[tag.GameID]
	return ok
}

func headerIndex(header []string, required ...string) (map[string]int, error) {
	index := make(map[string]int, len(header))
	for i, name := range header {
		index[strings.TrimSpace(name)] = i
	}
	for _, name := range required {
		if _, ok := index[name]; !ok {
			return nil, fmt.Errorf("missing required CSV column %q", name)
		}
	}
	return index, nil
}

func csvValue(record []string, index int) string {
	if index < 0 || index >= len(record) {
		return ""
	}
	return record[index]
}

func parseCopyBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "t", "true", "1":
		return true
	default:
		return false
	}
}

func nullToZero(value string) string {
	if value == "" || value == `\N` {
		return "0"
	}
	return value
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func copyFile(src string, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open backup source: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create backup: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy backup: %w", err)
	}
	return out.Close()
}

func copyDir(src string, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("read backup source dir: %w", err)
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat backup source: %w", err)
		}
		if info.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}

func formatFloat(value float64) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "0"
	}
	return strconv.FormatFloat(value, 'f', 6, 64)
}

func formatCSVTime(t time.Time) string {
	_, offset := t.Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	hours := offset / 3600
	minutes := (offset % 3600) / 60
	return fmt.Sprintf("%s%s%02d:%02d", t.Format("2006-01-02 15:04:05.000000"), sign, hours, minutes)
}

func targetModeName(kind targetKind) string {
	switch kind {
	case targetKindCSVExport:
		return "LunaBox CSV export"
	case targetKindDuckDB:
		return "DuckDB database"
	default:
		return "unknown"
	}
}

func exitErr(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
