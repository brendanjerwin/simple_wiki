import { create } from '@bufbuild/protobuf';
import { timestampFromMs } from '@bufbuild/protobuf/wkt';
import {
  ChecklistItemSchema,
  ChecklistSchema,
} from '../gen/api/v1/checklist_pb.js';
import type { Checklist, ChecklistItem } from '../gen/api/v1/checklist_pb.js';

/**
 * Build a {@link ChecklistItem} for tests/stories with sensible defaults.
 * Only the fields you care about need to be supplied.
 */
export function makeChecklistItem(overrides: Partial<{
  uid: string;
  text: string;
  checked: boolean;
  tags: string[];
  sortOrder: bigint;
  description: string;
  dueMs: number;
  alarmPayload: string;
  createdAtMs: number;
  updatedAtMs: number;
  completedAtMs: number;
  completedBy: string;
  automated: boolean;
}> = {}): ChecklistItem {
  return create(ChecklistItemSchema, {
    uid: overrides.uid ?? `uid-${Math.random().toString(36).slice(2, 10)}`,
    text: overrides.text ?? '',
    checked: overrides.checked ?? false,
    tags: overrides.tags ?? [],
    sortOrder: overrides.sortOrder ?? 1000n,
    description: overrides.description,
    due: overrides.dueMs !== undefined ? timestampFromMs(overrides.dueMs) : undefined,
    alarmPayload: overrides.alarmPayload,
    createdAt: overrides.createdAtMs !== undefined
      ? timestampFromMs(overrides.createdAtMs)
      : undefined,
    updatedAt: overrides.updatedAtMs !== undefined
      ? timestampFromMs(overrides.updatedAtMs)
      : undefined,
    completedAt: overrides.completedAtMs !== undefined
      ? timestampFromMs(overrides.completedAtMs)
      : undefined,
    completedBy: overrides.completedBy,
    automated: overrides.automated ?? false,
  });
}

/**
 * Build a {@link Checklist} for tests/stories. Items that are passed without
 * a sortOrder will be assigned monotonically increasing values.
 */
export function makeChecklist(overrides: Partial<{
  name: string;
  updatedAtMs: number;
  syncToken: bigint;
  items: ChecklistItem[];
}> = {}): Checklist {
  return create(ChecklistSchema, {
    name: overrides.name ?? '',
    updatedAt: overrides.updatedAtMs !== undefined
      ? timestampFromMs(overrides.updatedAtMs)
      : undefined,
    syncToken: overrides.syncToken ?? 0n,
    items: overrides.items ?? [],
    tombstones: [],
  });
}
