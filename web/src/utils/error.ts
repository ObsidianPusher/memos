import { ClientError } from "nice-grpc-web";

export const getErrorMessage = (error: unknown, fallback = "Unknown error") => {
  if (error instanceof ClientError) {
    return error.details || error.message || fallback;
  }

  if (error instanceof Error) {
    return error.message || fallback;
  }

  if (typeof error === "string" && error.trim()) {
    return error;
  }

  return fallback;
};
