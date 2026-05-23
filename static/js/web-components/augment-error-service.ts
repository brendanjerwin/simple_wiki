import { ConnectError, Code } from '@connectrpc/connect';

/**
 * Coerces an unknown value from a third-party library boundary into an Error object.
 *
 * Use this at catch boundaries where the error source is external code (gRPC clients,
 * fetch, external libraries) that may not always throw proper Error objects.
 *
 * For internal code, prefer throwing Error objects directly rather than relying on coercion.
 *
 * @param err - The caught value (may be Error, string, or any other type)
 * @param errorContext - Description of where/what failed (e.g., "Search failed", "fetching page")
 * @returns An Error object
 */
export function coerceThirdPartyError(err: unknown, errorContext: string): Error {
  if (err instanceof Error) {
    return err;
  }
  if (typeof err === 'string') {
    return new Error(err);
  }
  if (err !== null && err !== undefined) {
    if (typeof err === 'symbol' || typeof err === 'function' ||
        typeof err === 'number' || typeof err === 'boolean' || typeof err === 'bigint') {
      return new Error(err.toString());
    }
    try {
      return new Error(JSON.stringify(err));
    } catch {
      return new Error(Object.prototype.toString.call(err));
    }
  }
  return new Error(errorContext);
}

/**
 * Standard error kinds for categorizing different types of errors
 */
export const ErrorKind = {
  WARNING: 'warning',           // General warnings and errors
  ERROR: 'error',               // Critical errors and failures
  NETWORK: 'network',           // Network and connectivity errors
  PERMISSION: 'permission',     // Permission and authorization errors
  TIMEOUT: 'timeout',           // Timeout and performance errors
  NOT_FOUND: 'not-found',       // Missing files or resources
  VALIDATION: 'validation',     // Input validation errors
  SERVER: 'server',             // Server and system errors
} as const;
export type ErrorKind = typeof ErrorKind[keyof typeof ErrorKind];

/**
 * Standard error icons for common error types
 */
export type StandardErrorIcon =
  | 'warning'      // ⚠️ - General warnings and errors
  | 'error'        // ❌ - Critical errors and failures  
  | 'network'      // 🌐 - Network and connectivity errors
  | 'permission'   // 🔒 - Permission and authorization errors
  | 'timeout'      // ⏱️ - Timeout and performance errors
  | 'not-found'    // 📄 - Missing files or resources
  | 'validation'   // ✏️ - Input validation errors
  | 'server'       // 🚨 - Server and system errors
  ;

/**
 * Map of standard icons to their emoji representations
 */
const STANDARD_ICONS: Record<StandardErrorIcon, string> = {
  'warning': '⚠️',
  'error': '❌',
  'network': '🌐',
  'permission': '🔒',
  'timeout': '⏱️',
  'not-found': '📄',
  'validation': '✏️',
  'server': '🚨',
};

/**
 * Type guard to check if a string is a StandardErrorIcon
 */
function isStandardErrorIcon(icon: string): icon is StandardErrorIcon {
  return icon in STANDARD_ICONS;
}

/**
 * Map error kinds to their corresponding icons
 */
const ERROR_KIND_TO_ICON: Record<ErrorKind, StandardErrorIcon> = {
  [ErrorKind.WARNING]: 'warning',
  [ErrorKind.ERROR]: 'error',
  [ErrorKind.NETWORK]: 'network',
  [ErrorKind.PERMISSION]: 'permission',
  [ErrorKind.TIMEOUT]: 'timeout',
  [ErrorKind.NOT_FOUND]: 'not-found',
  [ErrorKind.VALIDATION]: 'validation',
  [ErrorKind.SERVER]: 'server',
};

/**
 * An Error augmented with additional error handling metadata
 */
export class AugmentedError extends Error {
  public readonly copyableDetails: string;

  constructor(
    public readonly originalError: Error,
    public readonly errorKind: ErrorKind,
    public readonly icon: string,
    public readonly failedGoalDescription?: string
  ) {
    // Call super with no arguments to avoid auto-generating new stack trace
    super();

    // Remove the auto-generated stack property so our getter can work
    delete (this as Error).stack;

    this.copyableDetails = buildCopyableDetails(this);
  }

  // Delegate message to original error
  override get message(): string {
    return this.originalError.message ?? '';
  }

  // Delegate stack to original error
  override get stack(): string {
    return this.originalError.stack ?? '';
  }

  // Delegate name to original error to preserve error type information
  override get name(): string {
    return this.originalError.name;
  }

  // Delegate cause to original error for full transparency
  override get cause(): unknown {
    // Use 'in' operator for type-safe property access on the originalError
    if ('cause' in this.originalError) {
      return this.originalError.cause;
    }
    return undefined;
  }
}

function buildCopyableDetails(error: AugmentedError): string {
  const connectError = error.originalError instanceof ConnectError
    ? error.originalError
    : undefined;
  const metadata = connectError ? formatMetadata(connectError.metadata) : '(none)';
  const stack = error.stack.trim() || '(none)';

  return [
    '# Error Details',
    '',
    `**Message:** ${fallback(error.message)}`,
    `**Failed goal:** ${fallback(error.failedGoalDescription)}`,
    `**Error kind:** ${error.errorKind}`,
    `**Error name:** ${fallback(error.name)}`,
    `**Connect code:** ${connectError ? Code[connectError.code] : '(none)'}`,
    `**gRPC code:** ${connectError ? connectError.code.toString() : '(none)'}`,
    `**Metadata:** ${metadata === '(none)' ? '(none)' : 'see Metadata section'}`,
    `**User-Agent:** ${readUserAgent()}`,
    `**Current URL:** ${readCurrentURL()}`,
    `**Timestamp:** ${new Date().toISOString()}`,
    `**Wiki version:** ${readWikiVersion()}`,
    '',
    '## Error Chain',
    '',
    formatErrorChain(error.originalError),
    '',
    '## Metadata',
    '',
    metadata,
    '',
    '## Stack Trace',
    '',
    '```',
    stack,
    '```',
    '',
  ].join('\n');
}

function fallback(value: string | undefined): string {
  const trimmed = value?.trim();
  return trimmed ? trimmed : '(none)';
}

function formatErrorChain(error: Error): string {
  const chain: string[] = [];
  let current: unknown = error;

  while (current !== undefined) {
    chain.push(`${chain.length + 1}. ${describeError(current)}`);

    if (current && typeof current === 'object' && 'cause' in current) {
      current = (current as { cause?: unknown }).cause;
    } else {
      current = undefined;
    }
  }

  return chain.length > 0 ? chain.join('\n') : '(none)';
}

function describeError(error: unknown): string {
  if (error instanceof Error) {
    return `${fallback(error.name)}: ${fallback(error.message)}`;
  }

  if (typeof error === 'string') {
    return error.trim() || '(none)';
  }

  if (error === null || error === undefined) {
    return '(none)';
  }

  try {
    return JSON.stringify(error);
  } catch {
    return Object.prototype.toString.call(error);
  }
}

function formatMetadata(metadata: Headers): string {
  const entries = Array.from(metadata.entries());
  if (entries.length === 0) {
    return '(none)';
  }

  return entries.map(([name, value]) => `- ${name}: ${value}`).join('\n');
}

function readUserAgent(): string {
  if (typeof navigator === 'undefined') {
    return '(none)';
  }

  return fallback(navigator.userAgent);
}

function readCurrentURL(): string {
  if (typeof window === 'undefined') {
    return '(none)';
  }

  return fallback(window.location.href);
}

function readWikiVersion(): string {
  if (typeof document === 'undefined') {
    return '(none)';
  }

  const systemInfo = document.querySelector('system-info');
  const version = systemInfo ? Reflect.get(systemInfo, 'version') : undefined;
  if (!version || typeof version !== 'object') {
    return '(none)';
  }

  const commit = stringProperty(version, 'commit');
  if (!commit) {
    return '(none)';
  }

  const buildTime = stringProperty(version, 'buildTime');
  if (!buildTime) {
    return commit;
  }

  return `${commit} (built ${buildTime})`;
}

function stringProperty(source: object, propertyName: string): string {
  const value = Reflect.get(source, propertyName);
  return typeof value === 'string' ? value.trim() : '';
}

/**
 * AugmentErrorService - Augments errors with errorKind and icon metadata
 * 
 * This service focuses solely on augmenting errors with classification metadata
 * rather than modifying error messages. It converts various error types into
 * AugmentedError instances with appropriate errorKind and icon values.
 */
export class AugmentErrorService {
  /**
   * Augment any error with errorKind and icon metadata
   */
  static augmentError(error: unknown, failedGoalDescription?: string): AugmentedError {
    if (error instanceof AugmentedError) {
      return error; // Already augmented
    }

    if (error instanceof ConnectError) {
      return this.augmentConnectError(error, failedGoalDescription);
    }

    if (error instanceof Error) {
      return this.augmentStandardError(error, failedGoalDescription);
    }

    // Handle non-Error objects by creating Error first
    return this.augmentUnknownError(error, failedGoalDescription);
  }

  /**
   * Get the display string for an error icon identifier (resolves standard icons to emojis)
   */
  static getIconString(icon: string): string {
    if (isStandardErrorIcon(icon)) {
      return STANDARD_ICONS[icon];
    }
    return icon;
  }

  /**
   * Augment Connect/gRPC errors using proper error codes
   */
  private static augmentConnectError(error: ConnectError, failedGoalDescription?: string): AugmentedError {
    let errorKind: ErrorKind;

    switch (error.code) {
      case Code.Unavailable:
        errorKind = ErrorKind.NETWORK;
        break;
      case Code.NotFound:
        errorKind = ErrorKind.NOT_FOUND;
        break;
      case Code.PermissionDenied:
      case Code.Unauthenticated:
        errorKind = ErrorKind.PERMISSION;
        break;
      case Code.InvalidArgument:
        errorKind = ErrorKind.VALIDATION;
        break;
      case Code.DeadlineExceeded:
        errorKind = ErrorKind.TIMEOUT;
        break;
      case Code.Internal:
        errorKind = ErrorKind.SERVER;
        break;
      default:
        errorKind = ErrorKind.ERROR;
    }

    const icon = ERROR_KIND_TO_ICON[errorKind];

    return new AugmentedError(
      error,
      errorKind,
      icon,
      failedGoalDescription
    );
  }

  /**
   * Augment standard Error objects
   */
  private static augmentStandardError(error: Error, failedGoalDescription?: string): AugmentedError {
    return new AugmentedError(
      error,
      ErrorKind.ERROR,
      ERROR_KIND_TO_ICON[ErrorKind.ERROR],
      failedGoalDescription
    );
  }

  /**
   * Augment non-Error objects by creating Error first, then augmenting
   */
  private static augmentUnknownError(error: unknown, failedGoalDescription?: string): AugmentedError {
    let message: string;

    if (typeof error === 'string') {
      message = error;
    } else if (error !== null && error !== undefined) {
      if (typeof error === 'symbol' || typeof error === 'function' ||
          typeof error === 'number' || typeof error === 'boolean' || typeof error === 'bigint') {
        message = error.toString();
      } else {
        try {
          message = JSON.stringify(error);
        } catch {
          message = Object.prototype.toString.call(error);
        }
      }
    } else {
      message = 'An unknown error occurred';
    }

    // Create Error first, then augment it
    const createdError = new Error(message);

    return new AugmentedError(
      createdError,
      ErrorKind.ERROR,
      ERROR_KIND_TO_ICON[ErrorKind.ERROR],
      failedGoalDescription
    );
  }
}
