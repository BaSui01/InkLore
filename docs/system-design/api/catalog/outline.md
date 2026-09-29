# outline API操作映射

> 由build-contracts.cjs生成。字段与响应见源JSON；不是运行完成证明。

| 方法/路径 | operationId | 需求 | 源分片 |
| --- | --- | --- | --- |
| POST /v1/outline/beats/{beat_id}/restore | outline_restoreBeat | FR-03 outline.restoreBeat | [outline-planning.json](../outline-planning.json) |
| DELETE /v1/outline/beats/{beat_id} | outline_deleteBeat | FR-08 outline.deleteBeat | [outline-planning.json](../outline-planning.json) |
| GET /v1/outline/beats/{beat_id} | outline_getBeat | FR-08 outline.getBeat | [outline-planning.json](../outline-planning.json) |
| PUT /v1/outline/beats/{beat_id} | outline_updateBeat | FR-08 outline.updateBeat | [outline-planning.json](../outline-planning.json) |
| GET /v1/outline/clues/{clue_id}/events | outline_listClueEvents | FR-09 outline.listClueEvents | [outline-clues.json](../outline-clues.json) |
| POST /v1/outline/clues/{clue_id}/restore | outline_restoreClue | FR-03 outline.restoreClue | [outline-clues.json](../outline-clues.json) |
| POST /v1/outline/clues/{clue_id}/transition | outline_transitionClue | FR-09 outline.transitionClue | [outline-clues.json](../outline-clues.json) |
| DELETE /v1/outline/clues/{clue_id} | outline_deleteClue | FR-09 outline.deleteClue | [outline-clues.json](../outline-clues.json) |
| GET /v1/outline/clues/{clue_id} | outline_getClue | FR-09 outline.getClue | [outline-clues.json](../outline-clues.json) |
| PUT /v1/outline/clues/{clue_id} | outline_updateClue | FR-09 outline.updateClue | [outline-clues.json](../outline-clues.json) |
| POST /v1/outline/nodes/{node_id}/rebind | outline_rebindNode | FR-08 outline.rebindNode | [outline.json](../outline.json) |
| POST /v1/outline/nodes/{node_id}/reorder | outline_reorder | FR-08 outline.reorder | [outline.json](../outline.json) |
| POST /v1/outline/nodes/{node_id}/restore | outline_restoreNode | FR-03 outline.restoreNode | [outline.json](../outline.json) |
| DELETE /v1/outline/nodes/{node_id} | outline_deleteNode | FR-08 outline.deleteNode | [outline.json](../outline.json) |
| GET /v1/outline/nodes/{node_id} | outline_getNode | FR-08 outline.getNode | [outline.json](../outline.json) |
| PUT /v1/outline/nodes/{node_id} | outline_updateNode | FR-08 outline.updateNode | [outline.json](../outline.json) |
| GET /v1/outline/novels/{novel_id}/beats | outline_listbeats | FR-08 outline.listbeats | [outline-planning.json](../outline-planning.json) |
| POST /v1/outline/novels/{novel_id}/beats | outline_createBeat | FR-08 outline.createBeat | [outline-planning.json](../outline-planning.json) |
| GET /v1/outline/novels/{novel_id}/clues | outline_listclues | FR-09 outline.listclues | [outline-clues.json](../outline-clues.json) |
| POST /v1/outline/novels/{novel_id}/clues | outline_createClue | FR-09 outline.createClue | [outline-clues.json](../outline-clues.json) |
| GET /v1/outline/novels/{novel_id}/nodes | outline_listnodes | FR-08 outline.listnodes | [outline.json](../outline.json) |
| POST /v1/outline/novels/{novel_id}/nodes | outline_createNode | FR-08 outline.createNode | [outline.json](../outline.json) |
| GET /v1/outline/novels/{novel_id}/pacing | outline_pacing | FR-09 outline.pacing | [outline-clues.json](../outline-clues.json) |
| GET /v1/outline/novels/{novel_id}/scenes | outline_listscenes | FR-08 outline.listscenes | [outline-planning.json](../outline-planning.json) |
| POST /v1/outline/novels/{novel_id}/scenes | outline_createScene | FR-08 outline.createScene | [outline-planning.json](../outline-planning.json) |
| POST /v1/outline/scenes/{scene_id}/restore | outline_restoreScene | FR-03 outline.restoreScene | [outline-planning.json](../outline-planning.json) |
| DELETE /v1/outline/scenes/{scene_id} | outline_deleteScene | FR-08 outline.deleteScene | [outline-planning.json](../outline-planning.json) |
| GET /v1/outline/scenes/{scene_id} | outline_getScene | FR-08 outline.getScene | [outline-planning.json](../outline-planning.json) |
| PUT /v1/outline/scenes/{scene_id} | outline_updateScene | FR-08 outline.updateScene | [outline-planning.json](../outline-planning.json) |
