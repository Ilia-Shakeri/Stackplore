/** JSON string limits count Unicode code points, rather than UTF-16 units. */
export function hasCodePointLength(value: string, maximum: number, minimum = 0) {
  let length = 0;
  let offset = 0;
  while (offset < value.length) {
    if (++length > maximum) return false;
    offset += (value.codePointAt(offset) ?? 0) > 0xffff ? 2 : 1;
  }
  return length >= minimum;
}

export function sliceCodePoints(value: string, maximum: number) {
  let length = 0;
  let offset = 0;
  while (offset < value.length && length < maximum) {
    offset += (value.codePointAt(offset) ?? 0) > 0xffff ? 2 : 1;
    length++;
  }
  return value.slice(0, offset);
}
