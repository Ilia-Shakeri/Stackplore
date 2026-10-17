ALTER TABLE infrastructure_sources DROP CONSTRAINT infrastructure_sources_operator_profile_check;
ALTER TABLE infrastructure_sources ADD CONSTRAINT infrastructure_sources_operator_profile_check CHECK (
    (provider='demo' AND operator_profile_id='demo_fixture')
    OR (provider='docker' AND operator_profile_id IN ('docker_socket_dev','docker_restricted_proxy'))
    OR (provider='kubernetes' AND operator_profile_id IN ('kubernetes_incluster','kubernetes_kubeconfig'))
);
