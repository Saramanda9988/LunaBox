import { describe, expect, it } from "vitest";
import { parseReleaseFilters } from "../src/release-details";

describe("release detail filters", () => {
  it("parses supported filters and pagination", () => {
    const url = new URL("https://example.com/v1/admin/releases/2.0.0?page=2&page_size=50&event_type=install_failed&channel=windows-amd64-portable&from=2026-09-01T00:00:00Z");
    expect(parseReleaseFilters(url)).toEqual({
      page: 2,
      page_size: 50,
      event_type: "install_failed",
      channel: "windows-amd64-portable",
      architecture: undefined,
      build_mode: undefined,
      failure_code: undefined,
      failure_reason: undefined,
      from: "2026-09-01T00:00:00.000Z",
      to: undefined,
    });
  });

  it("bounds page size and ignores invalid values", () => {
    const url = new URL("https://example.com/v1/admin/releases/2.0.0?page=-1&page_size=999&event_type=unknown&channel=../../secret");
    const filters = parseReleaseFilters(url);
    expect(filters.page).toBe(1);
    expect(filters.page_size).toBe(100);
    expect(filters.event_type).toBeUndefined();
    expect(filters.channel).toBeUndefined();
  });
});
