export function readFormText(form: FormData, name: string): string {
  const value = form.get(name);
  return typeof value === 'string' ? value : '';
}

export function clearPasswordFields(form: HTMLFormElement) {
  for (const input of form.querySelectorAll<HTMLInputElement>('input[type="password"]')) {
    input.value = '';
  }
}

export function isValidPasswordText(value: string) {
  const bytes = new TextEncoder().encode(value).byteLength;
  return bytes >= 12 && bytes <= 128;
}
