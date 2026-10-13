import type { SourceSettingsFailure } from './source-settings-api';
import type { TextMessageKey } from '../../i18n';

export function sourceSettingsFailureKey(failure: SourceSettingsFailure): TextMessageKey {
  switch (failure) {
    case 'stale':
      return 'connections.error.stale';
    case 'confirmation':
      return 'connections.error.confirmation';
    case 'profile':
      return 'connections.error.profile';
    case 'denied':
      return 'connections.error.denied';
    case 'session':
      return 'connections.error.session';
    case 'rate_limited':
      return 'connections.error.rateLimited';
    case 'unavailable':
      return 'connections.error.unavailable';
    case 'uncertain':
      return 'connections.error.uncertain';
    case 'invalid':
      return 'connections.error.invalid';
    default:
      return 'connections.error.unexpected';
  }
}
