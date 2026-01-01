import { createClient } from '@connectrpc/connect';
import type { JsonObject } from '@bufbuild/protobuf';
import { create } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from './grpc-transport.js';
import {
  PageManagementService,
  CreatePageRequestSchema,
  ListTemplatesRequestSchema,
  GenerateIdentifierRequestSchema,
  type TemplateInfo,
  type ExistingPageInfo,
} from '../gen/api/v1/page_management_pb.js';
import {
  Frontmatter,
  GetFrontmatterRequestSchema,
} from '../gen/api/v1/frontmatter_pb.js';
import { AugmentErrorService } from './augment-error-service.js';
import { showToast, showToastAfter } from './toast-message.js';

const SUCCESS_TOAST_DURATION_SECONDS = 5;
const ERROR_TOAST_DURATION_SECONDS = 8;

/**
 * PageCreator - Creates wiki pages via gRPC APIs
 *
 * This class manages page creation operations including template support.
 *
 * Usage:
 * ```typescript
 * const creator = new PageCreator();
 * const templates = await creator.listTemplates();
 * const result = await creator.createPage('my_page', 'My Page', 'article_template', { author: 'John' });
 * ```
 */
export class PageCreator {
  private pageManagementClient = createClient(PageManagementService, getGrpcWebTransport());
  private frontmatterClient = createClient(Frontmatter, getGrpcWebTransport());

  /**
   * Lists all available page templates
   * @param excludeIdentifiers Template identifiers to exclude (e.g., ['inv_item'])
   * @returns Array of template info or error
   */
  async listTemplates(excludeIdentifiers: string[] = []): Promise<{
    templates: TemplateInfo[];
    error?: Error;
  }> {
    try {
      const request = create(ListTemplatesRequestSchema, {
        excludeIdentifiers,
      });

      const response = await this.pageManagementClient.listTemplates(request);

      return {
        templates: response.templates,
      };
    } catch (err) {
      const augmentedError = AugmentErrorService.augmentError(err, 'list templates');
      return {
        templates: [],
        error: augmentedError,
      };
    }
  }

  /**
   * Gets the frontmatter for a template (templates are just pages)
   * @param templateIdentifier The template page identifier
   * @returns The template's frontmatter or error
   */
  async getTemplateFrontmatter(templateIdentifier: string): Promise<{
    frontmatter: JsonObject;
    error?: Error;
  }> {
    try {
      const request = create(GetFrontmatterRequestSchema, {
        page: templateIdentifier,
      });

      const response = await this.frontmatterClient.getFrontmatter(request);

      return {
        frontmatter: response.frontmatter ?? {},
      };
    } catch (err) {
      const augmentedError = AugmentErrorService.augmentError(err, 'get template frontmatter');
      return {
        frontmatter: {},
        error: augmentedError,
      };
    }
  }

  /**
   * Generates a wiki identifier from text
   * @param text The text to convert to an identifier
   * @param ensureUnique If true, appends suffix to ensure no page exists with this identifier
   * @returns The generated identifier and availability info
   */
  async generateIdentifier(
    text: string,
    ensureUnique = false
  ): Promise<{
    identifier: string;
    isUnique: boolean;
    existingPage?: ExistingPageInfo;
    error?: Error;
  }> {
    if (!text) {
      return { identifier: '', isUnique: true };
    }

    try {
      const request = create(GenerateIdentifierRequestSchema, {
        text,
        ensureUnique,
      });

      const response = await this.pageManagementClient.generateIdentifier(request);

      const result: {
        identifier: string;
        isUnique: boolean;
        existingPage?: ExistingPageInfo;
      } = {
        identifier: response.identifier,
        isUnique: response.isUnique,
      };

      if (response.existingPage) {
        result.existingPage = response.existingPage;
      }

      return result;
    } catch (err) {
      const augmentedError = AugmentErrorService.augmentError(err, 'generate identifier');
      return {
        identifier: '',
        isUnique: true,
        error: augmentedError,
      };
    }
  }

  /**
   * Creates a new wiki page
   * @param identifier The page identifier
   * @param contentMarkdown Optional markdown content (uses default template if not provided)
   * @param template Optional template identifier to apply
   * @param frontmatter Optional structured frontmatter to merge with template
   * @returns Result with success status
   */
  async createPage(
    identifier: string,
    contentMarkdown = '',
    template?: string,
    frontmatter?: JsonObject
  ): Promise<{
    success: boolean;
    error?: Error;
  }> {
    if (!identifier) {
      return { success: false, error: new Error('Identifier is required') };
    }

    try {
      const request = create(CreatePageRequestSchema, {
        pageName: identifier,
        contentMarkdown,
        ...(frontmatter && { frontmatter }),
      });

      // Set template if provided
      if (template) {
        request.template = template;
      }

      const response = await this.pageManagementClient.createPage(request);

      if (response.success) {
        return { success: true };
      } else {
        return {
          success: false,
          error: new Error(response.error || 'Failed to create page'),
        };
      }
    } catch (err) {
      const augmentedError = AugmentErrorService.augmentError(err, 'create page');
      return {
        success: false,
        error: augmentedError,
      };
    }
  }

  /**
   * Shows a success toast message
   */
  showSuccess(message: string, callback?: () => void) {
    if (callback) {
      showToastAfter(message, 'success', SUCCESS_TOAST_DURATION_SECONDS, callback);
    } else {
      showToast(message, 'success', SUCCESS_TOAST_DURATION_SECONDS);
    }
  }

  /**
   * Shows an error toast message
   */
  showError(message: string) {
    showToast(message, 'error', ERROR_TOAST_DURATION_SECONDS);
  }
}
