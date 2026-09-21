package idmapper

import (
	"runtime"
	"strconv"
	"testing"

	"lunabox/internal/common/enums"
)

func TestMapperResolvesHikarinagiID(t *testing.T) {
	mapper := New([]IDs{{VNDBID: 10, BangumiID: 20, SteamID: 30, HikarinagiID: 40}})

	mapping, found := mapper.Resolve(enums.Hikarinagi, "40")
	if !found || mapping.VNDBID != 10 || mapping.HikarinagiID != 40 {
		t.Fatalf("Resolve(Hikarinagi, 40) = %+v, %t", mapping, found)
	}
}

func TestEmbeddedMappingsAreReciprocal(t *testing.T) {
	mapper, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}
	if len(mapper.byVNDB) == 0 {
		t.Fatal("embedded mapper contains no VNDB records")
	}

	for vndbID, index := range mapper.byVNDB {
		record := mapper.records[index]
		if record.VNDBID != vndbID {
			t.Fatalf("VNDB key %d points to record v%d", vndbID, record.VNDBID)
		}
		if record.BangumiID > 0 {
			if reverse, ok := mapper.byBangumi[record.BangumiID]; !ok || mapper.records[reverse].VNDBID != vndbID {
				t.Fatalf("Bangumi %d is not reciprocal with VNDB v%d", record.BangumiID, vndbID)
			}
		}
		if record.SteamID > 0 {
			if reverse, ok := mapper.bySteam[record.SteamID]; !ok || mapper.records[reverse].VNDBID != vndbID {
				t.Fatalf("Steam %d is not reciprocal with VNDB v%d", record.SteamID, vndbID)
			}
		}
		if record.HikarinagiID > 0 {
			if reverse, ok := mapper.byHikarinagi[record.HikarinagiID]; !ok || mapper.records[reverse].VNDBID != vndbID {
				t.Fatalf("Hikarinagi %d is not reciprocal with VNDB v%d", record.HikarinagiID, vndbID)
			}
		}
	}

	for bangumiID, index := range mapper.byBangumi {
		record := mapper.records[index]
		if record.BangumiID != bangumiID || mapper.records[mapper.byVNDB[record.VNDBID]] != record {
			t.Fatalf("Bangumi %d has a non-reciprocal mapping", bangumiID)
		}
	}
	for steamID, index := range mapper.bySteam {
		record := mapper.records[index]
		if record.SteamID != steamID || mapper.records[mapper.byVNDB[record.VNDBID]] != record {
			t.Fatalf("Steam %d has a non-reciprocal mapping", steamID)
		}
	}
	for hikarinagiID, index := range mapper.byHikarinagi {
		record := mapper.records[index]
		if record.HikarinagiID != hikarinagiID || mapper.records[mapper.byVNDB[record.VNDBID]] != record {
			t.Fatalf("Hikarinagi %d has a non-reciprocal mapping", hikarinagiID)
		}
	}
}

func TestMapperPreservesLookupSemantics(t *testing.T) {
	records := []IDs{
		{VNDBID: 1, BangumiID: 2, SteamID: 3},
		{VNDBID: 4, BangumiID: 2, HikarinagiID: 5},
		{VNDBID: 0, SteamID: -1},
	}
	mapper := New(records)
	first, last := records[0], records[1]
	records[0] = IDs{VNDBID: 999}
	for _, test := range []struct {
		source enums.SourceType
		id     string
		want   IDs
		found  bool
	}{
		{enums.VNDB, " V1 ", first, true},
		{enums.Steam, "3", first, true},
		{enums.Bangumi, "2", last, true},
		{enums.Hikarinagi, "5", last, true},
		{enums.VNDB, "999", IDs{}, false},
		{enums.VNDB, "0", IDs{}, false},
		{enums.Steam, "-1", IDs{}, false},
		{enums.Steam, "invalid", IDs{}, false},
		{enums.Ymgal, "1", IDs{}, false},
	} {
		if got, found := mapper.Resolve(test.source, test.id); found != test.found || got != test.want {
			t.Errorf("Resolve(%s, %q) = %+v, %t; want %+v, %t", test.source, test.id, got, found, test.want, test.found)
		}
	}
	for _, empty := range []*Mapper{nil, New(nil), {}} {
		if got, found := empty.Resolve(enums.VNDB, "1"); found || got != (IDs{}) {
			t.Errorf("empty mapper Resolve = %+v, %t", got, found)
		}
	}
}

func BenchmarkMapperNew(b *testing.B) {
	mapper, err := LoadEmbedded()
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		runtime.KeepAlive(New(mapper.records))
	}
}

func BenchmarkMapperResolve(b *testing.B) {
	mapper, err := LoadEmbedded()
	if err != nil {
		b.Fatal(err)
	}
	id := strconv.FormatInt(mapper.records[len(mapper.records)/2].VNDBID, 10)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		mapper.Resolve(enums.VNDB, id)
	}
}
