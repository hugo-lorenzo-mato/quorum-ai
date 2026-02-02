/**
 * Contract Validator
 *
 * Validates API responses against JSON schemas to ensure
 * frontend and backend stay in sync.
 */

import Ajv from 'ajv';

const ajv = new Ajv({
  allErrors: true,
  strict: false, // Allow additional properties in schemas
  validateFormats: false, // Don't validate formats like date-time
});

/**
 * Validates data against a JSON schema.
 * @param {Object} schema - JSON Schema to validate against
 * @param {*} data - Data to validate
 * @returns {{ valid: boolean, errors: Array|null }}
 */
export function validateSchema(schema, data) {
  const validate = ajv.compile(schema);
  const valid = validate(data);

  return {
    valid,
    errors: valid ? null : validate.errors,
  };
}

/**
 * Validates data and throws if invalid.
 * @param {Object} schema - JSON Schema to validate against
 * @param {*} data - Data to validate
 * @param {string} [context] - Context for error message
 * @throws {Error} If validation fails
 */
export function assertSchema(schema, data, context = 'Response') {
  const { valid, errors } = validateSchema(schema, data);

  if (!valid) {
    const errorMessages = errors
      .map((e) => `${e.instancePath || '(root)'}: ${e.message}`)
      .join('; ');
    throw new Error(`${context} contract violation: ${errorMessages}`);
  }

  return data;
}

/**
 * Creates a validated fetch wrapper.
 * @param {Object} schema - JSON Schema to validate responses against
 * @returns {Function} Fetch function that validates responses
 */
export function createValidatedFetch(schema) {
  return async (url, options) => {
    const response = await fetch(url, options);

    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      throw new Error(error.message || error.error || response.statusText);
    }

    if (response.status === 204) {
      return null;
    }

    const data = await response.json();
    assertSchema(schema, data, `${options?.method || 'GET'} ${url}`);
    return data;
  };
}

/**
 * Formats validation errors for display.
 * @param {Array} errors - AJV validation errors
 * @returns {string} Formatted error message
 */
export function formatValidationErrors(errors) {
  if (!errors || errors.length === 0) {
    return 'No errors';
  }

  return errors
    .map((e) => {
      const path = e.instancePath || '(root)';
      const message = e.message;
      const params = e.params ? JSON.stringify(e.params) : '';
      return `- ${path}: ${message} ${params}`.trim();
    })
    .join('\n');
}

export default {
  validateSchema,
  assertSchema,
  createValidatedFetch,
  formatValidationErrors,
};
