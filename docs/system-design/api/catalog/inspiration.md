# inspiration API操作映射

> 由build-contracts.cjs生成。字段与响应见源JSON；不是运行完成证明。

| 方法/路径 | operationId | 需求 | 源分片 |
| --- | --- | --- | --- |
| POST /v1/inspiration/adoptions/{adoption_id}/revoke | inspiration_revoke_adoption | FR-14 inspiration.revoke_adoption | [inspiration-library.json](../inspiration-library.json) |
| GET /v1/inspiration/analyses/{analysis_id}/report | inspiration_get_report | FR-13 inspiration.get_report | [inspiration-queries.json](../inspiration-queries.json) |
| GET /v1/inspiration/analyses/{analysis_id} | inspiration_get_analysis | FR-13 inspiration.get_analysis | [inspiration-queries.json](../inspiration-queries.json) |
| GET /v1/inspiration/analyses | inspiration_list_analyses | FR-13 inspiration.list_analyses | [inspiration-queries.json](../inspiration-queries.json) |
| POST /v1/inspiration/analyses | inspiration_create_analysis | FR-13 inspiration.create_analysis | [inspiration-commands.json](../inspiration-commands.json) |
| POST /v1/inspiration/assets/{asset_id}/restore | inspiration_restore_asset | FR-03 inspiration.restore_asset | [inspiration-commands.json](../inspiration-commands.json) |
| GET /v1/inspiration/assets/{asset_id}/segments | inspiration_list_segments | FR-13 inspiration.list_segments | [inspiration-queries.json](../inspiration-queries.json) |
| DELETE /v1/inspiration/assets/{asset_id} | inspiration_delete_asset | FR-03 inspiration.delete_asset | [inspiration-commands.json](../inspiration-commands.json) |
| GET /v1/inspiration/assets/{asset_id} | inspiration_get_asset | FR-13 inspiration.get_asset | [inspiration-queries.json](../inspiration-queries.json) |
| PUT /v1/inspiration/assets/{asset_id} | inspiration_update_asset | FR-13 inspiration.update_asset | [inspiration-commands.json](../inspiration-commands.json) |
| GET /v1/inspiration/assets | inspiration_list_assets | FR-13 inspiration.list_assets | [inspiration-queries.json](../inspiration-queries.json) |
| POST /v1/inspiration/assets | inspiration_create_asset | FR-13 inspiration.create_asset | [inspiration-commands.json](../inspiration-commands.json) |
| GET /v1/inspiration/cards/{card_id}/adoptions | inspiration_list_adoptions | FR-14 inspiration.list_adoptions | [inspiration-queries.json](../inspiration-queries.json) |
| POST /v1/inspiration/cards/{card_id}/adoptions | inspiration_record_adoption | FR-14 inspiration.record_adoption | [inspiration-library.json](../inspiration-library.json) |
| POST /v1/inspiration/cards/{card_id}/restore | inspiration_restore_card | FR-03 inspiration.restore_card | [inspiration-library.json](../inspiration-library.json) |
| POST /v1/inspiration/cards/{card_id}/state | inspiration_set_card_state | FR-14 inspiration.set_card_state | [inspiration-library.json](../inspiration-library.json) |
| DELETE /v1/inspiration/cards/{card_id} | inspiration_delete_card | FR-03 inspiration.delete_card | [inspiration-library.json](../inspiration-library.json) |
| GET /v1/inspiration/cards/{card_id} | inspiration_get_card | FR-14 inspiration.get_card | [inspiration-queries.json](../inspiration-queries.json) |
| PUT /v1/inspiration/cards/{card_id} | inspiration_update_card | FR-14 inspiration.update_card | [inspiration-library.json](../inspiration-library.json) |
| GET /v1/inspiration/cards | inspiration_list_cards | FR-14 inspiration.list_cards | [inspiration-queries.json](../inspiration-queries.json) |
| POST /v1/inspiration/cards | inspiration_create_card | FR-14 inspiration.create_card | [inspiration-library.json](../inspiration-library.json) |
| POST /v1/inspiration/styles/{style_id}/restore | inspiration_restore_style | FR-03 inspiration.restore_style | [inspiration-library.json](../inspiration-library.json) |
| GET /v1/inspiration/styles/{style_id}/versions/{version_id} | inspiration_get_style_version | FR-14 inspiration.get_style_version | [inspiration-queries.json](../inspiration-queries.json) |
| GET /v1/inspiration/styles/{style_id}/versions | inspiration_list_style_versions | FR-14 inspiration.list_style_versions | [inspiration-queries.json](../inspiration-queries.json) |
| DELETE /v1/inspiration/styles/{style_id} | inspiration_delete_style | FR-03 inspiration.delete_style | [inspiration-library.json](../inspiration-library.json) |
| GET /v1/inspiration/styles/{style_id} | inspiration_get_style | FR-14 inspiration.get_style | [inspiration-queries.json](../inspiration-queries.json) |
| PUT /v1/inspiration/styles/{style_id} | inspiration_update_style | FR-14 inspiration.update_style | [inspiration-library.json](../inspiration-library.json) |
| GET /v1/inspiration/styles | inspiration_list_styles | FR-14 inspiration.list_styles | [inspiration-queries.json](../inspiration-queries.json) |
| POST /v1/inspiration/styles | inspiration_create_style | FR-14 inspiration.create_style | [inspiration-library.json](../inspiration-library.json) |
