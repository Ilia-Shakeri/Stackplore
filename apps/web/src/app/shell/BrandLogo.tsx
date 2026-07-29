import { Link } from '@tanstack/react-router';

interface BrandLogoProps {
  compact?: boolean;
}

export function BrandLogo({ compact = false }: BrandLogoProps) {
  return (
    <Link className="brand-logo ui-focus-ring" to="/overview" aria-label="Stackplore overview">
      <img
        className="brand-logo__mark"
        src="/brand/stackplore-mark.svg"
        width="36"
        height="36"
        alt=""
      />
      <span className="brand-logo__word" hidden={compact}>
        Stackplore
      </span>
    </Link>
  );
}
