/**
 * Result of a text formatting operation.
 */
export interface FormattingResult {
  /** The complete text after formatting */
  newText: string;
  /** New selection start position */
  newSelectionStart: number;
  /** New selection end position */
  newSelectionEnd: number;
}

/**
 * TextFormattingService provides methods for applying markdown formatting to text.
 */
export class TextFormattingService {
  /**
   * Wraps the selected text in bold markers (**).
   * If no text is selected, inserts **bold** placeholder.
   */
  wrapBold(text: string, selectionStart: number, selectionEnd: number): FormattingResult {
    return this.wrapWithMarkers(text, selectionStart, selectionEnd, '**', '**', 'bold');
  }

  /**
   * Wraps the selected text in italic markers (*).
   * If no text is selected, inserts *italic* placeholder.
   */
  wrapItalic(text: string, selectionStart: number, selectionEnd: number): FormattingResult {
    return this.wrapWithMarkers(text, selectionStart, selectionEnd, '*', '*', 'italic');
  }

  /**
   * Wraps the selected text in a markdown link.
   * If no text is selected, inserts [link text](url) placeholder.
   * @param url Optional URL to use; if not provided, uses 'url' placeholder
   */
  insertLink(text: string, selectionStart: number, selectionEnd: number, url?: string): FormattingResult {
    const before = text.substring(0, selectionStart);
    const selected = text.substring(selectionStart, selectionEnd);
    const after = text.substring(selectionEnd);
    const urlToUse = url || 'url';

    if (selectionStart === selectionEnd) {
      // No selection - insert placeholder
      const placeholder = 'link text';
      const newText = `${before}[${placeholder}](${urlToUse})${after}`;
      return {
        newText,
        newSelectionStart: selectionStart + 1, // After [
        newSelectionEnd: selectionStart + 1 + placeholder.length, // Before ]
      };
    }

    // Has selection - wrap it
    const newText = `${before}[${selected}](${urlToUse})${after}`;

    if (url) {
      // URL provided - position cursor after the link
      const linkEnd = selectionStart + 1 + selected.length + 2 + urlToUse.length + 1;
      return {
        newText,
        newSelectionStart: linkEnd,
        newSelectionEnd: linkEnd,
      };
    }

    // No URL - select the url placeholder
    const urlStart = selectionStart + 1 + selected.length + 2; // After ](
    return {
      newText,
      newSelectionStart: urlStart,
      newSelectionEnd: urlStart + urlToUse.length,
    };
  }

  private wrapWithMarkers(
    text: string,
    selectionStart: number,
    selectionEnd: number,
    openMarker: string,
    closeMarker: string,
    placeholder: string
  ): FormattingResult {
    const before = text.substring(0, selectionStart);
    const selected = text.substring(selectionStart, selectionEnd);
    const after = text.substring(selectionEnd);

    if (selectionStart === selectionEnd) {
      // No selection - insert placeholder
      const newText = `${before}${openMarker}${placeholder}${closeMarker}${after}`;
      return {
        newText,
        newSelectionStart: selectionStart + openMarker.length,
        newSelectionEnd: selectionStart + openMarker.length + placeholder.length,
      };
    }

    // Has selection - wrap it
    const newText = `${before}${openMarker}${selected}${closeMarker}${after}`;
    return {
      newText,
      newSelectionStart: selectionStart + openMarker.length,
      newSelectionEnd: selectionStart + openMarker.length + selected.length,
    };
  }
}

// Export a singleton instance for convenience
export const textFormattingService = new TextFormattingService();
