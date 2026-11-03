> 历史记录：本文件保留迁移前的设计或实施证据，不是当前工程规范，也不证明当前功能已验收。当前规则见 [AGENTS](../../../../AGENTS.md)；实现以源码和本轮验证为准。

# 设计系统与无障碍合同 R1

状态：INK-141 冻结目标，尚未实现。遵循 [00基线](00-CONTRACT-BASELINE.md)，页面功能见 [15创作者](15-CREATOR-FUNCTION-UX-SPEC.md)、[16管理端](16-ADMIN-FUNCTION-UX-SPEC.md)，Props与状态归属见 [20](20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md)。本册是一套语义token、断点、组件交互与验收规则，双端只在明确theme值和页面组合上区别，不创建第二套框架。

## 1. 应用与组件边界

React19/TS5.8/Vite7/antd6/Lucide；Creator:3000、Admin:3001分别入口、路由表、构建产物和strictPort。共享包不含端别切换，Creator不显示治理菜单，Admin不显示作者正文。导航、翻译、样式参数是静态配置；模型、作品、曲线、价格、执行进度必须真实API数据，不能用固定数组兜底。

antd ConfigProvider承载统一token与locale，App承载message/modal上下文。页面只使用共享包装组件，必要原生语义控件仍保留浏览器行为；不另造组件框架、不替换为另一套UI库。原生textarea是唯一正文输入核心；图谱与工作流使用SVG/Canvas并配同权限等价表。所有可见文案含错误、空态、按钮、aria-label均t(key,params)，默认zh-CN，提供en-US完整词条；每个页面使用同一locale，不裸露拼接双语。未知后端message_key显示本地安全兜底与错误code，不回显堆栈。

## 2. 单套token与双端theme

单位为CSS px，字体使用rem随根字号缩放；各业务组件只读语义token，不散落hex。主题由(appKind,mode)映射，mode=light|dark，默认跟随系统，用户可手动选择，偏好仅存无敏感信息的本地设置。下面四列是确定值。Creator纸色只作用canvas/paper，不改变公共状态语义；Admin使用冷灰背景。

|token|Creator light|Admin light|Creator dark|Admin dark|
|---|---|---|---|---|
|bg.canvas|#F6F3ED|#F5F7FA|#171A20|#111827|
|bg.surface|#FFFFFF|#FFFFFF|#1F2937|#1F2937|
|bg.paper|#FFFCF5|#FFFFFF|#20242C|#1F2937|
|bg.subtle|#EEEAE1|#EEF2F6|#2A303B|#273449|
|text.primary|#172033|#172033|#F3F4F6|#F3F4F6|
|text.secondary|#4B5563|#4B5563|#D1D5DB|#D1D5DB|
|text.muted|#526071|#526071|#B5C0D0|#B5C0D0|
|border.decorative|#D7D2C7|#D8DEE8|#465061|#42516A|
|border.control|#6B7280|#6B7280|#94A3B8|#94A3B8|
|action.primary|#1D4ED8|#1D4ED8|#93C5FD|#93C5FD|
|action.onPrimary|#FFFFFF|#FFFFFF|#172033|#172033|
|action.hover|#1E40AF|#1E40AF|#BFDBFE|#BFDBFE|
|focus.ring|#1D4ED8|#1D4ED8|#93C5FD|#93C5FD|
|status.success|#166534|#166534|#86EFAC|#86EFAC|
|status.warning|#92400E|#92400E|#FDE68A|#FDE68A|
|status.danger|#B91C1C|#B91C1C|#FCA5A5|#FCA5A5|
|status.info|#1E40AF|#1E40AF|#BFDBFE|#BFDBFE|
|diff.addBg|#DCFCE7|#DCFCE7|#163628|#163628|
|diff.removeBg|#FEE2E2|#FEE2E2|#45252D|#45252D|

状态文本使用对应status前景加surface底；diff文字使用text.primary，增删底色外必须标“新增/删除”和符号。primary按钮只用onPrimary文字，disabled不降低至不可读的透明字。错误输入同时边框、图标、文字，不仅红色。选择底色使用subtle加左侧primary标记，不能以与focus相同外框混淆当前选项和键盘焦点。

|尺寸token|冻结值|
|---|---|
|space|4/8/12/16/24/32/48|
|radius|sm4/md8/lg12|
|border.width|1|
|focus.width/offset|2/2，forced-colors时系统Highlight|
|control.height|40；紧凑表格36但命中区≥44×44|
|row.height|常规48，宽松56；内容换行自动增高|
|header.minHeight|56，缩放可增高|
|nav.width/collapsed|224/64|
|drawer.width|480；复杂表单640；窄屏全宽|
|modal.width|确认440，普通640，比较960；不超过视口减32|
|overlay.maxHeight|100dvh减32，移动端减安全区|
|shadow.overlay|0 12px 32px rgba(15,23,42,.18)|
|z|base0/sticky10/dropdown30/drawer50/modal60/notice70/tooltip80|
|motion|进入120ms，布局160ms，退出100ms；reduced-motion全部0|

字体栈 UI：system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif；编辑paper："Noto Serif SC","Songti SC",serif（不要求联网字体）；机器ID：ui-monospace,SFMono-Regular,Consolas,monospace。UI正文16/1.5，小字14/1.5，h1为28/1.3、h2为22/1.4、h3为18/1.4；不将关键错误降至12px。正文默认18px/1.8，用户调16..28px，行高1.5..2.2，正文宽度640..800；偏好不写进正文内容。标题不大写转换机器key。

## 3. 唯一断点与页面重排

|宽度|共用壳|Creator写作特化|Admin表格/表单|
|---|---|---|---|
|≥1440|导航224、边距32，主区max1600|目录240+正文640..800+检查器320，正文优先|完整列，详情抽屉480|
|1200..1439|导航64、边距24|目录240+正文，检查器覆盖抽屉|次要列移详情，保留名/状态/操作|
|768..1199|导航抽屉、边距20|正文单主区，目录/助手互斥抽屉|筛选换行、表单单/双列按字段长度|
|480..767|导航抽屉、边距16|正文100%，工具条换行，目录全高抽屉|表格局部横滚或语义卡片，同权限动作|
|320..479|导航抽屉、边距12|正文100%，检查器全屏，保存条不遮输入|表单一列，详情全屏，错误摘要常驻内容顶部|

不再设置独立1280/960写作断点。写作内容在小屏取消640下限而采用100%宽；横向滚动只允许代码/确需比较表格的局部区域，不允许整个页面溢出。200%缩放仍可操作；400%或等效320 CSS px下内容重排，非二维信息无横向滚动。底部sticky操作条预留等高padding与safe-area，焦点滚动目标不被顶栏/浮层遮挡。

桌面工作台：header为书名/章名/保存状态/主保存；左结构与资料；中textarea；右AI/上下文；底状态为本地字数、服务器保存时间、恢复保护状态。专注模式只隐藏两侧与应用导航，保留保存、退出专注、错误和恢复警告。Admin header保持端别/当前租户/身份，页首单一主操作、筛选区、状态条、表/图和详情；不把多个危险按钮并排当主操作。

## 4. 控件组合与语义

|控件|固定行为|禁止行为|
|---|---|---|
|Button/ActionGroup|每视区一个主操作，异步pending禁重复，说明按钮名与副作用|仅图标无名称、loading期间仍重复提交|
|Field|label绑定id，帮助/错误通过aria-describedby，required文本说明|placeholder替代label、只靠颜色报错|
|SchemaForm|按Schema格式控件，错误摘要可跳字段；嵌套fieldset/legend|未知schema静默当字符串、自动丢弃多余字段后提交|
|Table|caption或区域名、th scope、排序aria-sort、分页说明已加载数|把未知总数显示0、假全选未加载全部数据|
|Tree|roving tabindex，上下选择、左右展开/父子、Home/End，明确层级|只有拖拽操作、tab进入每个装饰节点|
|Tabs|tablist/tab/tabpanel，箭头切焦点，Enter选择|路由变化但不更新aria-selected|
|Dialog/Drawer|role dialog、labelledby、焦点锁、Esc、关闭回触发源；危险默认取消|背景仍可点击、丢稿不确认、不可退出焦点陷阱|
|Graph/Chart|有标题/摘要/等价表，同选择与动作权限|颜色唯一编码、仅Canvas可编辑|
|DiffView|base/current/proposed标明来源修订，新增删除文字标识|红绿唯一差异信号、任一侧失败显示空文本|
|EmptyState|只有成功空数据，说明当前筛选，给合法下一步|权限错误冒充空、内置样例业务记录|
|StatusBanner|文字+Lucide，说明范围与恢复按钮|Toast是唯一错误反馈、断SSE冒充任务失败|

输入校验先本地Ajv，服务端422仍映射field_errors到JSON Pointer；跨字段错误放组级，未知字段错误放摘要。客户端不清洗掉合法空行/缩进，标题trim明确可见；数字空与0区分，金额按Schema十进制字符串处理。密码允许粘贴、密码管理器、显示/隐藏切换；autocomplete使用语义值，不禁用辅助工具。

## 5. 键盘、读屏与异步内容

Tab/Shift+Tab循DOM顺序，禁止正tabindex；页面最前“跳到主要内容”。路由切换后聚焦h1并播报页名，保留查询刷新期间原焦点；关闭抽屉回触发按钮，按钮因删除消失则回列表标题/相邻项目。Esc只关闭最上层可关闭浮层，再退出专注，不调用CancelRun。

快捷键：Ctrl/Cmd+S仅编辑器保存且阻止浏览器另存；Ctrl/Cmd+Enter提交当前明确表单但IME组合时不触发；Ctrl/Cmd+Z/Shift+Z正文撤销/重做；不劫持浏览器查找、返回、刷新。选区工具条可通过textarea旁固定“选区操作”按钮进入；无选区禁对应动作并说明，不依赖悬浮鼠标。拖拽排序有上移/下移/移入容器对话框等价能力。

run文本增量不逐Token触发aria-live；流区域默认aria-live=off，另一个polite状态区最多每2秒播报状态摘要/收到新内容，用户主动选择朗读结果。run终态、保存失败、审批到期播报一次，seq去重也防重复播报；assertive仅用需立即处理且不可忽略的会话失效/危险错误。用户离开底部后停止自动滚动，显示“有新内容”按钮；重新回底或点击才跟随。

loading使用aria-busy；骨架对读屏隐藏并保留加载文本；无法访问页面一次性说明，不把权限轮询反复播报。离线与stale分别文案；partial列出失败分区，聚焦重试不丢其他区域输入。审批倒计时不每秒播报，临近到期与已到期各一次；服务端判定优先于本机时钟。

## 6. 无障碍、安全显示与国际化

验收目标WCAG2.2 AA：普通文本≥4.5:1，大字≥3:1，识别控件/焦点≥3:1；所有四theme实际渲染测量，不能仅凭hex清单宣称通过。禁用控件仍有可读原因；触摸目标≥44×44，目标不重叠。prefers-reduced-motion禁流式光标闪烁/曲线动画；不超过每秒3次闪烁；forced-colors保留边框和系统焦点。

API内容、Markdown产物、文件名与工具返回按文本显示；需要富展示时限定安全组件映射，不使用未消毒HTML，不执行链接内脚本。外链有可访问提示和noopener/noreferrer。正文/口令/Key/完整参数不进入Toast、URL、遥测、复制诊断包；诊断复制仅code/request_id/trace_id/operation_id/UTC时间/操作名。多行用户文本可以含完整Unicode，UI自身无emoji图标。

UI时间固定Asia/Shanghai，显示可理解日期与时区标识，机器ISO时间可复制但不混用本地系统时区；相对时间附精确时间tooltip及读屏文本。字数=Unicode码点且不含空白，输入框限制不得使用UTF16 length误判；金额/百分比使用Intl并保持Schema精度，未知显示“暂无数据”，零仅用于真实0。

翻译命名按common/auth/creator.content/creator.outline/admin.llm/admin.platform/errors命名空间。后端message_key只选择已有错误词条；参数内容按文本转义；英文长度增长30%后按钮可换行，不裁掉唯一动作。持久语言/主题/字号只属于本端偏好，不共享session或正文缓存。

## 7. 可复现验收追踪

|编号|功能→组件→能力→数据Owner|通过证据|
|---|---|---|
|DS01|双端主题→ConfigProvider/ThemeProvider→本地视觉偏好→UI|四theme320/768/1200/1440截图及对比度值；没有另一端菜单|
|DS02|身份页→AuthForm/PageGate→Auth.Login/Refresh/Logout→platform|键盘完成登录、正确aria错误、刷新无localStorage token|
|DS03|写作→PlainTextEditor/SaveIndicator→Content.SaveBody→content|IME不误提交，Ctrl+S单写，读屏失败播报一次，400%可编辑|
|DS04|配置→SchemaForm→各域Update→对应域|required/嵌套/422均聚焦正确字段，0不变空|
|DS05|生成→RunInspector→Agent.GetEvents→agent|大量delta不持续打断读屏，离底不抢滚动，键盘显式取消|
|DS06|图谱→RelationGraph+Table→Worldview.SaveRelation→worldview|无鼠标创建/删除同一边，表/图一致，过滤范围有说明|
|DS07|审批→ApprovalCard→Agent.ApproveApproval/RejectApproval→agent|焦点/风险摘要/到期提示完整，无代批入口，Esc不批不取消run|
|DS08|恢复→RecoveryBanner/DiffView→Content.GetBody/SaveBody→content|冲突三份文本可键盘比较，offline可下载，警告不只Toast|
|DS09|审计→SafeDiff/Table→Platform.ListAudit→platform|读屏列标题与排序正确，导出无正文/凭据，审计无删除动作|
|DS10|状态→PageGate/ErrorPresenter→全部域读取→各Owner|每页八态独立演示，error不变empty，partial不清成功区|

实施验收使用Chrome键盘与NVDA/Windows读屏、浏览器200%/400%缩放、系统高对比与reduced-motion；另以触屏尺寸验证命中目标。每条记录真实页面、主题、viewport、操作、期待/实际、截图或读屏记录，发现失败即修正样式/语义，不以组件库默认可访问性作为替代证据。R1文档冻结不等于这些运行验收已经执行。
