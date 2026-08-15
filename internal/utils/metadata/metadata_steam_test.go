package metadata

import (
	"encoding/json"
	"io"
	"lunabox/internal/common/enums"
	"lunabox/internal/version"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestBuildSteamPortraitCoverURLsPrioritizesLanguage(t *testing.T) {
	got := buildSteamPortraitCoverURLs(12345, "schinese")
	want := []string{
		"https://cdn.akamai.steamstatic.com/steam/apps/12345/library_600x900_schinese.jpg",
		"https://cdn.akamai.steamstatic.com/steam/apps/12345/library_600x900.jpg",
		"https://cdn.akamai.steamstatic.com/steam/apps/12345/library_600x900_english.jpg",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected Steam portrait cover candidates: got %v, want %v", got, want)
	}
}

func TestResolveSteamCoverURLUsesFirstAvailablePortrait(t *testing.T) {
	requested := make([]string, 0, 2)
	client := &http.Client{Transport: metadataRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		requested = append(requested, req.URL.String())
		status := http.StatusNotFound
		if strings.HasSuffix(req.URL.Path, "/library_600x900.jpg") {
			status = http.StatusOK
		}
		return steamCoverTestResponse(req, status, "image/jpeg"), nil
	})}

	getter := NewSteamInfoGetterWithLanguage("zh-CN", WithHTTPClient(client))
	got := getter.resolveSteamCoverURL(12345, "schinese", "https://example.com/header.jpg")
	want := "https://cdn.akamai.steamstatic.com/steam/apps/12345/library_600x900.jpg"
	if got != want {
		t.Fatalf("unexpected resolved Steam cover: got %q, want %q", got, want)
	}
	if len(requested) != 2 {
		t.Fatalf("expected localized and generic portrait probes, got %v", requested)
	}
}

func TestResolveSteamCoverURLFallsBackToHeaderImage(t *testing.T) {
	client := &http.Client{Transport: metadataRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return steamCoverTestResponse(req, http.StatusNotFound, "text/html"), nil
	})}

	getter := NewSteamInfoGetterWithLanguage("zh-CN", WithHTTPClient(client))
	const headerImage = "https://example.com/header.jpg"
	if got := getter.resolveSteamCoverURL(12345, "schinese", headerImage); got != headerImage {
		t.Fatalf("expected Steam header fallback %q, got %q", headerImage, got)
	}
}

func TestResolveSteamCoverURLUsesHeaderImageForLandscape(t *testing.T) {
	requestCount := 0
	client := &http.Client{Transport: metadataRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		return steamCoverTestResponse(req, http.StatusOK, "image/jpeg"), nil
	})}

	getter := NewSteamInfoGetterWithLanguage(
		"zh-CN",
		WithHTTPClient(client),
		WithSteamCoverOrientation(enums.SteamCoverOrientationLandscape),
	)
	const headerImage = "https://example.com/header.jpg"
	if got := getter.resolveSteamCoverURL(12345, "schinese", headerImage); got != headerImage {
		t.Fatalf("expected Steam landscape cover %q, got %q", headerImage, got)
	}
	if requestCount != 0 {
		t.Fatalf("expected no portrait cover probes, got %d requests", requestCount)
	}
}

func TestFetchSteamMetadataStoresPortraitAsCoverSource(t *testing.T) {
	originalLimiter := sharedMetadataRateLimiter
	defer func() {
		sharedMetadataRateLimiter = originalLimiter
	}()
	sharedMetadataRateLimiter = newMetadataRateLimiter(nil)

	client := &http.Client{Transport: metadataRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodHead {
			status := http.StatusNotFound
			if strings.HasSuffix(req.URL.Path, "/library_600x900_schinese.jpg") {
				status = http.StatusOK
			}
			return steamCoverTestResponse(req, status, "image/jpeg"), nil
		}

		body := `{
			"12345": {
				"success": true,
				"data": {
					"steam_appid": 12345,
					"name": "Sample Game",
					"header_image": "https://example.com/header.jpg",
					"short_description": "Sample",
					"metacritic": {"score": 80}
				}
			}
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})}

	getter := NewSteamInfoGetterWithLanguage("zh-CN", WithHTTPClient(client))
	result, err := getter.FetchMetadata("12345", "")
	if err != nil {
		t.Fatalf("failed to fetch Steam metadata: %v", err)
	}

	want := "https://cdn.akamai.steamstatic.com/steam/apps/12345/library_600x900_schinese.jpg"
	if result.Game.CoverURL != want || result.Game.CoverSourceURL != want {
		t.Fatalf("expected localized portrait cover %q, got cover=%q source=%q", want, result.Game.CoverURL, result.Game.CoverSourceURL)
	}
}

func TestFetchSteamCommunityTagsUsesPopularUserTags(t *testing.T) {
	originalLimiter := sharedMetadataRateLimiter
	defer func() {
		sharedMetadataRateLimiter = originalLimiter
	}()
	sharedMetadataRateLimiter = newMetadataRateLimiter(nil)

	browseRequests := 0
	catalogRequests := 0
	client := &http.Client{Transport: metadataRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("User-Agent"); got != version.UserAgent() {
			t.Fatalf("unexpected Steam community tag user agent: %q", got)
		}

		switch req.URL.Path {
		case "/IStoreBrowseService/GetItems/v1/":
			browseRequests++
			var input steamStoreBrowseRequest
			if err := json.Unmarshal([]byte(req.URL.Query().Get("input_json")), &input); err != nil {
				t.Fatalf("failed to decode Steam store browse request: %v", err)
			}
			if len(input.IDs) != 1 || input.IDs[0].AppID != 12345 {
				t.Fatalf("unexpected Steam store browse app IDs: %#v", input.IDs)
			}
			if input.Context.Language != "schinese" || input.Context.CountryCode != "CN" {
				t.Fatalf("unexpected Steam store browse context: %#v", input.Context)
			}
			if input.DataRequest.IncludeTagCount != 3 {
				t.Fatalf("unexpected Steam store browse tag count: %d", input.DataRequest.IncludeTagCount)
			}
			body := `{
				"response": {
					"store_items": [{
						"appid": 12345,
						"success": 1,
						"visible": true,
						"tags": [
							{"tagid": 1742, "weight": 100},
							{"tagid": 6971, "weight": 80},
							{"tagid": 5608, "weight": 50}
						]
					}]
				}
			}`
			return steamTestResponse(req, http.StatusOK, "application/json", body), nil
		case "/IStoreService/GetMostPopularTags/v1/":
			catalogRequests++
			if got := req.URL.Query().Get("language"); got != "schinese" {
				t.Fatalf("unexpected Steam tag catalog language: %q", got)
			}
			body := `{
				"response": {
					"tags": [
						{"tagid": 1742, "name": "剧情丰富"},
						{"tagid": 6971, "name": "多结局"},
						{"tagid": 5608, "name": "情感"}
					]
				}
			}`
			return steamTestResponse(req, http.StatusOK, "application/json", body), nil
		default:
			t.Fatalf("unexpected Steam community tag request path: %s", req.URL.Path)
			return nil, nil
		}
	})}

	getter := NewSteamInfoGetterWithLanguage("zh-CN", WithHTTPClient(client), WithTagLimit(3))
	getter.tagCatalog = newSteamTagCatalogCache()
	tags, err := getter.fetchCommunityTags(12345, "schinese")
	if err != nil {
		t.Fatalf("failed to fetch Steam community tags: %v", err)
	}

	gotNames := make([]string, 0, len(tags))
	for _, tag := range tags {
		gotNames = append(gotNames, tag.Name)
		if tag.Source != "steam" {
			t.Fatalf("unexpected Steam tag source: %q", tag.Source)
		}
	}
	wantNames := []string{"剧情丰富", "多结局", "情感"}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("unexpected Steam community tags: got %v, want %v", gotNames, wantNames)
	}
	if tags[0].Weight != 1 || tags[1].Weight != 0.8 || tags[2].Weight != 0.5 {
		t.Fatalf("expected normalized Steam API tag weights, got %#v", tags)
	}

	if _, err := getter.fetchCommunityTags(12345, "schinese"); err != nil {
		t.Fatalf("failed to fetch Steam community tags with cached catalog: %v", err)
	}
	if browseRequests != 2 || catalogRequests != 1 {
		t.Fatalf("unexpected Steam community tag request counts: browse=%d catalog=%d", browseRequests, catalogRequests)
	}
}

func TestFetchSteamCommunityTagsFallsBackToStorePage(t *testing.T) {
	originalLimiter := sharedMetadataRateLimiter
	defer func() {
		sharedMetadataRateLimiter = originalLimiter
	}()
	sharedMetadataRateLimiter = newMetadataRateLimiter(nil)

	client := &http.Client{Transport: metadataRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("User-Agent"); got != version.UserAgent() {
			t.Fatalf("unexpected Steam community tag user agent: %q", got)
		}

		switch req.URL.Path {
		case "/IStoreBrowseService/GetItems/v1/":
			return steamTestResponse(req, http.StatusServiceUnavailable, "application/json", "{}"), nil
		case "/app/12345/":
			if got := req.URL.Query().Get("l"); got != "schinese" {
				t.Fatalf("unexpected Steam store page language: %q", got)
			}
			if !strings.Contains(req.Header.Get("Cookie"), "lastagecheckage=") {
				t.Fatalf("expected Steam age-check cookie, got %q", req.Header.Get("Cookie"))
			}
			body := `
				<div class="glance_tags popular_tags">
					<a class="app_tag">剧情丰富</a>
					<a class="app_tag">多结局</a>
				</div>`
			return steamTestResponse(req, http.StatusOK, "text/html", body), nil
		default:
			t.Fatalf("unexpected Steam community tag fallback request path: %s", req.URL.Path)
			return nil, nil
		}
	})}

	getter := NewSteamInfoGetterWithLanguage("zh-CN", WithHTTPClient(client), WithTagLimit(2))
	tags, err := getter.fetchCommunityTags(12345, "schinese")
	if err != nil {
		t.Fatalf("failed to fetch Steam community tags from store page: %v", err)
	}
	gotNames := []string{tags[0].Name, tags[1].Name}
	wantNames := []string{"剧情丰富", "多结局"}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("unexpected Steam store page tags: got %v, want %v", gotNames, wantNames)
	}
}

func TestFetchSteamMetadataFallsBackToGenresWithoutCategories(t *testing.T) {
	originalLimiter := sharedMetadataRateLimiter
	defer func() {
		sharedMetadataRateLimiter = originalLimiter
	}()
	sharedMetadataRateLimiter = newMetadataRateLimiter(nil)

	client := &http.Client{Transport: metadataRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodHead {
			return steamCoverTestResponse(req, http.StatusNotFound, "text/html"), nil
		}

		switch req.URL.Path {
		case "/api/appdetails":
			body := `{
				"12345": {
					"success": true,
					"data": {
						"steam_appid": 12345,
						"name": "Sample Game",
						"header_image": "https://example.com/header.jpg",
						"metacritic": {"score": 80},
						"genres": [
							{"id": "25", "description": "冒险"},
							{"id": "23", "description": "独立"}
						],
						"categories": [
							{"id": 1, "description": "单人"},
							{"id": 23, "description": "Steam 云"}
						]
					}
				}
			}`
			return steamTestResponse(req, http.StatusOK, "application/json", body), nil
		case "/IStoreBrowseService/GetItems/v1/":
			return steamTestResponse(req, http.StatusServiceUnavailable, "application/json", "{}"), nil
		case "/app/12345/":
			return steamTestResponse(req, http.StatusServiceUnavailable, "text/html", "temporarily unavailable"), nil
		default:
			t.Fatalf("unexpected Steam metadata request path: %s", req.URL.Path)
			return nil, nil
		}
	})}

	getter := NewSteamInfoGetterWithLanguage("zh-CN", WithHTTPClient(client))
	result, err := getter.FetchMetadata("12345", "")
	if err != nil {
		t.Fatalf("failed to fetch Steam metadata with genre fallback: %v", err)
	}

	gotNames := make([]string, 0, len(result.Tags))
	for _, tag := range result.Tags {
		gotNames = append(gotNames, tag.Name)
	}
	wantNames := []string{"冒险", "独立"}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("unexpected Steam genre fallback tags: got %v, want %v", gotNames, wantNames)
	}
}

func steamCoverTestResponse(req *http.Request, status int, contentType string) *http.Response {
	return steamTestResponse(req, status, contentType, "")
}

func steamTestResponse(req *http.Request, status int, contentType string, body string) *http.Response {
	header := make(http.Header)
	header.Set("Content-Type", contentType)
	return &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}
