import { describe, expect, it } from "vitest";
import { getHumanReadableTime } from "./IndexInfo.vue";

describe("getHumanReadableTime", () => {
  it("reads lastIndexedUnixTime as Unix seconds", () => {
    const twoHoursAgo = Math.floor(Date.now() / 1000) - 2 * 3600;
    expect(getHumanReadableTime(twoHoursAgo, "en-us")).toBe("2 hours ago");
  });

  it("shows now for a zero timestamp", () => {
    expect(getHumanReadableTime(0, "en-us")).toBe("now");
  });
});
