export function compareNullableDateLike(
  leftValue: unknown,
  rightValue: unknown,
) {
  const left = String(leftValue || "");
  const right = String(rightValue || "");

  if (!left && !right) {
    return 0;
  }
  if (!left) {
    return -1;
  }
  if (!right) {
    return 1;
  }

  return left.localeCompare(right);
}
