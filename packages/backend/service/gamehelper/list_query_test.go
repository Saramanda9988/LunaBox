package gamehelper

import (
	"context"
	"database/sql"
	"reflect"
	"testing"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/migrations"

	_ "lunabox/backend/utils/dbutils/duckdbdriver"
)

func setupGameListQueryTest(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err := migrations.InitSchema(db); err != nil {
		t.Fatalf("initialize test schema: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO games (
			id, name, source_type, source_id, cached_at, created_at, updated_at
		) VALUES
			('legacy-bangumi', 'Legacy Bangumi', 'Bangumi', '101', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('local', 'Local', 'local', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('empty-source', 'Empty Source', '', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('mixed', 'Mixed Legacy', 'mixed', 'legacy', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('multi', 'Multi Source', 'bangumi', '202', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('steam', 'Steam', 'steam', '303', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
		INSERT INTO game_metadata_sources (game_id, source_type, source_id)
		VALUES
			('multi', 'bangumi', '202'),
			('multi', 'vndb', 'v202'),
			('steam', 'steam', '303');
	`); err != nil {
		t.Fatalf("insert game list fixtures: %v", err)
	}
	return db
}

func TestQueryGameListFiltersMetadataSources(t *testing.T) {
	db := setupGameListQueryTest(t)
	tests := []struct {
		name   string
		source enums.SourceType
		want   []string
	}{
		{name: "local", source: enums.Local, want: []string{"local"}},
		{name: "legacy remote", source: enums.Bangumi, want: []string{"legacy-bangumi", "multi"}},
		{name: "secondary remote", source: enums.VNDB, want: []string{"multi"}},
		{name: "single remote", source: enums.Steam, want: []string{"steam"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := test.source
			response, err := QueryGameList(context.Background(), db, vo.GameListRequest{
				Limit:          100,
				MetadataSource: &source,
				SortBy:         enums.GameListSortByName,
				SortOrder:      enums.SortOrderAsc,
			}, GameListScope{})
			if err != nil {
				t.Fatalf("query games: %v", err)
			}
			got := make([]string, 0, len(response.Games))
			for _, game := range response.Games {
				got = append(got, game.ID)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("unexpected game IDs: got %v want %v", got, test.want)
			}
			if response.Total != len(test.want) {
				t.Fatalf("unexpected total: got %d want %d", response.Total, len(test.want))
			}
		})
	}
}

func TestQueryGameListExcludesMetadataSource(t *testing.T) {
	db := setupGameListQueryTest(t)
	source := enums.Bangumi
	response, err := QueryGameList(context.Background(), db, vo.GameListRequest{
		Limit:                 100,
		MetadataSource:        &source,
		ExcludeMetadataSource: true,
		SortBy:                enums.GameListSortByName,
		SortOrder:             enums.SortOrderAsc,
	}, GameListScope{})
	if err != nil {
		t.Fatalf("query games: %v", err)
	}
	got := make([]string, 0, len(response.Games))
	for _, game := range response.Games {
		got = append(got, game.ID)
	}
	want := []string{"empty-source", "local", "mixed", "steam"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected excluded game IDs: got %v want %v", got, want)
	}
}

func TestQueryGameListUsesSecondarySort(t *testing.T) {
	db := setupGameListQueryTest(t)
	if _, err := db.Exec(`
		INSERT INTO games (
			id, name, company, rating, source_type, source_id,
			cached_at, created_at, updated_at
		) VALUES
			('alpha-low', 'Secondary Alpha Low', 'Alpha', 3, 'local', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('alpha-high', 'Secondary Alpha High', 'Alpha', 9, 'local', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('beta-low', 'Secondary Beta Low', 'Beta', 2, 'local', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('beta-high', 'Secondary Beta High', 'Beta', 8, 'local', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`); err != nil {
		t.Fatalf("insert secondary sorting fixtures: %v", err)
	}

	response, err := QueryGameList(context.Background(), db, vo.GameListRequest{
		Limit:              100,
		SearchQuery:        "Secondary",
		SortBy:             enums.GameListSortByCompany,
		SortOrder:          enums.SortOrderAsc,
		SecondarySortBy:    enums.GameListSortByRating,
		SecondarySortOrder: enums.SortOrderDesc,
	}, GameListScope{})
	if err != nil {
		t.Fatalf("query games with secondary sorting: %v", err)
	}

	got := make([]string, 0, len(response.Games))
	for _, game := range response.Games {
		got = append(got, game.ID)
	}
	want := []string{"alpha-high", "alpha-low", "beta-high", "beta-low"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected secondary sorting: got %v want %v", got, want)
	}
}

func TestNormalizeGameListRequestClearsDuplicateSecondarySort(t *testing.T) {
	req := normalizeGameListRequest(vo.GameListRequest{
		SortBy:             enums.GameListSortByRating,
		SortOrder:          enums.SortOrderDesc,
		SecondarySortBy:    enums.GameListSortByRating,
		SecondarySortOrder: enums.SortOrderAsc,
	})
	if req.SecondarySortBy != "" || req.SecondarySortOrder != "" {
		t.Fatalf("expected duplicate secondary sorting to be cleared: %#v", req)
	}
}

func TestQueryGameListFiltersIDs(t *testing.T) {
	db := setupGameListQueryTest(t)
	resp, err := QueryGameList(context.Background(), db, vo.GameListRequest{
		IDs:       []string{"local", "steam", "local", "not-exist"},
		SortBy:    enums.GameListSortByName,
		SortOrder: enums.SortOrderAsc,
	}, GameListScope{})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, g := range resp.Games {
		got = append(got, g.ID)
	}
	if want := []string{"local", "steam"}; resp.Total != 2 || !reflect.DeepEqual(got, want) {
		t.Fatalf("total %d, ids %v, want %v", resp.Total, got, want)
	}
}
