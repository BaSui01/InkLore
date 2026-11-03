# 前端复用、状态与接口适配合同 R1

状态：INK-141 冻结目标，尚未实现。遵循 [00唯一基线](00-CONTRACT-BASELINE.md)；页面组合见 [15](15-CREATOR-FUNCTION-UX-SPEC.md)、[16](16-ADMIN-FUNCTION-UX-SPEC.md)，tokens/无障碍见 [17](17-DESIGN-SYSTEM-AND-ACCESSIBILITY.md)。冻结决策依据[ADR-0016](../../.agents/notes/reference/decisions/0016-r1-reconstruction-design-freeze.md)。本册冻结React19、ReactRouter7、TanStackQuery5、Ajv8+formats3、antd6下的模块边界，不引入第二个状态框架、表单框架或富文本框架。

## 1. 目录、依赖与事实Owner

下列是R1目标模块目录，不表示已经创建源码。frontend/apps/creator与admin独立入口、路由、构建、ConfigProvider、QueryClient、AuthSession，分别strictPort3000/3001。frontend/packages作为工作区包发布给两端，不从一个app导入另一个app。

|目录|提供的模块|允许依赖|明确不持有|
|---|---|---|---|
|packages/contracts|api生成wire类型、UUID/Revision/Timestamp、事件判别联合、验证Schema索引|无运行时app依赖|HTTP调用、页面状态、业务默认数据|
|packages/transport|ApiClient、AuthSession、StreamClient、NotificationClient、OperationRegistry|contracts、浏览器fetch/IndexedDB接口的技术适配|Novel/Role/Run业务状态机、组件|
|packages/ui|PageGate、ErrorPresenter、SchemaForm、DiffView、FilePicker、StatusBanner、DataTable、Modal、ThemeProvider、i18n|contracts、antd6、Lucide、Ajv8；通过props接收动作|域API、业务工作流、全局query实例|
|packages/run-ui|RunInspector、ApprovalCard、EventTimeline、UsageSummary|contracts、ui；输入typed view与callbacks|读取作者正文权限、自动执行工具、创建run|
|apps/creator/domains/content|EditorController、RecoveryRepository、PlainTextEditor、章节/版本/结构hooks和adapter|contracts/transport/ui/run-ui|worldview私有仓储、admin页|
|apps/creator/domains/outline|OutlineTree/SceneBoard/Foreshadowing、域adapter/hooks|公共包|content写状态|
|apps/creator/domains/worldview|CharacterForm/EntryForm/RelationGraph/FactLedger、adapter/hooks|公共包|content确认事务|
|apps/creator/domains/inspiration|Asset/Analysis/Style/Card视图、adapter/hooks|公共包|直接写正文/事实|
|apps/creator/domains/compliance|Report/Rewrite/Issue/Review视图、adapter/hooks|公共包|直接写正文|
|apps/creator/domains/agent|Run/Workflow页面、agent adapter/hooks、产物采纳编排|公共包；通过页面注入ContentDraftPort|content库/LLM配置治理|
|apps/admin/domains/llm|Provider/Model/Channel/Key/Policy/Budget页面及adapter/hooks|公共包|creator正文控制器|
|apps/admin/domains/agent|Definition/Workflow/Tool/MCP/脱敏运行视图|公共包|作者产物采纳|
|apps/admin/domains/platform|成员/审计/观测adapter/hooks|公共包|业务域私有状态|
|apps/admin/domains/compliance|规则治理视图及adapter/hooks|公共包|作者报告正文|
|apps/*/app|Router、navigation、QueryClient、providers、跨页面组合与失效登记|本端domain公开入口、公共包|领域仓储、任何另一端app导入|

依赖方向 app composition→domain components/hooks→domain adapters→ApiClient→contracts；presentation props→ui/run-ui。ui/run-ui不得反向import domain/app，contracts不得依赖transport。跨域页面组合在本端app层通过公开callbacks/只读view完成，不在共享层建AllBusinessService。EditorController仅creator/content拥有；它通过明确定义端口被AI页面组合复用，不为“共享”搬到双端包。

Owner划分：服务器实体/列表由TanStackQuery持有；认证access/CSRF由AuthSession内存持有；URL只持非敏感路由/筛选；编辑草稿/选区由EditorController；普通表单草稿由页面useState/reducer；流缓冲由订阅控制器、持久事件以agent为权威；theme/locale/字号为本端偏好；恢复副本由creator RecoveryRepository；权限权威永远服务端。不能在组件、Query、全局store各保存一份可变Chapter。

## 2. wire与视图类型

UUID为小写带连字符UUIDv4品牌字符串；Revision与seq在wire及组件内均为number类型正整数，范围1..9007199254740991。ApiClient以Number.isSafeInteger(value)并检查上下界，Ajv使用common Revision/事件Schema相同maximum；超限返回协议校验错误，拒绝缓存和提交。正文控制器只接受服务器返回revision，不自行算术+1。标准JSON整数序列化保持原值，不引入额外整数解析依赖，组件与wire保持同一number类型；seq游标传Last-Event-ID时仅作精确十进制格式化。

公共成功Success<T>={code:ok,data:T,request_id:UUID,trace_id:string}；失败ApiError={httpStatus,code,message_key,details,request_id,trace_id,retryAfter}，details仅安全typed字段field_errors/current_revision/retry_after_seconds/conflicts/operation_id。客户端绝不依靠message文本判断业务。ApiAdapter显式decode类型化Schema、map view、encode command；view可以camelCase，但wire永远snake_case；不可用任意object绕过Schema。

ScopeKey={tenantId,userId,audience,sessionEpoch,permissionRevision,novelId?}；ResourceKey={domain,type,id}。sessionEpoch是本地每次主体重置递增值，不是服务端revision；请求完成时ScopeKey不再相等则丢弃其结果，不进入新主体缓存。列表View={items:T[],nextCursor:null|string,hasMore:boolean}；未知总数不推算。Unknown wire enum显示受限只读“无法识别状态”并报告协议问题，不将其映射为成功。

## 3. 复用模块输入输出与组合

### 3.1 ApiClient（技术包）

输入：request<TReq,TRes>({operationId,method,path,pathParams,query,body,responseSchema,scope,signal,idempotencyKey?,expectedRevision?})；构造时注入baseURL、getAccessToken、refreshOnce、getCsrf、onSessionInvalid、clock、safeTelemetry。输出Promise<Success<TRes>>或ApiError；二进制输出DownloadResult(blob,filename,mediaType,sha256)，不能硬套JSON。

状态所有权：客户端仅持单飞refresh promise和请求AbortController，不持领域实体缓存。operationId与method/path由生成映射确定，域adapter不能自由拼旧路由。GET默认网络重试最多2次（短退避1/2秒，仅网络/503且受abort控制），命令默认0自动重试；显式unknown重试由OperationRegistry同键同载荷调用。401只允许单次refresh后同原请求重试，幂等键不变，主体epoch变化拒绝继续；403/404/422不重试。

Header：Authorization Bearer内存access、Accept、内容类型；写入Idempotency-Key；DELETE expected_revision在query，其他状态命令放JSON；自动附request追踪不把request_id当幂等key。Cookie credentials策略限同源API，GET /v1/auth/session-csrf获取CSRF nonce后POST refresh；nonce只内存，刷新/退出附Origin和CSRF。非JSON下载错误先按Content-Type判别安全错误封包，不下载错误页当文件。所有错误统一给ErrorPresenter，不在每个API函数弹Toast。

验收 RE01：同页10个401仅一次refresh，重放仍原幂等key；403不刷新；换主体迟到响应不入新缓存；revision/seq上界安全且超界被拒绝；413明确尺寸错误。

### 3.2 AuthSession与PageGate

AuthSession输入audience与平台adapter，输出snapshot={status:bootstrapping|authenticated|anonymous|expired,subject,scope,accessExpiresAt}和login/refresh/logout。access与CSRF不在localStorage/sessionStorage/IndexedDB；主题与恢复正文也不能夹带token。登出先阻止新请求与流、清Query/OperationRegistry/恢复副本、调用带CSRF退出并在本地结束会话；服务端失败保留明确“服务端退出未确认”提示，但本端不保留access。

PageGate Props={session,scopeQuery,resourceQueries,requiredCapability,children,fallbacks,onRetry,onReauthenticate}。输出一个稳定内容区域及派生GateState；只派生不另存query。判定优先级：未认证→authGate；明确403→forbidden；对象404→notFound；无快照首次失败→error；无快照请求→loading；有部分独立数据→partial；离线→offline overlay；成功快照过期→stale overlay；成功空列表→empty；其余ready。offline/stale可与partial并存以标签集合表达，主阻断态只有一个。

资源级Gate将父novel归属及chapter归属校验串联，不能“已登录即允许请求所有ID”。权限缺失不发敏感detail query；同一页面只读与写permission分开，auditor仍可看安全摘要。forbidden清对应query并关闭敏感抽屉，不把权限错误当空。清未提交稿遵循15的本人显式下载规则，不能因导航刷新把稿广播给其他主体。

组合：CreatorChapterRoute→AuthGate→NovelPageGate→ChapterPageGate→Editor；AdminRunRoute→AdminAudienceGate→AuditRunPageGate→RunInspector(mode=redacted-summary)。验收 RE02：直接深链跨租户404、撤权后无缓存闪现、子资源不能跳过父scope。

### 3.3 ErrorPresenter

Props={error:ApiError|NetworkError,context:{operationLabel,resourceLabel?,hasDraft},variant:inline|banner|dialog,onRetry?,onResolveConflict?,onLogin?}；输出错误标题、局部解释、字段摘要、恢复按钮和安全诊断复制。无本地状态所有权，展开详情只属组件。

映射：400格式/游标失效→回首游标重读；401登录；403停止并清scope；404统一不可访问；409 revision→DiffView、idempotency_conflict→固定操作不可改载荷、operation_in_progress→Retry-After核对；413缩小输入/文件；422定位field；429显示预算/容量及等待；500未知保留稿；502/503/504若已有operation/run则核对，不创建替代run。不得显示SQL、Key、正文、stack。验收 RE03：同一个错误只产生一个持久可见提示；Toast辅助不能替代字段错误。

### 3.4 StreamClient与NotificationClient

StreamClient仅creator audience订阅。subscribe({runId,scope,lastSeq,onEvent,onConnection,onError,signal})返回unsubscribe与getCursor；调用GET /v1/agents/runs/{run_id}/events，使用fetch ReadableStream解析SSE，支持Authorization header、UTF8跨chunk、多行data、event/id、15秒注释心跳。不能以原生EventSource无法带Bearer为由把JWT放URL。输入事件判别联合严格验证envelope={run_id,seq,occurred_at,payload}，seq必须递增，重复丢弃、缺口从lastSeq重连；不按文本重复内容去重。

事件run.started/text.delta/reasoning.summary/tool.started/tool.completed/approval.required/node.updated/usage.updated/run.succeeded/run.failed/run.cancelled；只有reasoning.summary，不显示隐藏推理。接收终态后标记completed并GET run权威摘要、停止自动重连；EOF没有终态只为disconnected，绝不推断成功。重连1/2/4/8/16/30秒带抖动，Last-Event-ID保持；401按ApiClient刷新一次，403停止，410事件过期先GetRun并提示历史不完整。unsubscribe仅关闭GET，不调用CancelRun。只有UI显式cancel callback调用带CAS+幂等的取消命令。

流状态idle→connecting→open→reconnecting→closed；terminal是run状态而非连接状态。每run每app一个活动订阅，页面引用计数；离页无消费者断开，后台继续，列表可低频权威GET。流增量只进入RunEventBuffer，不写正文/版本。缓冲最多保留最近2000事件/2MiB展示片段，超过截取旧展示段并提供权威产物读取，不能把截取后的文本当完整可采纳产物。

NotificationClient使用POST /v1/auth/ws-ticket获取60秒一次性票据，再WS /v1/platform/events，URL只票据。消息仅event_id/topic/resource_id/revision/occurred_at，不正文；event_id去重，按注册表失效query。断连1..30秒退避，重连使活动资源重新GET，不宣称补齐通知历史。WS故障不显示API整体离线。RE04：断SSE/关页后台继续；重复seq无重复段；410不重建run；WS重连重读权威值。

### 3.5 RunInspector

Props为判别联合：owner分支接收mode=owner、RunView、nodes、events、artifacts、connectionState、permissions和显式callbacks；admin分支只接mode=redacted-summary、AuditRun、onRefresh。owner输出节点/事件/usage事件/产物，admin仅node_count/duration_ms/cost_micros/error_code摘要，绝不传作者完整Run再CSS隐藏。组件本地仅折叠、滚动、所选安全项；请求与鉴权由域hooks/PageGate负责。

owner产物读取必须当前作品scope；admin仅GET /v1/agents/audit/runs及其详情，不可调用creator-only run/events/artifacts/cancel。admin无节点细节时不渲染假空轨迹。creator ArtifactReview→Agent.GetArtifact→EditorController.previewProposal；管理诊断用独立DiagnosticInspector，GET diagnostics/{run_id}轮询typed Diagnostic。RE05：共享展示基础但两端输入不同typed投影，admin网络/props无原文，关闭抽屉不取消run。

### 3.6 SchemaForm

Props<T>={schema:JSONSchema202012,schemaId,schemaRevision,value:T,onChange,fieldErrors,readOnly,disabled,onSubmit,submitLabel,uiHints}；输出onChange(nextValue,validationSummary)/onSubmit(validatedValue)。value/dirty由调用页面拥有；内部仅focus/touched/展开分组；不得在schema default改变后自动覆盖用户value。Ajv8 strict模式+formats3编译缓存key=(schemaId,schemaRevision,hash)，关闭coerceTypes/removeAdditional/useDefaults副作用；初始化默认值是页面显式动作。

支持object properties/required/additionalProperties=false、数组items/min/max/uniqueItems、嵌套对象、string min/max/pattern/format、integer/number min/max、boolean、enum、const、nullable的type union、oneOf/anyOf/allOf/$ref本地受信资源。oneOf使用显式分支选择并验证唯一分支；allOf合并约束不只合并UI标签。禁止远程任意URL $ref拉取；循环Schema/超深或无法渲染结构显示UnsupportedSchemaError并禁提交，保留已填值供导出安全表单，不执行错参数。

uiHints只labelKey/helpKey/order/widget/secretDisplay策略，不覆盖Schema限制。数组增删有可访问名称，错误路径映射JSON Pointer，分支切换提示被清字段并可取消；隐形额外字段也必须拒绝，不能只验证屏幕上字段。内置title/name长度与JSON 2MiB预检；服务端422覆盖到对应字段；unsupported schema不得降级JSON字符串偷偷提交。RE06：temperature=0、false、空字符串三者区别；嵌套数组错误正确聚焦；schemaRevision更新脏表单先差异确认。

### 3.7 DiffView

Props={base?:TextSnapshot,left:TextSnapshot,right:TextSnapshot,mode:twoWay|threeWay,sourceLabel,onSelectResolution?,readOnly}；TextSnapshot={resourceId,revision,contentHash,text,status:loaded|error|loading}。输出按行/码点的DiffHunk及用户选择；差异计算纯函数，可由本端Worker后台执行，组件本地只持展开状态。任何一侧未loaded禁止当空字符串计算；大章200000码点按块呈现/虚拟化并有全文文本对照，不卡主输入。

三方合并不自动保存；onSelectResolution返回MergedDraft+remoteBaseRevision，EditorController负责设draft及再次CAS。新增/删除除颜色有文字标记，键盘上下块、左右选择、可复制本地全文。不得让DiffView直接调用回滚或保存API。RE07：重复行保持来源位置、冲突再次变化重新核对、审计SafeDiff只接安全配置字段不含正文。

### 3.8 EditorController与ContentDraftPort

位于creator/content，输入{documentKey,initialSnapshot,savePort,recoveryPort,clock,selectionAdapter,onConflict,onScopeLost}。输出snapshot={base,draft,titleDirty,bodyDirty,dirty,saveState,recoveryState,selection,editGeneration}及commands setText/setSelection/requestSave/previewProposal/applyProposal/undo/redo/resolveConflict/flushRecovery/dispose。

savePort.updateMetadata({chapterId,title,expectedRevision,idempotencyKey})→Chapter，savePort.saveBody({chapterId,body,expectedRevision,idempotencyKey})→Body，loadLatest(chapterId)→Body；wire分别PUT chapters/{id}与chapters/{id}/body，SaveBody没有title/source字段。统一保存协调器串行持有单章写锁，metadata成功返回的revision交给body；每步独立幂等键/失败状态，metadata已成功而body失败时titleDirty=false/bodyDirty=true，不能回滚标题或显示全章已保存。版本仅正文保存按content规则产生。RecoveryPort.load/list/write/remove/clearSubject返回typed结果与QuotaError；ClockPort.now/schedule/cancel只做保存计时不伪造业务进度。ContentDraftPort仅getAnchor/previewProposal/applyReviewedProposal，领域建议接受后调用正文保存，不把content API封装注入共享UI。

base由服务器成功读取/写入推进；draft由用户输入/明确采纳推进。30秒停输或60秒最长间隔请求保存；单章单在途冻结payload，按metadata→body串行，未变化步骤跳过；期间输入归pendingLatest，成功旧回包不覆盖新draft。409停止自动写，base/local/remote三方，人工合并用新remote revision+新操作key；unknown固定旧key原payload核对。dispose关闭计时/订阅，不能偷偷CancelRun。

本地选区锚点chapter/source_revision/content_hash/start_offset/end_offset/selected_text_hash，UTF16转Unicode码点；adapter把chapter映射resource_id、content_hash映射source_hash并构造selection={start_offset,end_offset,source_hash}，selected_text_hash不发送到wire；任何基线变化禁自动按旧位置写。sourceRevision必须已保存，编辑draft与服务器不等时先保存再发AI。采纳更新draft一个undo单元，保存才成为content版本。RE08：IME不提交半字、A/B冲突不丢稿、切章迟到响应不改新章、重复句不靠indexOf定位。

### 3.9 RecoveryRepository

IndexedDB仅creator；主体键tenant/user，文档键novel/chapter，副本包含recovery_id/base_revision/base_hash/draft/saved_at/checksum。停输2秒、7天TTL、每章最多3份、主体100MiB软上限。写入与旧备份淘汰同一个IDB事务；过期优先，再淘汰未打开章旧备份，当前最新未同步稿保留。异常QuotaExceeded/不可用/权限撤销返回RecoveryUnavailable，UI持续告警、允许本地文本下载；不能仅打console。

shared-device默认disabled；启用用户确认本地留存，退出主动clearSubject。恢复读取先校checksum/主体/TTL，显示差异后恢复到draft，不自动提交；损坏副本隔离并允许安全丢弃，不当作服务器版本。schema迁移在IDB事务执行，失败不清库后装作成功。不能保证断电最后2秒数据。RE09：三份上限、7天清理、100MiB软限、不跨主体恢复、主动退出清库。

### 3.10 ApprovalCard

Props接approval:ApprovalView、actorId、canDecide、riskSummary、serverNow及onApprove/onReject/onReload；仅creator页面接入真实Approval。输出decision载荷expected_revision、arguments_hash、source_revision、可选reason，并携idempotencyKey；本地仅确认弹窗/请求状态。admin审批审计用AuditTable+ApprovalAuditSummary读取Platform Audit，不调用creator-only票据接口，不接arguments_json。

仅actorId等于owner_user_id且canDecide、pending、未过期、scope有效显示按钮，服务端最终鉴权。展示run/node/tool_version_id/arguments_hash/source_revision/影响/到期；参数摘要未完整禁批。批准status=consumed，拒绝rejected，另有expired/cancelled；命令CAS消费一次、恢复同run、不重跑副作用。到期GET Approval核对，reason按Schema。RE10：管理员不能代作者批，双标签重复安全，参数变化票据失效。

### 3.11 FilePicker

Props={purpose,accept:FileTypeRule[],maxInputBytes:20971520,multiple:false,disabled,uploadPort,onUploaded,onRejected}；uploadPort.upload(file,{idempotencyKey,signal})→FileMetadata；downloadPort由调用方另传DownloadAction。输出file_id/media_type/size_bytes/sha256，绝不输出Base64入表单。

状态idle→selected→validating→uploading→uploaded或error/unknown；本地文件名/MIME只预检，服务器签名与内容再验。上传进度仅真实传输可报告；fetch无可靠上传字节时显示不定进度而非假百分比。重试unknown同键同文件；替换文件新key。取消网络不证明已删服务器文件，先核对file_id；文件Owner有清理策略，不以自动DELETE补偿未知上传。平台文件DB bytea权威，鉴权下载≤100MiB，不构建外链签名URL。

组合：creator ImportPreview/FilePicker→platform upload→content preview/commit；inspiration AssetForm→upload→CreateAsset；admin审计GET NDJSON导出不用FilePicker。RE11：20MiB边界、伪扩展名、401下载、超时不重复记录、取消不误删已引用文件。

### 3.12 R1.x TaskPolicyPicker与QuestionController

TaskPolicyPicker只消费Agent Creator安全目录API，持tenant/user/audience scope query key与标准cursor；视图字段仅policy_id/policy_version_id/task_key/title/version/task_kinds/input_schema/budget，不暴露Admin LLM策略对象。选择值是不可变version_id；提交CreateRun时task_key不得出现在Creator payload。目录无数据/失败禁提交。策略竞态409保留表单，刷新目录后由作者再次确认，不静默改选。

QuestionController归Creator Agent页面。输入run/question ID、scope、question view与API/Stream ports；提供GET恢复、answer与subscribe/cursor。Question schema只允许安全单选/多选/文本，不渲染HTML；答案与source revision/hash绑定且视作untrusted data，仅追加到Run上下文，不调用Content/Worldview写入。回答命令携expected_revision、question_version和固定Idempotency-Key；同键同载荷重放，别键并发CAS输方GET权威状态。`question.required`保存question view，`question.answered`仅显示hash和actor，不回显事件答案。挂起/断线/退出保留后台run，页面重新加载GET恢复并以Last-Event-ID继续同一Run；仅明确CancelRun取消。Admin RunInspector维持redacted-summary判别分支，不显示题目原文或答案。



每app创建独立QueryClient；key统一[appKind,tenantId,userId,sessionEpoch,permissionRevision,domain,resourceType,scopeId,view,normalizedFilters]，列表分页pageParam为opaque cursor，filters稳定序列化含sort/limit。单体详情key末项resourceId；runEvents独立事件缓冲，不把每个delta invalidation触发GET风暴。禁止queryKey只用chapterId或runId。

默认staleTime：可变详情30秒、列表15秒、运行/审批0秒、Schema/不可变版本5分钟；gcTime5分钟，敏感正文/完整产物离开最后消费者后60秒回收且不持久化；退出/撤权立即removeQueries不等GC。配置刷新聚焦时可重读；dirty表单与Editor不被Query变化覆盖。Query只能持服务器事实，不能用setQueryData写未保存draft。

|成功动作/事件|立即缓存动作|必须失效的查询|保留/禁止|
|---|---|---|---|
|Create/Update/Delete/RestoreNovel|返回实体写当前scope详情；删除移除正文/权限缓存|书架、回收站、作品scope、统计|不可把软删正文放共享缓存|
|UpdateChapter metadata|更新title基线和返回revision，不清bodyDirty|章节、结构标题、书架更新时间、search|不创建正文版本，不覆盖未保存正文|
|SaveBody/RollbackChapter|写返回chapter base|章节、结构字数、versions、统计、search；旧报告/产物标stale|保留较新draft；不更新事实账本为已撤销|
|ConfirmChapter|更新chapter确认与world revision|chapter、fact heads/events、角色事实摘要|事务失败不做局部乐观确认|
|Outline/Scene/Beat/Foreshadowing变更|写返回域实体|对应树/场景/伏笔/节奏与上下文预览|不直接改content正文|
|Worldview档案/关系/事实更正|写返回实体或新事件|角色、设定、图谱、账本、上下文；相关旧报告标stale|append事件不可客户端覆盖旧事件|
|CreateAnalysis/Report/Rewrite|缓存pending记录|各列表；CreateRun成功关联run|创建run失败保留pending记录，允许同业务ID重试启动|
|Run终态/产物就绪|终态GET权威run，停止订阅|run列表、artifacts、关联analysis/report/rewrite、usage|不自动采纳产物或改章节|
|Approve/Reject/Cancel|写响应票据/run或失效重读|approval列表、run、nodes、usage|等待服务端真实终态|
|LLM配置/预算更新|返回配置revision|对应目录、影响策略、overview、usage/budget|旧run快照不改；不自动替换模型选择|
|Member权限变化|清目标scope敏感缓存并重新授权|session/membership/成员/审计|角色JWT不能覆盖即时权限结果|
|WS资源revision更高|按domain/type路由失效|当前活动详情/对应列表；重连重读活动资源|较旧通知忽略；通知不作权威写值|

列表删除不本地推算游标，成功后重新拉首屏或当前可验证页；重排用后端原子批量能力，失败不留下乐观半序。列表选中项ID保留但执行前再查可用性。cursor_invalid清cursor回首屏保留筛选，不能用旧cursor不同query继续翻页。

## 5. 事务、幂等与应用编排端口

OperationRegistry存内存{operationId,key,canonicalMethodPath,payloadHash,scope,createdAt,state,resultRef}；state prepared→inflight→succeeded|failed|unknown。key UUIDv4每次用户新意图生成，网络重试/401重放不变；同key载荷不可变。24小时窗口服务端幂等为权威，客户端超窗口unknown不得直接新key再执行危险命令，先资源/operation查询核对。不得将password/secret/正文载荷持久化OperationRegistry；正文恢复另归RecoveryRepository。

页面ApplicationFacade是领域hooks的薄编排，不是公共god-service：

|调用者|调用顺序/Port|失败处理与事实Owner|
|---|---|---|
|ChapterPage|EditorController→ContentSavePort.updateMetadata/saveBody→Content.UpdateChapter/SaveBody|metadata与body为两个串行CAS命令；各自幂等。只有body事务包含正文/版本/outbox；允许标题成功正文失败持续dirty，不补偿删版本|
|ConfirmDialog|Content.ConfirmChapter→服务端WorldviewFactPort同UoW|UI只发一个确认；章与账本同提交/同失败|
|AnalysisPage|Inspiration.CreateAnalysis(201 pending)→Agent.CreateRun(task_kind=inspiration_analyze，resource.resource_type=analysis，resource.resource_id=analysis_id，resource.source_revision=analysis.revision)|run受理失败保留analysis，重新启动沿原analysis_id，不重复资料上传|
|QualityPage|Compliance.CreateReport/CreateRewrite→Agent.CreateRun的task_kind为compliance_audit或compliance_rewrite；resource_type为report或rewrite、resource_id为对应ID、source_revision为该记录revision|run失败报告显式失败/可核对，不把pending当通过|
|ArtifactReview|Agent.GetArtifact→ContentDraftPort.preview/apply→Content.SaveBody|草稿预览不写事实；只有作者接受+保存才落content|
|RuleGovernance|Compliance.CreateRuleSet(base_rule_set_id克隆)→规则CRUD→PublishRuleSet|发布前完整Schema验证，CAS失败整集不发布|
|AdminDiagnostic|Agent.CreateDiagnostic(task_key,probe_key)→GetDiagnostic轮询|仅固定探针，不接受任意工具参数；不调用creator的events/cancel/approval|
|MemberEditor|Platform.UpdateMember→失效授权/成员/审计|平台最后operator事务保护；UI不直接吊改其他tenant|

浏览器不持DB事务；服务器Owner仓储查询全部tenant/scope索引化。UI读取需求索引语义：按tenant+owner+updated_at/id列表、tenant+novel+position树、tenant+chapter+created_at版本、tenant+run+seq事件、tenant+actor+status审批、tenant+resource+created_at审计；data分册实现真实SQL，公共前端包没有SQL、没有业务跨表join。跨域读通过适配器/各域API并行组合，局部失败不伪成功。

## 6. FR→组件→能力→Owner→验收矩阵

|需求|主组件与组合|接口能力/Owner|验收|
|---|---|---|---|
|FR-01 认证|AuthSession+PageGate|Auth.Login/SessionCsrf/Refresh/Logout；platform|RE01/02，双端audience、内存token、清主体缓存|
|FR-02 生命周期/副本|BookGrid+NovelSettings|Content.Create/Update/Archive/CopyNovel；content|返回202 Job，succeeded后使用target_novel_id，复制仅指定快照范围、不复用源revision|
|FR-03 软删恢复|DangerZone+RecycleList|Content.Delete/Restore；content|同ID新revision、30天界限与scope撤销|
|FR-04 卷章排序|StructureTree|Content.Move/Reorder；content|原子排序、键盘等价、CAS409|
|FR-05 编辑防丢|EditorController+RecoveryBanner|Content.SaveBody；content|RE08/09，2秒/7天/3份/100MiB及30/60秒|
|FR-06 版本|VersionList+DiffView|Content.ListVersions/Rollback；content|RE07，新版本回滚不撤事实|
|FR-07 确认|FactProposalReview|Content.ConfirmChapter→WorldviewFactPort|整事务、逐条冲突resolution|
|FR-08 大纲场景节拍|OutlineTree+SceneBoard|Outline.SaveNode/Scene/Beat；outline|三级/无环/正文不变|
|FR-09 伏笔节奏|ForeshadowingBoard+RhythmChart|Outline.TransitionClue；outline|合法状态迁移、图表同权威数据|
|FR-10 角色设定图谱|CharacterForm+EntryForm+RelationGraph|Worldview CRUD；worldview|表图等价、scope/UUID引用|
|FR-11 事实更正|FactLedger|Worldview.SupersedeFact/RetractFact；worldview|只追加supersede/retract|
|FR-12 AI选区|SelectionToolbar+ArtifactReview|Agent.CreateRun→ContentDraftPort|RE04/08，码点锚点、作者采纳|
|FR-13 拆书|AnalysisPanel+FilePicker|Inspiration.CreateAsset/CreateAnalysis→Agent.CreateRun|RE11，pending后run，失败不伪结果|
|FR-14 文风灵感|StyleProfile+InspirationCard|Inspiration.Style版本CRUD/Card CRUD/RecordAdoption/RevokeAdoption；inspiration|外部建议不自动写正文|
|FR-15 质检净化|FindingsTable+DiffView|Compliance.Scan/CreateReport/CreateRewrite→Agent.Run；ListIssues/Ignore/Reopen/CreateReview|源revision/hash过期禁直接替换|
|FR-16 DAG|WorkflowGraph+SchemaForm|Agent.CreateWorkflow/UpdateWorkflow（生成不可变版本）|RE06，无环、不可变发布快照|
|FR-17 审批取消SSE|RunInspector+ApprovalCard|Agent.Runs/Events/Approve/Reject/Cancel|RE04/05/10，本人审批同run|
|FR-18 导入|FilePicker+ImportPreview|Platform.Upload→Content.Import|≤20MiB、预览确认、未知同键|
|FR-19 导出|ExportJobs+DownloadAction|Content.Export→Platform.Download|≤100MiB，快照版本、鉴权|
|FR-20 检索统计|SourceAnchor+WritingChart|Content.Search/Statistics|码点定位、北京时间自然日|
|FR-21 模型渠道Key|SchemaForm+SecretReferencePicker|LLM.Provider/Model/Channel/Key|无Key正文，引用约束、CAS|
|FR-22 策略预算|PolicyMatrix+BudgetForm|LLM.Policy/Budget/Usage|temperature0、预留结算、429|
|FR-23 Agent工具MCP|DefinitionForm+SchemaForm+DiagnosticInspector|Agent.Definition/Tool/Mcp/Diagnostic|受控Schema与两种固定probe、admin无作者正文/票据入口|
|FR-24 成员审计观测|MemberForm+AuditTable+DependencyCards|Platform.Members/Audit/Status|最后operator、脱敏、Redis degraded|
|FR-25 共享UI|全部公共模块|技术包+域adapter|RE01..11与17的DS01..10；不跨app依赖|

接口x-use-case以对应FR-XX开头，域adapter登记operationId→FR→权限→query失效集合；一操作可覆盖多个FR但不得无所属。组件目录、输入输出、数据Owner和页面验收同时变更才算完成复用变更，不用“共享组件已存在”代替实际调用链。

## 7. 实施验收与文件治理

不编写测试文件；验收以真实API/浏览器/故障注入结果记录。必须检查两app打包依赖无反向跨app导入，公共ui/run-ui无业务adapter import；每个模块使用方至少在本册或页面册有明确调用者。验证身份切换、并发CAS、未知结果同键核对、权限撤回缓存、SSE断流/410、Schema失败、文件边界和编辑恢复。

本册不建新业务表；IndexedDB是恢复副本非权威表，不改变所有服务器实体UUIDv4/tenant_id/timestamps/mutable revision、tenant FK到platform_tenants、可选user FK到platform_users、跨业务引用UUID无crossjoin规则。所有真实DDL在data，机读OpenAPI分片在api；本册只引用接口能力与类型Owner。R1变更通过00规定的R1.x影响记录，不能把未确定实现留给页面自行选择。
