# worldview API操作映射

> 由build-contracts.cjs生成。字段与响应见源JSON；不是运行完成证明。

| 方法/路径 | operationId | 需求 | 源分片 |
| --- | --- | --- | --- |
| POST /v1/worldview/characters/{character_id}/restore | worldview_restoreCharacter | FR-03 worldview.restoreCharacter | [worldview.json](../worldview.json) |
| DELETE /v1/worldview/characters/{character_id} | worldview_deleteCharacter | FR-10 worldview.deleteCharacter | [worldview.json](../worldview.json) |
| GET /v1/worldview/characters/{character_id} | worldview_getCharacter | FR-10 worldview.getCharacter | [worldview.json](../worldview.json) |
| PUT /v1/worldview/characters/{character_id} | worldview_updateCharacter | FR-10 worldview.updateCharacter | [worldview.json](../worldview.json) |
| POST /v1/worldview/facts/{fact_id}/retract | worldview_retractFact | FR-11 worldview.retractFact | [worldview-facts.json](../worldview-facts.json) |
| POST /v1/worldview/facts/{fact_id}/supersede | worldview_supersedeFact | FR-11 worldview.supersedeFact | [worldview-facts.json](../worldview-facts.json) |
| POST /v1/worldview/lore/{lore_id}/restore | worldview_restoreLore | FR-03 worldview.restoreLore | [worldview.json](../worldview.json) |
| DELETE /v1/worldview/lore/{lore_id} | worldview_deleteLore | FR-10 worldview.deleteLore | [worldview.json](../worldview.json) |
| GET /v1/worldview/lore/{lore_id} | worldview_getLore | FR-10 worldview.getLore | [worldview.json](../worldview.json) |
| PUT /v1/worldview/lore/{lore_id} | worldview_updateLore | FR-10 worldview.updateLore | [worldview.json](../worldview.json) |
| GET /v1/worldview/novels/{novel_id}/characters | worldview_listcharacters | FR-10 worldview.listcharacters | [worldview.json](../worldview.json) |
| POST /v1/worldview/novels/{novel_id}/characters | worldview_createCharacter | FR-10 worldview.createCharacter | [worldview.json](../worldview.json) |
| GET /v1/worldview/novels/{novel_id}/fact-events | worldview_listFactEvents | FR-11 worldview.listFactEvents | [worldview-facts.json](../worldview-facts.json) |
| POST /v1/worldview/novels/{novel_id}/facts/check | worldview_checkFacts | FR-11 worldview.checkFacts | [worldview-facts.json](../worldview-facts.json) |
| GET /v1/worldview/novels/{novel_id}/facts | worldview_listFacts | FR-11 worldview.listFacts | [worldview-facts.json](../worldview-facts.json) |
| GET /v1/worldview/novels/{novel_id}/graph | worldview_graph | FR-10 worldview.graph | [worldview-facts.json](../worldview-facts.json) |
| GET /v1/worldview/novels/{novel_id}/lore | worldview_listlore | FR-10 worldview.listlore | [worldview.json](../worldview.json) |
| POST /v1/worldview/novels/{novel_id}/lore | worldview_createLore | FR-10 worldview.createLore | [worldview.json](../worldview.json) |
| GET /v1/worldview/novels/{novel_id}/relations | worldview_listrelations | FR-10 worldview.listrelations | [worldview-relations.json](../worldview-relations.json) |
| POST /v1/worldview/novels/{novel_id}/relations | worldview_createRelation | FR-10 worldview.createRelation | [worldview-relations.json](../worldview-relations.json) |
| GET /v1/worldview/novels/{novel_id}/world-revision | worldview_getWorldRevision | FR-11 worldview.getWorldRevision | [worldview-facts.json](../worldview-facts.json) |
| POST /v1/worldview/relations/{relation_id}/restore | worldview_restoreRelation | FR-03 worldview.restoreRelation | [worldview-relations.json](../worldview-relations.json) |
| DELETE /v1/worldview/relations/{relation_id} | worldview_deleteRelation | FR-10 worldview.deleteRelation | [worldview-relations.json](../worldview-relations.json) |
| GET /v1/worldview/relations/{relation_id} | worldview_getRelation | FR-10 worldview.getRelation | [worldview-relations.json](../worldview-relations.json) |
| PUT /v1/worldview/relations/{relation_id} | worldview_updateRelation | FR-10 worldview.updateRelation | [worldview-relations.json](../worldview-relations.json) |
