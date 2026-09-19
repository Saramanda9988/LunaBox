import { describe, expect, it } from "vitest";
import {
  assetObjectKey,
  channelObjectKey,
  manifestObjectKey,
  parseUpdateEvent,
  versionObjectKey,
} from "../src/validation";

describe("update object keys", () => {
  it("builds versioned keys", () => {
    expect(channelObjectKey("windows-stable")).toBe("channels/windows-stable/version.json");
    expect(manifestObjectKey("2.0.0-test.3")).toBe("releases/2.0.0-test.3/manifest.json");
    expect(versionObjectKey("2.0.0-test.3")).toBe("releases/2.0.0-test.3/version.json");
    expect(assetObjectKey("2.0.0-test.3", "LunaBox.exe.zst")).toBe("releases/2.0.0-test.3/LunaBox.exe.zst");
  });

  it("rejects directory traversal", () => {
    expect(() => assetObjectKey("2.0.0", "../secret")).toThrow();
  });
});

describe("update events", () => {
  it("accepts a verified download event", () => {
    const event = parseUpdateEvent({
      event_id: "event_12345678",
      transaction_id: "transaction_12345678",
      event_type: "download_verified",
      current_version: "1.11.2",
      target_version: "2.0.0-test.3",
      channel: "windows-amd64-portable",
      architecture: "amd64",
      build_mode: "portable",
      artifact: "LunaBox.exe.zst",
      transferred_bytes: 1024,
      client_time: "2026-08-17T10:00:00Z",
    });

    expect(event.event_type).toBe("download_verified");
    expect(event.transferred_bytes).toBe(1024);
  });

  it("keeps the anonymous installation identifier", () => {
    const event = parseUpdateEvent({
      event_id: "event_12345678",
      transaction_id: "transaction_12345678",
      installation_id: "7cb8064e-479b-49a9-9f48-45ff62e62c66",
      event_type: "install_success",
      current_version: "1.13.0",
      target_version: "1.14.0",
      channel: "windows-amd64-portable",
      architecture: "amd64",
      build_mode: "portable",
    });

    expect(event.installation_id).toBe("7cb8064e-479b-49a9-9f48-45ff62e62c66");
  });

  it("rejects unsupported event types", () => {
    expect(() => parseUpdateEvent({
      event_id: "event_12345678",
      event_type: "unknown",
      target_version: "2.0.0",
      channel: "windows-amd64-portable",
      architecture: "amd64",
      build_mode: "portable",
    })).toThrow("invalid event_type");
  });

  it("keeps a normalized failure reason", () => {
    const event = parseUpdateEvent({
      event_id: "event_12345678",
      event_type: "install_failed",
      target_version: "2.0.0",
      channel: "windows-amd64-portable",
      architecture: "amd64",
      build_mode: "portable",
      failure_code: "prepare_failed",
      failure_reason: "signature_invalid",
    });

    expect(event.failure_code).toBe("prepare_failed");
    expect(event.failure_reason).toBe("signature_invalid");
  });

  it("drops a free-form failure reason without rejecting the event", () => {
    const event = parseUpdateEvent({
      event_id: "event_12345678",
      event_type: "install_failed",
      target_version: "2.0.0",
      channel: "windows-amd64-portable",
      architecture: "amd64",
      build_mode: "portable",
      failure_code: "prepare_failed",
      failure_reason: "verify Authenticode signature for C:\\Users\\alice\\LunaBox.exe",
    });

    expect(event.failure_code).toBe("prepare_failed");
    expect(event.failure_reason).toBeUndefined();
  });
});
