// Minimal type declarations for Firefox WebExtension APIs used by this extension.

declare namespace browser {
  namespace runtime {
    interface MessageSender {
      tab?: { id?: number; url?: string };
      frameId?: number;
      id?: string;
      url?: string;
    }

    interface MessageEvent {
      addListener(
        callback: (
          message: unknown,
          sender: MessageSender,
          sendResponse: (response: unknown) => void
        ) => true | undefined
      ): void;
    }

    function sendMessage(message: unknown): Promise<unknown>;

    const onMessage: MessageEvent;
  }

  namespace browserAction {
    function setBadgeText(details: { text: string; tabId?: number }): Promise<void>;
    function setBadgeBackgroundColor(details: { color: string; tabId?: number }): Promise<void>;
  }

  namespace storage {
    namespace local {
      function get(keys?: string | string[]): Promise<Record<string, unknown>>;
      function set(items: Record<string, unknown>): Promise<void>;
    }
  }
}
