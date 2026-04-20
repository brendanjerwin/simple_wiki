import { css } from 'lit';

export const wikiSurveyStyles = css`
  :host {
    display: block;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto',
      'Oxygen', 'Ubuntu', 'Cantarell', sans-serif;
    color: var(--color-text-primary);
  }

  .survey-container {
    border: 1px solid var(--color-border-subtle);
    border-radius: 8px;
    background: var(--color-surface-primary);
    padding: 16px;
    max-width: 600px;
  }

  .survey-question {
    font-size: 18px;
    font-weight: 600;
    margin: 0 0 16px 0;
    color: var(--color-text-primary);
  }

  .survey-fields {
    display: flex;
    flex-direction: column;
    gap: 12px;
    margin-bottom: 16px;
  }

  .field-group {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .field-label {
    font-size: 14px;
    font-weight: 500;
    color: var(--color-text-secondary);
    text-transform: capitalize;
  }

  .required-indicator {
    color: var(--color-error, #dc3545);
    margin-left: 2px;
  }

  .field-input {
    padding: 8px 10px;
    border: 1px solid var(--color-border-default);
    border-radius: 4px;
    background: var(--color-surface-primary);
    color: var(--color-text-primary);
    font-size: 14px;
    width: 100%;
    box-sizing: border-box;
  }

  .field-input:focus {
    outline: none;
    border-color: var(--color-accent-primary);
  }

  select.field-input {
    cursor: pointer;
  }

  .checkbox-group {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .checkbox-group input[type='checkbox'] {
    width: 16px;
    height: 16px;
    cursor: pointer;
  }

  .submit-row {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-top: 4px;
  }

  .submit-btn {
    padding: 8px 16px;
    background: var(--color-accent-primary);
    color: var(--color-text-on-accent);
    border: none;
    border-radius: 4px;
    font-size: 14px;
    font-weight: 500;
    cursor: pointer;
  }

  .submit-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .submit-btn:hover:not(:disabled) {
    opacity: 0.85;
  }

  .saving-indicator {
    font-size: 13px;
    color: var(--color-text-secondary);
  }

  .success-message {
    font-size: 13px;
    color: var(--color-success);
  }

  .loading {
    color: var(--color-text-secondary);
    font-size: 14px;
    padding: 8px 0;
  }

  .not-configured {
    color: var(--color-text-secondary);
    font-size: 14px;
    font-style: italic;
  }

  .login-required {
    color: var(--color-text-secondary);
    font-size: 14px;
  }

  .closed-notice {
    color: var(--color-text-secondary);
    font-size: 13px;
    font-style: italic;
    margin-top: 8px;
  }

  .responses-section {
    margin-top: 20px;
    padding-top: 16px;
    border-top: 1px solid var(--color-border-subtle);
  }

  .responses-title {
    font-size: 14px;
    font-weight: 600;
    color: var(--color-text-secondary);
    margin: 0 0 8px 0;
  }

  .response-item {
    font-size: 13px;
    color: var(--color-text-secondary);
    padding: 4px 0;
    border-bottom: 1px solid var(--color-border-subtle);
  }

  .response-user {
    font-weight: 500;
    color: var(--color-text-primary);
  }

  .response-date {
    color: var(--color-text-tertiary);
    font-size: 12px;
  }

  .response-values {
    margin-top: 2px;
  }

  .error-wrapper {
    margin-top: 8px;
  }
`;
