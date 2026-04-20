import { html, LitElement, nothing } from 'lit';
import { property, state } from 'lit/decorators.js';
import { createClient } from '@connectrpc/connect';
import { create, type JsonObject } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from './grpc-transport.js';
import {
  Frontmatter,
  GetFrontmatterRequestSchema,
  MergeFrontmatterRequestSchema,
} from '../gen/api/v1/frontmatter_pb.js';
import {
  foundationCSS,
  buttonCSS,
  inputCSS,
  sharedStyles,
} from './shared-styles.js';
import { AugmentErrorService, type AugmentedError } from './augment-error-service.js';
import './error-display.js';
import {
  extractSurveyData,
  findUserResponse,
  upsertResponse,
  asRecord,
  type SurveyData,
  type SurveyField,
  type SurveyResponse,
} from './survey-data-service.js';
import { wikiSurveyStyles } from './wiki-survey-styles.js';

export type { SurveyData, SurveyField, SurveyResponse };

/**
 * WikiSurvey - An interactive survey component that persists responses to frontmatter.
 *
 * Survey configuration (question, fields) lives in frontmatter under `surveys.<name>`.
 * Responses are stored as an array in `surveys.<name>.responses`, keyed by username.
 * Each user's response is editable (resubmit replaces their existing entry).
 *
 * @property {string} name - Survey name in frontmatter (matches frontmatter key)
 * @property {string} page - Page identifier for gRPC calls
 *
 * @example
 * <wiki-survey name="pastry_2026_w15" page="pastry_project"></wiki-survey>
 */
export class WikiSurvey extends LitElement {
  static override readonly styles = [foundationCSS, buttonCSS, inputCSS, wikiSurveyStyles];

  @property({ type: String })
  declare name: string;

  @property({ type: String })
  declare page: string;

  @state()
  declare surveyData: SurveyData | null;

  @state()
  declare loading: boolean;

  @state()
  declare saving: boolean;

  @state()
  declare saved: boolean;

  @state()
  declare error: AugmentedError | null;

  @state()
  private declare fieldValues: Record<string, unknown>;

  readonly client = createClient(Frontmatter, getGrpcWebTransport());

  constructor() {
    super();
    this.name = '';
    this.page = '';
    this.surveyData = null;
    this.loading = false;
    this.saving = false;
    this.saved = false;
    this.error = null;
    this.fieldValues = {};
  }

  /**
   * Return the current user's login name from the page-level simple_wiki global.
   * Empty string means anonymous / not authenticated.
   */
  get _currentUsername(): string {
    return globalThis.simple_wiki?.username ?? '';
  }

  override connectedCallback(): void {
    super.connectedCallback();
    // Remove server-rendered fallback content now that JS has taken over.
    this.innerHTML = '';
    if (this.page) {
      this.loading = true;
      void this._fetchData();
    }
  }

  private async _fetchData(): Promise<void> {
    if (!this.page) {
      throw new Error('wiki-survey: page attribute is required but not set');
    }
    try {
      const request = create(GetFrontmatterRequestSchema, { page: this.page });
      const response = await this.client.getFrontmatter(request);
      const data = extractSurveyData(response.frontmatter ?? {}, this.name);
      this.surveyData = data;
      this._prefillCurrentUserValues(data);
      this.error = null;
    } catch (err) {
      this.error = AugmentErrorService.augmentError(err, 'loading survey');
    } finally {
      this.loading = false;
    }
  }

  private _prefillCurrentUserValues(data: SurveyData): void {
    const username = this._currentUsername;
    if (!username) return;
    const existing = findUserResponse(data.responses, username);
    if (existing) {
      this.fieldValues = { ...existing.values };
    }
  }

  private async _handleSubmit(): Promise<void> {
    if (!this.page) {
      throw new Error('wiki-survey: page attribute is required but not set');
    }
    const username = this._currentUsername;
    if (!username) return;

    try {
      this.saving = true;
      this.saved = false;

      // Read-modify-write: get current frontmatter, update survey responses, merge back.
      const getRequest = create(GetFrontmatterRequestSchema, { page: this.page });
      const currentResponse = await this.client.getFrontmatter(getRequest);
      const currentFrontmatter: JsonObject = { ...currentResponse.frontmatter };

      // Get current surveys object
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- asRecord narrows to non-null object; values originate from parsed JSON and are valid JsonValues
      const existingSurveys = (asRecord(currentFrontmatter['surveys']) ?? {}) as JsonObject;

      // Get current survey data (to preserve question, fields, etc.)
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- same as above
      const currentSurveyObj = (asRecord(existingSurveys[this.name]) ?? {}) as JsonObject;

      // Get existing responses and upsert the current user's response
      const existingData = extractSurveyData(currentFrontmatter as JsonObject, this.name);
      const updatedResponses = upsertResponse(existingData.responses, username, this.fieldValues);

      // Build updated survey object (preserve all existing keys, update responses)
      const updatedSurveyObj: JsonObject = {
        ...currentSurveyObj,
        responses: updatedResponses.map(r => ({
          user: r.user,
          anonymous: r.anonymous,
          submitted_at: r.submitted_at,
          // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- r.values is Record<string, unknown>; values are form inputs (strings/primitives) so JsonObject is safe here
          values: r.values as unknown as JsonObject,
        })),
      };

      const updatedSurveys: JsonObject = {
        ...existingSurveys,
        [this.name]: updatedSurveyObj,
      };

      const mergeRequest = create(MergeFrontmatterRequestSchema, {
        page: this.page,
        frontmatter: { surveys: updatedSurveys },
      });
      const mergeResponse = await this.client.mergeFrontmatter(mergeRequest);

      // Update local state from the response
      if (mergeResponse.frontmatter) {
        const data = extractSurveyData(mergeResponse.frontmatter, this.name);
        this.surveyData = data;
      }

      this.saved = true;
      this.error = null;
    } catch (err) {
      this.error = AugmentErrorService.augmentError(err, 'saving survey response');
    } finally {
      this.saving = false;
    }
  }

  private _handleFieldChange(fieldName: string, value: unknown): void {
    this.fieldValues = { ...this.fieldValues, [fieldName]: value };
    this.saved = false;
  }

  private _renderField(field: SurveyField) {
    const currentValue = this.fieldValues[field.name];

    switch (field.type) {
      case 'number': {
        const numVal = typeof currentValue === 'number' ? currentValue : '';
        return html`
          <div class="field-group">
            <label class="field-label" for="field-${field.name}">
              ${field.name}${field.required ? html`<span class="required-indicator" aria-hidden="true"> *</span>` : nothing}
            </label>
            <input
              id="field-${field.name}"
              type="number"
              class="field-input"
              .value="${String(numVal)}"
              min="${field.min ?? nothing}"
              max="${field.max ?? nothing}"
              aria-required="${field.required ? 'true' : nothing}"
              ?disabled="${this.saving}"
              @input="${(e: InputEvent) => {
                if (!(e.target instanceof HTMLInputElement)) return;
                const n = parseFloat(e.target.value);
                this._handleFieldChange(field.name, isNaN(n) ? '' : n);
              }}"
            />
          </div>
        `;
      }
      case 'boolean': {
        const boolVal = Boolean(currentValue);
        return html`
          <div class="field-group">
            <div class="checkbox-group">
              <input
                id="field-${field.name}"
                type="checkbox"
                .checked="${boolVal}"
                aria-required="${field.required ? 'true' : nothing}"
                ?disabled="${this.saving}"
                @change="${(e: Event) => {
                  if (!(e.target instanceof HTMLInputElement)) return;
                  this._handleFieldChange(field.name, e.target.checked);
                }}"
              />
              <label class="field-label" for="field-${field.name}">
                ${field.name}${field.required ? html`<span class="required-indicator" aria-hidden="true"> *</span>` : nothing}
              </label>
            </div>
          </div>
        `;
      }
      case 'choice': {
        const choiceVal = typeof currentValue === 'string' ? currentValue : '';
        const options = field.options ?? [];
        return html`
          <div class="field-group">
            <label class="field-label" for="field-${field.name}">
              ${field.name}${field.required ? html`<span class="required-indicator" aria-hidden="true"> *</span>` : nothing}
            </label>
            <select
              id="field-${field.name}"
              class="field-input"
              .value="${choiceVal}"
              aria-required="${field.required ? 'true' : nothing}"
              ?disabled="${this.saving}"
              @change="${(e: Event) => {
                if (!(e.target instanceof HTMLSelectElement)) return;
                this._handleFieldChange(field.name, e.target.value);
              }}"
            >
              <option value="">-- select --</option>
              ${options.map(opt => html`<option value="${opt}" ?selected="${choiceVal === opt}">${opt}</option>`)}
            </select>
          </div>
        `;
      }
      default: {
        // text
        const textVal = typeof currentValue === 'string' ? currentValue : '';
        return html`
          <div class="field-group">
            <label class="field-label" for="field-${field.name}">
              ${field.name}${field.required ? html`<span class="required-indicator" aria-hidden="true"> *</span>` : nothing}
            </label>
            <input
              id="field-${field.name}"
              type="text"
              class="field-input"
              .value="${textVal}"
              aria-required="${field.required ? 'true' : nothing}"
              ?disabled="${this.saving}"
              @input="${(e: InputEvent) => {
                if (!(e.target instanceof HTMLInputElement)) return;
                this._handleFieldChange(field.name, e.target.value);
              }}"
            />
          </div>
        `;
      }
    }
  }

  private _renderResponses(responses: SurveyResponse[]) {
    if (responses.length === 0) return nothing;

    return html`
      <div class="responses-section">
        <p class="responses-title">${responses.length} response${responses.length === 1 ? '' : 's'}</p>
        ${responses.map(r => {
          const date = r.submitted_at.length >= 10 ? r.submitted_at.slice(0, 10) : r.submitted_at;
          const valuePairs = Object.entries(r.values)
            .map(([k, v]) => `${k}: ${String(v)}`)
            .join(', ');
          return html`
            <div class="response-item">
              <span class="response-user">${r.user}</span>
              <span class="response-date"> · ${date}</span>
              <div class="response-values">${valuePairs}</div>
            </div>
          `;
        })}
      </div>
    `;
  }

  override render() {
    if (this.loading) {
      return html`
        ${sharedStyles}
        <div class="survey-container system-font">
          <div class="loading" role="status" aria-live="polite">Loading survey\u2026</div>
        </div>
      `;
    }

    if (this.error) {
      return html`
        ${sharedStyles}
        <div class="survey-container system-font">
          <div class="error-wrapper" role="alert">
            <error-display
              .augmentedError="${this.error}"
              .action="${{
                label: 'Retry',
                onClick: () => {
                  this.error = null;
                  this.loading = true;
                  void this._fetchData();
                },
              }}"
            ></error-display>
          </div>
        </div>
      `;
    }

    const data = this.surveyData;
    if (!data || !data.question) {
      return html`
        ${sharedStyles}
        <div class="survey-container system-font">
          <p class="not-configured">Survey not configured. Add question and fields to frontmatter under <code>surveys.${this.name}</code>.</p>
        </div>
      `;
    }

    const username = this._currentUsername;

    return html`
      ${sharedStyles}
      <div class="survey-container system-font">
        <p class="survey-question" id="survey-question-${this.name}">${data.question}</p>

        ${data.closed
          ? html`<p class="closed-notice">This survey is closed.</p>`
          : username
            ? html`
                <div class="survey-fields" role="group" aria-labelledby="survey-question-${this.name}">
                  ${data.fields.map(f => this._renderField(f))}
                </div>
                <div class="submit-row">
                  <button
                    type="button"
                    class="submit-btn button-base button-primary"
                    ?disabled="${this.saving || data.fields.length === 0}"
                    @click="${this._handleSubmit}"
                  >
                    Submit
                  </button>
                  <div role="status" aria-live="polite" class="submit-status">
                    ${this.saving
                      ? html`<span class="saving-indicator">Saving\u2026</span>`
                      : this.saved
                        ? html`<span class="success-message">Response saved!</span>`
                        : nothing}
                  </div>
                </div>
              `
            : html`<p class="login-required">Log in to submit a response.</p>`}

        ${this._renderResponses(data.responses)}
      </div>
    `;
  }
}

customElements.define('wiki-survey', WikiSurvey);

declare global {
  interface HTMLElementTagNameMap {
    'wiki-survey': WikiSurvey;
  }
}
