import type { components } from './schema';

type ErrorBody = components['schemas']['Error'];

/** An error answer from the API: `{"error": {"code", "message", "details"}}`. */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly details?: Record<string, string[]>;

  constructor(status: number, code: string, message: string, details?: Record<string, string[]>) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

function readBody(error: unknown): ErrorBody['error'] | null {
  if (typeof error === 'object' && error !== null && 'error' in error) {
    const inner = (error as ErrorBody).error;
    if (inner && typeof inner.message === 'string') {
      return inner;
    }
  }
  return null;
}

/** Builds an ApiError from the body openapi-fetch returned (may be text or empty). */
export function toApiError(error: unknown, response: Response): ApiError {
  const body = readBody(error);
  return new ApiError(
    response.status,
    body?.code ?? 'internal',
    body?.message ?? `Ошибка сервера (${response.status})`,
    body?.details,
  );
}

/** Returns `data` of an openapi-fetch call, or throws ApiError. */
export async function unwrap<T>(
  request: Promise<{ data?: T; error?: unknown; response: Response }>,
): Promise<T> {
  const { data, error, response } = await request;
  if (error !== undefined || data === undefined) {
    throw toApiError(error, response);
  }
  return data;
}
