# ADR-0019：条件下载响应声明补齐

- **状态**：Accepted；用户在goal round 4明确批准
- **日期**：2026-09-18
- **关联任务**：INK-179、INK-162
- **替代关系**：补充ADR-0018后的文件下载响应声明；不改变其余R1.1内容。

## 背景

10 分册第 43 行规定 ETag 为 sha256，If-None-Match 只用于已鉴权读。实现先完成当前身份、owner、purpose、novel 授权，再对匹配 ETag 返回 HTTP 304 空响应体。platformDownloadFile 的机读 responses 已有 200/206/400/401/403/404/409/416/500，却没有 304。严格响应验证器因此正确拒绝这一响应。ADR-0018 的四项授权未包含此项，不能将它混入已批准范围。

## 决策

只在 api/platform.json 的 GET /v1/platform/files/{file_id}/download responses 增加304空响应声明；已于2026-09-18重建派生contracts与freeze manifest。

```json
"304": {
  "description": "当前身份与文件授权通过且 If-None-Match 匹配；无响应体"
}
```

具体 RFC 6902 补丁见 [0019-download-304.patch.json](assets/0019-download-304.patch.json)。已保存变更前冻结资产并重建派生 paths-platform 与当前freeze manifest。鉴权失败必须先返回原有错误，不能泄露 ETag；304 不携带 JSON 或二进制正文。DDL、错误码、下载上限、所有权、TTL 均没有变更。

## 备选与代价

- 已采用：保留已鉴权条件下载语义，严格验收新增空响应体分支。
- 未采用：保持当前冻结会使已实现且已鉴权的304分支无法通过机读验收。

## 验证门禁

真实上传并读取 ETag；匹配请求返回 304 且 body 长度为 0；不匹配返回 200 完整原字节；陌生 owner、吊销身份、已删除资源即使携带已知 ETag，也只能返回对应授权错误。默认 15 操作与全部新操作的 Schema 回归、冻结资产与 DDL 字节检查必须通过。

验证结果：真实上传、ETag匹配304空body、非匹配200原字节、授权失败隔离、默认/路由canary、全平台CLI均通过。
