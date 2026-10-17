import type { ApiClient } from '@stackplore/api-client';

import { createActivityApi } from './product-api-activity';
import { createAlertApi } from './product-api-alerts';
import { createInventoryApi } from './product-api-inventory';
import { createObservationApi } from './product-api-observations';
import type { ProductApi, SessionActionRunner } from './product-api-types';
import { createSourceSettingsApi } from './source-settings-api';
import { createOperationApi } from './operation-api';
import { createApplicationSettingsApi } from '../settings/settings-api';

export type * from './product-api-types';

export function createProductApi(
  client: ApiClient,
  sessionActions?: SessionActionRunner,
): ProductApi {
  return {
    ...createApplicationSettingsApi(client, sessionActions),
    ...createOperationApi(client, sessionActions),
    ...createSourceSettingsApi(client, sessionActions),
    ...createInventoryApi(client),
    ...createObservationApi(client),
    ...createAlertApi(client, sessionActions),
    ...createActivityApi(client, sessionActions),
  };
}
