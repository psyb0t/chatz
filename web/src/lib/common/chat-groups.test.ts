import { describe, expect, it } from "vitest";
import { groupChats, groupLabel } from "./chat-groups";

const NOW = new Date(2026, 10, 16, 15, 0, 0);

function daysAgo(days: number): string {
  return new Date(2026, 10, 16 - days, 12, 0, 0).toISOString();
}

describe("groupLabel", () => {
  it("buckets by calendar day relative to now", () => {
    expect(groupLabel(daysAgo(0), NOW)).toBe("Today");
    expect(groupLabel(daysAgo(1), NOW)).toBe("Yesterday");
    expect(groupLabel(daysAgo(3), NOW)).toBe("Previous 7 days");
    expect(groupLabel(daysAgo(20), NOW)).toBe("Previous 30 days");
    expect(groupLabel(daysAgo(90), NOW)).toMatch(/2026/);
  });

  it("treats a future or unparsable time as today", () => {
    expect(groupLabel(daysAgo(-2), NOW)).toBe("Today");
    expect(groupLabel("not a date", NOW)).toBe("Today");
  });
});

describe("groupChats", () => {
  it("keeps list order and merges consecutive items of one bucket", () => {
    const groups = groupChats(
      [
        { id: "a", updatedAt: daysAgo(0) },
        { id: "b", updatedAt: daysAgo(0) },
        { id: "c", updatedAt: daysAgo(1) },
        { id: "d", updatedAt: daysAgo(4) },
      ],
      NOW,
    );

    expect(groups.map((group) => group.label)).toEqual([
      "Today",
      "Yesterday",
      "Previous 7 days",
    ]);
    expect(groups[0].items.map((item) => item.id)).toEqual(["a", "b"]);
  });
});
