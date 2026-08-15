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

export function imageSourceCandidates(
  src: string | null | undefined,
  fallbackSrc?: string | null | undefined,
): string[] {
  const sources: string[] = [];
  const addSource = (value: string | null | undefined) => {
    const normalizedValue = value?.trim() ?? "";
    if (!normalizedValue) {
      return;
    }

    const proxyValue = proxiedImageSrc(normalizedValue);
    if (proxyValue && !sources.includes(proxyValue)) {
      sources.push(proxyValue);
    }
    if (normalizedValue !== proxyValue && !sources.includes(normalizedValue)) {
      sources.push(normalizedValue);
    }
  };

  addSource(src);
  addSource(fallbackSrc);
  return sources;
}

export type ImageDimensions = {
  width: number;
  height: number;
};

function loadImageDimensions(src: string): Promise<ImageDimensions> {
  return new Promise((resolve, reject) => {
    const image = new Image();
    image.referrerPolicy = "no-referrer";
    image.onload = () => {
      resolve({
        width: image.naturalWidth,
        height: image.naturalHeight,
      });
    };
    image.onerror = () => reject(new Error(`Failed to load image: ${src}`));
    image.src = src;
  });
}

export async function preloadImageDimensions(
  src: string | null | undefined,
  fallbackSrc?: string | null | undefined,
): Promise<ImageDimensions | null> {
  const candidates = imageSourceCandidates(src, fallbackSrc);
  for (const candidate of candidates) {
    try {
      return await loadImageDimensions(candidate);
    }
    catch {
      // Continue with the same fallback order used by ProxyImage.
    }
  }
  return null;
}
