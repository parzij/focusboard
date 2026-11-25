type ErrorBody = {
  error?: { code?: string; message?: string; fields?: Record<string, string> };
};

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
    readonly fields: Record<string, string> = {},
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export type RequestOptions = {
  method?: string;
  json?: unknown;
  signal?: AbortSignal;
};

export async function request<T>(
  path: `/${string}`,
  options: RequestOptions = {},
  access?: string,
): Promise<T> {
  const headers = new Headers({ Accept: "application/json" });
  if (options.json !== undefined)
    headers.set("Content-Type", "application/json");
  if (access) headers.set("Authorization", `Bearer ${access}`);

  let response: Response;
  try {
    response = await fetch(`/api/v1${path}`, {
      method: options.method ?? "GET",
      body: JSON.stringify(options.json),
      signal: options.signal,
      credentials: "same-origin",
      headers,
    });
  } catch (error) {
    if (options.signal?.aborted) throw error;
    throw new ApiError(
      0,
      "NETWORK_ERROR",
      "Could not connect. Check your connection and try again.",
    );
  }

  if (response.status === 204) return undefined as T;

  const data = await response.json().catch(() => null);
  options.signal?.throwIfAborted();
  if (!response.ok) {
    const error = (data as ErrorBody | null)?.error;
    throw new ApiError(
      response.status,
      error?.code ?? "HTTP_ERROR",
      error?.message ?? "Something went wrong. Try again.",
      error?.fields,
    );
  }
  if (data === null)
    throw new ApiError(
      response.status,
      "INVALID_RESPONSE",
      "The server returned an unexpected response.",
    );
  return data as T;
}
