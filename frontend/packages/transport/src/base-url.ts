/**
 * Remove trailing path separators without regex backtracking.
 * An empty string and strings containing only separators normalize to "".
 */
export function stripTrailingSlashes(baseUrl: string): string {
  let end = baseUrl.length;
  while (end > 0 && baseUrl.charCodeAt(end - 1) === 47) {
    end -= 1;
  }
  return baseUrl.slice(0, end);
}
