import { create, type MessageInitShape } from '@bufbuild/protobuf';
import { timestampFromMs } from '@bufbuild/protobuf/wkt';
import {
  ChecklistItemSchema,
  ChecklistSchema,
} from '../gen/api/v1/checklist_pb.js';
import type {
  Checklist,
  ChecklistItem,
} from '../gen/api/v1/checklist_pb.js';

// Test-fixture uid generator. A monotonic counter keeps tests
// deterministic and quiet: no Math.random() (which CodeQL flags as
// insecure-randomness in a security context — irrelevant for fixtures
// but cheaper to avoid than to suppress).
let nextFixtureUidSeq = 0;
function nextFixtureUid(): string {
  nextFixtureUidSeq += 1;
  return `uid-test-${nextFixtureUidSeq.toString().padStart(8, '0')}`;
}

export interface ChecklistItemOverrides {
  uid?: string;
  text?: string;
  checked?: boolean;
  tags?: string[];
  sortOrder?: bigint;
  description?: string;
  dueMs?: number;
  alarmPayload?: string;
  createdAtMs?: number;
  updatedAtMs?: number;
  completedAtMs?: number;
  completedBy?: string;
  automated?: boolean;
}

/**
 * Build a {@link ChecklistItem} for tests/stories with sensible defaults.
 * Only the fields you care about need to be supplied. Fields the proto
 * marks `optional` are only set on the message when the override is provided
 * (matching `exactOptionalPropertyTypes`).
 */
export function makeChecklistItem(overrides: ChecklistItemOverrides = {}): ChecklistItem {
  const init: MessageInitShape<typeof ChecklistItemSchema> = {
    uid: overrides.uid ?? nextFixtureUid(),
    text: overrides.text ?? '',
    checked: overrides.checked ?? false,
    tags: overrides.tags ?? [],
    sortOrder: overrides.sortOrder ?? 1000n,
    automated: overrides.automated ?? false,
  };
  if (overrides.description !== undefined) init.description = overrides.description;
  if (overrides.dueMs !== undefined) init.due = timestampFromMs(overrides.dueMs);
  if (overrides.alarmPayload !== undefined) init.alarmPayload = overrides.alarmPayload;
  if (overrides.createdAtMs !== undefined) init.createdAt = timestampFromMs(overrides.createdAtMs);
  if (overrides.updatedAtMs !== undefined) init.updatedAt = timestampFromMs(overrides.updatedAtMs);
  if (overrides.completedAtMs !== undefined) init.completedAt = timestampFromMs(overrides.completedAtMs);
  if (overrides.completedBy !== undefined) init.completedBy = overrides.completedBy;
  return create(ChecklistItemSchema, init);
}

export interface ChecklistOverrides {
  name?: string;
  updatedAtMs?: number;
  syncToken?: bigint;
  items?: ChecklistItem[];
}

/**
 * Build a {@link Checklist} for tests/stories.
 */
export function makeChecklist(overrides: ChecklistOverrides = {}): Checklist {
  const init: MessageInitShape<typeof ChecklistSchema> = {
    name: overrides.name ?? '',
    syncToken: overrides.syncToken ?? 0n,
    items: overrides.items ?? [],
    tombstones: [],
  };
  if (overrides.updatedAtMs !== undefined) {
    init.updatedAt = timestampFromMs(overrides.updatedAtMs);
  }
  return create(ChecklistSchema, init);
}
