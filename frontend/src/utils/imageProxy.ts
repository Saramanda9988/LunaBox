const IMAGE_PROXY_PATH = "/proxy/image";

export function shouldProxyImageSrc(
  src: string | null | undefined,
): src is string {
  const value = src?.trim();
  if (!value) {
    return false;
  }

  if (!/^https?:\/\//i.test(value)) {
    return false;
  }

  try {
    const url = new URL(value);
    return url.hostname.toLowerCase() !== "wails.localhost";
  }
  catch {
    return false;
  }
}

export function proxiedImageSrc(src: string | null | undefined): string {
  const value = src?.trim() ?? "";
  if (!shouldProxyImageSrc(value)) {
    return value;
  }

  const params = new URLSearchParams({ url: value });
  return `${IMAGE_PROXY_PATH}?${params.toString()}`;
}
