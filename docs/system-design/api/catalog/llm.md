# llm API操作映射

> 由build-contracts.cjs生成。字段与响应见源JSON；不是运行完成证明。

| 方法/路径 | operationId | 需求 | 源分片 |
| --- | --- | --- | --- |
| GET /v1/llm/budget | llm_get_budget | FR-22 llm.get_budget | [llm-policy.json](../llm-policy.json) |
| PUT /v1/llm/budget | llm_configure_budget | FR-22 llm.configure_budget | [llm-policy.json](../llm-policy.json) |
| DELETE /v1/llm/channels/{channel_id} | llm_delete_channel | FR-21 llm.delete_channel | [llm-admin.json](../llm-admin.json) |
| GET /v1/llm/channels/{channel_id} | llm_get_channel | FR-21 llm.get_channel | [llm-admin.json](../llm-admin.json) |
| PUT /v1/llm/channels/{channel_id} | llm_update_channel | FR-21 llm.update_channel | [llm-admin.json](../llm-admin.json) |
| GET /v1/llm/channels | llm_list_channels | FR-21 llm.list_channels | [llm-admin.json](../llm-admin.json) |
| POST /v1/llm/channels | llm_create_channel | FR-21 llm.create_channel | [llm-admin.json](../llm-admin.json) |
| DELETE /v1/llm/keys/{key_id} | llm_delete_key | FR-21 llm.delete_key | [llm-admin.json](../llm-admin.json) |
| GET /v1/llm/keys/{key_id} | llm_get_key | FR-21 llm.get_key | [llm-admin.json](../llm-admin.json) |
| PUT /v1/llm/keys/{key_id} | llm_update_key | FR-21 llm.update_key | [llm-admin.json](../llm-admin.json) |
| GET /v1/llm/keys | llm_list_keys | FR-21 llm.list_keys | [llm-admin.json](../llm-admin.json) |
| POST /v1/llm/keys | llm_create_key | FR-21 llm.create_key | [llm-admin.json](../llm-admin.json) |
| DELETE /v1/llm/models/{model_id} | llm_delete_model | FR-21 llm.delete_model | [llm-admin.json](../llm-admin.json) |
| GET /v1/llm/models/{model_id} | llm_get_model | FR-21 llm.get_model | [llm-admin.json](../llm-admin.json) |
| PUT /v1/llm/models/{model_id} | llm_update_model | FR-21 llm.update_model | [llm-admin.json](../llm-admin.json) |
| GET /v1/llm/models | llm_list_models | FR-21 llm.list_models | [llm-admin.json](../llm-admin.json) |
| POST /v1/llm/models | llm_create_model | FR-21 llm.create_model | [llm-admin.json](../llm-admin.json) |
| GET /v1/llm/pricing/{pricing_id} | llm_get_pricing | FR-22 llm.get_pricing | [llm-policy.json](../llm-policy.json) |
| GET /v1/llm/pricing | llm_list_pricing | FR-22 llm.list_pricing | [llm-policy.json](../llm-policy.json) |
| POST /v1/llm/pricing | llm_create_pricing | FR-22 llm.create_pricing | [llm-policy.json](../llm-policy.json) |
| DELETE /v1/llm/providers/{provider_id} | llm_delete_provider | FR-21 llm.delete_provider | [llm-admin.json](../llm-admin.json) |
| GET /v1/llm/providers/{provider_id} | llm_get_provider | FR-21 llm.get_provider | [llm-admin.json](../llm-admin.json) |
| PUT /v1/llm/providers/{provider_id} | llm_update_provider | FR-21 llm.update_provider | [llm-admin.json](../llm-admin.json) |
| GET /v1/llm/providers | llm_list_providers | FR-21 llm.list_providers | [llm-admin.json](../llm-admin.json) |
| POST /v1/llm/providers | llm_create_provider | FR-21 llm.create_provider | [llm-admin.json](../llm-admin.json) |
| DELETE /v1/llm/task-policies/{policy_id} | llm_delete_policy | FR-22 llm.delete_policy | [llm-policy.json](../llm-policy.json) |
| GET /v1/llm/task-policies/{policy_id} | llm_get_policy | FR-22 llm.get_policy | [llm-policy.json](../llm-policy.json) |
| PUT /v1/llm/task-policies/{policy_id} | llm_publish_policy | FR-22 llm.publish_policy | [llm-policy.json](../llm-policy.json) |
| GET /v1/llm/task-policies | llm_list_policies | FR-22 llm.list_policies | [llm-policy.json](../llm-policy.json) |
| POST /v1/llm/task-policies | llm_create_policy | FR-22 llm.create_policy | [llm-policy.json](../llm-policy.json) |
| POST /v1/llm/upstream-models | llm_fetch_upstream_models | FR-21 llm.fetch_upstream_models | [llm-admin.json](../llm-admin.json) |
| GET /v1/llm/usage/{usage_id} | llm_get_usage | FR-22 llm.get_usage | [llm-policy.json](../llm-policy.json) |
| GET /v1/llm/usage | llm_list_usage | FR-22 llm.list_usage | [llm-policy.json](../llm-policy.json) |
