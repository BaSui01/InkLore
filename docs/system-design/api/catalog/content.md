# content API操作映射

> 由build-contracts.cjs生成。字段与响应见源JSON；不是运行完成证明。

| 方法/路径 | operationId | 需求 | 源分片 |
| --- | --- | --- | --- |
| GET /v1/content/chapters/{chapter_id}/body | content_getBody | FR-05 content.getBody | [content-commands.json](../content-commands.json) |
| PUT /v1/content/chapters/{chapter_id}/body | content_saveBody | FR-05 content.saveBody | [content-commands.json](../content-commands.json) |
| POST /v1/content/chapters/{chapter_id}/confirm | content_confirmChapter | FR-07 content.confirmChapter | [content-commands.json](../content-commands.json) |
| GET /v1/content/chapters/{chapter_id}/diff | content_diffVersions | FR-06 content.diffVersions | [content-commands.json](../content-commands.json) |
| POST /v1/content/chapters/{chapter_id}/restore | content_restoreChapter | FR-03 content.restoreChapter | [content-commands.json](../content-commands.json) |
| POST /v1/content/chapters/{chapter_id}/rollback | content_rollbackChapter | FR-06 content.rollbackChapter | [content-commands.json](../content-commands.json) |
| GET /v1/content/chapters/{chapter_id}/versions/{version_id} | content_getVersion | FR-06 content.getVersion | [content-commands.json](../content-commands.json) |
| GET /v1/content/chapters/{chapter_id}/versions | content_listVersions | FR-06 content.listVersions | [content-commands.json](../content-commands.json) |
| DELETE /v1/content/chapters/{chapter_id} | content_deleteChapter | FR-03 content.deleteChapter | [content.json](../content.json) |
| GET /v1/content/chapters/{chapter_id} | content_getChapter | FR-04 content.getChapter | [content.json](../content.json) |
| PUT /v1/content/chapters/{chapter_id} | content_updateChapter | FR-04 content.updateChapter | [content.json](../content.json) |
| POST /v1/content/imports/{job_id}/commit | content_commitImport | FR-18 content.commitImport | [content-jobs.json](../content-jobs.json) |
| GET /v1/content/imports/{job_id}/preview | content_previewImport | FR-18 content.previewImport | [content-jobs.json](../content-jobs.json) |
| POST /v1/content/jobs/{job_id}/cancel | content_cancelJob | FR-18 content.cancelJob | [content-jobs.json](../content-jobs.json) |
| GET /v1/content/jobs/{job_id} | content_getJob | FR-18 content.getJob | [content-jobs.json](../content-jobs.json) |
| POST /v1/content/novels/{novel_id}/archive | content_archiveNovel | FR-02 content.archiveNovel | [content-commands.json](../content-commands.json) |
| GET /v1/content/novels/{novel_id}/chapters | content_listchapters | FR-04 content.listchapters | [content.json](../content.json) |
| POST /v1/content/novels/{novel_id}/chapters | content_createChapter | FR-04 content.createChapter | [content.json](../content.json) |
| POST /v1/content/novels/{novel_id}/copy | content_copyNovel | FR-02 content.copyNovel | [content-jobs.json](../content-jobs.json) |
| POST /v1/content/novels/{novel_id}/exports | content_createExport | FR-19 content.createExport | [content-jobs.json](../content-jobs.json) |
| POST /v1/content/novels/{novel_id}/imports | content_createImport | FR-18 content.createImport | [content-jobs.json](../content-jobs.json) |
| GET /v1/content/novels/{novel_id}/jobs | content_listJobs | FR-18 content.listJobs | [content-jobs.json](../content-jobs.json) |
| POST /v1/content/novels/{novel_id}/organization/reorder | content_reorderOrganization | FR-04 content.reorderOrganization | [content-commands.json](../content-commands.json) |
| GET /v1/content/novels/{novel_id}/organization | content_getOrganization | FR-04 content.getOrganization | [content-commands.json](../content-commands.json) |
| POST /v1/content/novels/{novel_id}/restore | content_restoreNovel | FR-03 content.restoreNovel | [content-commands.json](../content-commands.json) |
| GET /v1/content/novels/{novel_id}/search | content_search | FR-20 content.search | [content-jobs.json](../content-jobs.json) |
| GET /v1/content/novels/{novel_id}/statistics | content_statistics | FR-20 content.statistics | [content-jobs.json](../content-jobs.json) |
| GET /v1/content/novels/{novel_id}/volumes | content_listvolumes | FR-04 content.listvolumes | [content.json](../content.json) |
| POST /v1/content/novels/{novel_id}/volumes | content_createVolume | FR-04 content.createVolume | [content.json](../content.json) |
| DELETE /v1/content/novels/{novel_id} | content_deleteNovel | FR-03 content.deleteNovel | [content.json](../content.json) |
| GET /v1/content/novels/{novel_id} | content_getNovel | FR-02 content.getNovel | [content.json](../content.json) |
| PUT /v1/content/novels/{novel_id} | content_updateNovel | FR-02 content.updateNovel | [content.json](../content.json) |
| GET /v1/content/novels | content_listNovels | FR-02 content.listNovels | [content.json](../content.json) |
| POST /v1/content/novels | content_createNovel | FR-02 content.createNovel | [content.json](../content.json) |
| POST /v1/content/volumes/{volume_id}/restore | content_restoreVolume | FR-03 content.restoreVolume | [content-commands.json](../content-commands.json) |
| DELETE /v1/content/volumes/{volume_id} | content_deleteVolume | FR-03 content.deleteVolume | [content.json](../content.json) |
| GET /v1/content/volumes/{volume_id} | content_getVolume | FR-04 content.getVolume | [content.json](../content.json) |
| PUT /v1/content/volumes/{volume_id} | content_updateVolume | FR-04 content.updateVolume | [content.json](../content.json) |
