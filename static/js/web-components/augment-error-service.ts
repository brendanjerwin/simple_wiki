import { ConnectError, Code } from '@connectrpc/connect';

/**
 * Standard error kinds for categorizing different types of errors
 */
export enum ErrorKind {
  WARNING = 'warning',          // General warnings and errors
  ERROR = 'error',              // Critical errors and failures  
  NETWORK = 'network',          // Network and connectivity errors
  PERMISSION = 'permission',    // Permission and authorization errors
  TIMEOUT = 'timeout',          // Timeout and performance errors
  NOT_FOUND = 'not-found',      // Missing files or resources
  VALIDATION = 'validation',    // Input validation errors
  SERVER = 'server',            // Server and system errors
}

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
 * Icon type can be a standard icon or any custom string (emoji, unicode, etc.)
 */
export type ErrorIcon = StandardErrorIcon | string;

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
  constructor(
    public readonly originalError: Error,
    public readonly errorKind: ErrorKind,
    public readonly icon: ErrorIcon,
    public readonly failedGoalDescription?: string
  ) {
    // Call super with no arguments to avoid auto-generating new stack trace
    super();

    // Remove the auto-generated stack property so our getter can work
    delete (this as Error).stack;
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
  get cause(): unknown {
    // Use 'in' operator for type-safe property access on the originalError
    if ('cause' in this.originalError) {
      return this.originalError.cause;
    }
    return undefined;
  }
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
   * Get icon string for an ErrorIcon (resolves standard icons to emojis)
   */
  static getIconString(icon: ErrorIcon): string {
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
      // Try to stringify the object to preserve information
      try {
        message = JSON.stringify(error);
      } catch {
        // If JSON.stringify fails, use toString or fallback
        message = error.toString?.() || 'An unknown error occurred';
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
