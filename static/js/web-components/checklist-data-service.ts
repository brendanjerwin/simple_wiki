import type { JsonObject } from '@bufbuild/protobuf';
import { create } from '@bufbuild/protobuf';
import {
  GetFrontmatterRequestSchema,
  MergeFrontmatterRequestSchema,
} from '../gen/api/v1/frontmatter_pb.js';
import type { ChecklistItem, ChecklistData } from './checklist-tag-parser.js';

// The FrontmatterClient interface (what we need from the gRPC client)
export interface FrontmatterClient {
  getFrontmatter(req: unknown): Promise<{ frontmatter?: JsonObject }>;
  mergeFrontmatter(req: unknown): Promise<{ frontmatter?: JsonObject }>;
}

/**
 * Narrow `value` to a non-null, non-array object, or return null.
 */
export function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- narrowed above: non-null, non-array object
  return value as Record<string, unknown>;
}

/**
 * Extract tags from a raw checklist item record.
 * Prefers new `tags` array format; falls back to old `tag` string.
 */
function parseItemTags(r: Record<string, unknown>): string[] {
  if (Array.isArray(r['tags'])) {
    return r['tags'].filter(
      (t): t is string => typeof t === 'string' && t !== ''
    );
  }
  if (typeof r['tag'] === 'string' && r['tag']) {
    return [r['tag']];
  }
  return [];
}

/**
 * Parse a single raw checklist item into a ChecklistItem, or return null if invalid.
 */
function parseChecklistItem(raw: unknown): ChecklistItem | null {
  const r = asRecord(raw);
  if (!r) return null;
  return {
    text: typeof r['text'] === 'string' ? r['text'] : '',
    checked: Boolean(r['checked']),
    tags: parseItemTags(r),
  };
}

/**
 * Extract ChecklistData from the raw frontmatter object.
 * Backward-compatible: reads both `tag` (old string) and `tags` (new array).
 */
export function extractChecklistData(
  frontmatter: JsonObject,
  listName: string
): ChecklistData {
  const checklistsObj = asRecord(frontmatter['checklists']);
  if (!checklistsObj) return { items: [] };

  const listObj = asRecord(checklistsObj[listName]);
  if (!listObj) return { items: [] };

  const rawItems = listObj['items'];
  if (!Array.isArray(rawItems)) return { items: [] };

  const items: ChecklistItem[] = [];
  for (const raw of rawItems) {
    const item = parseChecklistItem(raw);
    if (item) items.push(item);
  }
  return { items };
}

export class ChecklistDataService {
  constructor(private readonly client: FrontmatterClient) {}

  async fetch(page: string, listName: string): Promise<ChecklistData> {
    const request = create(GetFrontmatterRequestSchema, { page });
    const response = await this.client.getFrontmatter(request);
    return extractChecklistData(response.frontmatter ?? {}, listName);
  }

  async persist(page: string, listName: string, items: ChecklistItem[]): Promise<ChecklistData> {
    // Read current frontmatter
    const getRequest = create(GetFrontmatterRequestSchema, { page });
    const currentResponse = await this.client.getFrontmatter(getRequest);
    const currentFrontmatter: JsonObject = {
      ...currentResponse.frontmatter,
    };

    // Build updated checklist data
    const checklistData: JsonObject = {
      items: items.map(item => {
        const obj: JsonObject = { text: item.text, checked: item.checked };
        if (item.tags.length > 0) {
          obj['tags'] = item.tags;
        }
        return obj;
      }),
    };

    // Update the checklists key
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- asRecord narrows to non-null object; values originate from parsed JSON and are valid JsonValues
    const existingChecklists = (asRecord(currentFrontmatter['checklists']) ?? {}) as JsonObject;
    const updatedChecklists: JsonObject = {
      ...existingChecklists,
      [listName]: checklistData,
    };

    const mergeRequest = create(MergeFrontmatterRequestSchema, {
      page,
      frontmatter: { checklists: updatedChecklists },
    });
    const mergeResponse = await this.client.mergeFrontmatter(mergeRequest);

    return extractChecklistData(mergeResponse.frontmatter ?? {}, listName);
  }
}
