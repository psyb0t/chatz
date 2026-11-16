import {
  GROUP_TODAY,
  GROUP_YESTERDAY,
  GROUP_WEEK,
  GROUP_MONTH,
} from "#lib/common/labels.js";

// ChatGroup is one sidebar section: a heading and the chats under it, newest
// first, in the order the list already has.
export interface ChatGroup<T> {
  label: string;
  items: T[];
}

const MS_PER_DAY = 86_400_000;
const WEEK_DAYS = 7;
const MONTH_DAYS = 30;

function startOfDay(date: Date): number {
  return new Date(
    date.getFullYear(),
    date.getMonth(),
    date.getDate(),
  ).getTime();
}

// groupLabel buckets one timestamp relative to now: Today, Yesterday, the
// previous 7 and 30 days, then the month ("March 2026").
export function groupLabel(updatedAt: string, now: Date): string {
  const at = new Date(updatedAt);
  if (Number.isNaN(at.getTime())) {
    return GROUP_TODAY;
  }

  const days = Math.floor((startOfDay(now) - startOfDay(at)) / MS_PER_DAY);
  if (days <= 0) {
    return GROUP_TODAY;
  }

  if (days === 1) {
    return GROUP_YESTERDAY;
  }

  if (days < WEEK_DAYS) {
    return GROUP_WEEK;
  }

  if (days < MONTH_DAYS) {
    return GROUP_MONTH;
  }

  return at.toLocaleDateString(undefined, { month: "long", year: "numeric" });
}

// groupChats splits an already sorted list into consecutive date sections.
export function groupChats<T extends { updatedAt: string }>(
  list: readonly T[],
  now: Date,
): ChatGroup<T>[] {
  const groups: ChatGroup<T>[] = [];

  for (const item of list) {
    const label = groupLabel(item.updatedAt, now);
    const last = groups[groups.length - 1];
    if (last !== undefined && last.label === label) {
      last.items.push(item);
    } else {
      groups.push({ label, items: [item] });
    }
  }

  return groups;
}
