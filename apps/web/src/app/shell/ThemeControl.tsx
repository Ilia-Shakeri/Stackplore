import { Select, SelectGroup, SelectOption, useTheme } from '@stackplore/ui';

export function ThemeControl() {
  const { setTheme, theme } = useTheme();

  return (
    <Select
      className="theme-control"
      aria-label="Theme"
      value={theme}
      onChange={(event) => {
        const value = event.currentTarget.value;
        if (value === 'light' || value === 'dark' || value === 'system') {
          setTheme(value);
        }
      }}
    >
      <SelectGroup label="Theme">
        <SelectOption value="light">Light</SelectOption>
        <SelectOption value="dark">Dark</SelectOption>
        <SelectOption value="system">System</SelectOption>
      </SelectGroup>
    </Select>
  );
}
