import { Link } from '@tanstack/react-router';

import { useLocale } from '../../i18n';

interface BrandLogoProps {
  compact?: boolean;
}

export function BrandLogo({ compact = false }: BrandLogoProps) {
  const { t } = useLocale();

  return (
    <Link
      className="brand-logo ui-focus-ring"
      to="/overview"
      aria-label={t('shell.brand.overview')}
    >
      <img
        className="brand-logo__mark"
        src="/brand/stackplore-mark.svg"
        width="36"
        height="36"
        alt=""
      />
      <bdi className="brand-logo__word" dir="ltr" lang="en" hidden={compact}>
        Stackplore
      </bdi>
    </Link>
  );
}
