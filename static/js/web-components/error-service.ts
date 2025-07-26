import { ConnectError, Code } from '@connectrpc/connect';
import type { ErrorIcon } from './error-display.js';

/**
 * Result of processing an error into user-friendly information
 */
export interface ProcessedError {
  message: string;
  details?: string;
  icon: ErrorIcon;
}

/**
 * ErrorService - Centralized error processing and user message generation
 * 
 * This service handles the conversion of gRPC/Connect errors into user-friendly
 * error messages with appropriate icons and details. By centralizing error handling,
 * we ensure consistent error presentation across the application and avoid
 * business logic in UI components.
 */
export class ErrorService {
  /**
   * Process any error into a user-friendly format
   */
  static processError(error: unknown, context: string = 'operation'): ProcessedError {
    if (error instanceof ConnectError) {
      return this.processConnectError(error, context);
    }
    
    if (error instanceof Error) {
      return {
        message: error.message || `Failed to ${context}`,
        details: error.stack || error.toString(),
        icon: 'error'
      };
    }
    
    // Handle non-Error objects
    return {
      message: `Failed to ${context}`,
      details: typeof error === 'string' ? error : JSON.stringify(error),
      icon: 'error'
    };
  }

  /**
   * Process Connect/gRPC errors using proper error codes
   */
  private static processConnectError(error: ConnectError, context: string): ProcessedError {
    const baseDetails = `gRPC error: ${error.message}`;
    
    switch (error.code) {
      case Code.Unavailable:
        return {
          message: 'Unable to connect to server',
          details: `${baseDetails}\n\nThe server may be down or unreachable. Please check your network connection and try again.`,
          icon: 'network'
        };
        
      case Code.NotFound:
        return {
          message: this.getNotFoundMessage(context),
          details: `${baseDetails}\n\nThe requested resource may not exist or you may not have permission to access it.`,
          icon: 'not-found'
        };
        
      case Code.PermissionDenied:
        return {
          message: 'Access denied',
          details: `${baseDetails}\n\nYou do not have permission to perform this action.`,
          icon: 'permission'
        };
        
      case Code.InvalidArgument:
        return {
          message: 'Invalid request',
          details: `${baseDetails}\n\nThe request contains invalid data. Please check your input and try again.`,
          icon: 'validation'
        };
        
      case Code.DeadlineExceeded:
        return {
          message: 'Request timed out',
          details: `${baseDetails}\n\nThe operation took too long to complete. Please try again.`,
          icon: 'timeout'
        };
        
      case Code.Internal:
        return {
          message: 'Server error',
          details: `${baseDetails}\n\nAn internal server error occurred. Please try again later or contact support.`,
          icon: 'server'
        };
        
      case Code.Unauthenticated:
        return {
          message: 'Authentication required',
          details: `${baseDetails}\n\nYou must be logged in to perform this action.`,
          icon: 'permission'
        };
        
      default:
        return {
          message: `Failed to ${context}`,
          details: `${baseDetails}\n\nAn unexpected error occurred.`,
          icon: 'error'
        };
    }
  }

  /**
   * Generate context-appropriate "not found" messages
   */
  private static getNotFoundMessage(context: string): string {
    switch (context) {
      case 'load frontmatter':
        return 'Page not found';
      case 'save frontmatter':
        return 'Cannot save to non-existent page';
      default:
        return 'Resource not found';
    }
  }
}