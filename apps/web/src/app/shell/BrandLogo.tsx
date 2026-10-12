import { Link } from '@tanstack/react-router';

import { useLocale } from '../../i18n';
import { useProductSource } from '../../features/product/product-context';

interface BrandLogoProps {
  compact?: boolean;
}

export function BrandLogo({ compact = false }: BrandLogoProps) {
  const { t } = useLocale();
  const { selectedSource } = useProductSource();

  return (
    <Link
      className="brand-logo ui-focus-ring"
      to="/overview"
      search={selectedSource === undefined ? {} : { source: selectedSource }}
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
