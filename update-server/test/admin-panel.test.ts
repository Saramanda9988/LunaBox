import { describe, expect, it } from "vitest";
import { ADMIN_HTML, ADMIN_SCRIPT } from "../src/admin-panel";

describe("admin panel assets", () => {
  it("ships a parseable dashboard script", () => {
    // new Function only parses the body, so this catches syntax errors in the
    // embedded script without running it against the DOM.
    expect(() => new Function(ADMIN_SCRIPT)).not.toThrow();
  });

  it("styles the failure reason column", () => {
    expect(ADMIN_HTML).toContain(".failure-reason");
  });
});
