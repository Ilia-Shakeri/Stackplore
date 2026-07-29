export type ClassValue = string | false | null | undefined;

export function cn(...values: ClassValue[]) {
  return values
    .filter((value): value is string => typeof value === 'string' && value.length > 0)
    .join(' ');
}
