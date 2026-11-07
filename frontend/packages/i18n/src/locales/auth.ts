/**
 * @inklore/i18n - 认证与身份模块词库 (Auth & Identity Dictionary)
 * 纯净排版与零 Emoji 铁律，全代码库严禁出现任何 Unicode Emoji 字符。
 * 遵循现代开源设计规范（登录、注册、找回密码、验证码、第三方登录）。
 */

export const authZhCN: Record<string, string> = {
  "auth.operation_unavailable": "当前操作尚未接通，请稍后重试",
  // 品牌与通用
  "auth.creator_brand_title": "InkLore · 创作者工作台",
  "auth.creator_brand_label": "创作者工作台",
  "auth.admin_brand_label": "系统管理端",
  "auth.creator_brand_subtitle": "长篇小说创作工作台",
  "auth.platform_contract_badge": "InkLore 工作台",
  "auth.system_status_healthy": "系统运行正常",
  "auth.footer_copyright": "© 2026 InkLore. 保留所有权利。",
  "auth.footer_slogan": "长篇小说创作工作台",

  // 页面标题与引导
  "auth.login_title": "欢迎回来",
  "auth.login_subtitle": "登录账号，继续创作。",
  "auth.register_title": "创建新账号",
  "auth.register_subtitle": "创建账号，开始写作。",
  "auth.forgot_title": "找回账号密码",
  "auth.forgot_subtitle": "输入注册邮箱接收验证码以重置密码",

  // 模式切换与标签
  "auth.tab_login": "登录",
  "auth.tab_pwd_login": "密码登录",
  "auth.tab_code_login": "验证码登录",
  "auth.tab_register": "注册",
  "auth.tab_forgot": "找回密码",

  // 表单字段
  "auth.email": "电子邮箱",
  "auth.email_ph": "请输入电子邮箱",
  "auth.login_identifier": "电子邮箱或用户名",
  "auth.login_identifier_ph": "请输入电子邮箱或用户名",
  "auth.login_identifier_required": "请输入有效的电子邮箱地址或管理员用户名 root",
  "auth.email_required": "请输入有效的电子邮箱地址",
  "auth.password": "密码",
  "auth.password_ph": "请输入密码（至少 6 位）",
  "auth.password_required": "密码长度不能少于 6 位",
  "auth.password_confirm": "确认密码",
  "auth.password_confirm_ph": "请再次输入密码",
  "auth.password_mismatch": "两次输入的密码不一致",
  "auth.pen_name": "用户昵称 / 笔名",
  "auth.pen_name_ph": "如：风清扬、南山客",
  "auth.pen_name_required": "请输入用户昵称或笔名",
  "auth.new_password": "设置新密码",
  "auth.new_password_ph": "请输入新密码（至少 6 位）",
  "auth.new_password_confirm": "确认新密码",
  "auth.new_password_confirm_ph": "请再次输入新密码",

  // 交互与链接
  "auth.remember_me": "记住我",
  "auth.forgot_password_link": "忘记密码？",
  "auth.back_to_login": "返回登录",
  "auth.submit_login": "立即登录",
  "auth.submit_register": "立即注册",
  "auth.submit_reset": "确认重置密码",
  "auth.submitting": "正在提交...",
  "auth.has_account": "已有账号？",
  "auth.no_account": "还没有账号？",
  "auth.sign_up_now": "立即注册",
  "auth.sign_in_now": "立即登录",

  // 验证码与安全防护
  "auth.verify_code": "安全验证码",
  "auth.verify_code_optional": "邮箱验证码（可选，填写后验证邮箱）",
  "auth.verify_code_ph": "请输入 6 位验证码",
  "auth.verify_code_required": "请输入有效的验证码",
  "auth.send_code": "获取验证码",
  "auth.resend_code_cooldown": "重新发送 ({0}s)",
  "auth.send_code_success": "邮件服务已接收验证码邮件，验证码 5 分钟内有效。此提示不确认账号是否存在",
  "auth.captcha": "图形验证码",
  "auth.captcha_ph": "请输入图形验证码",
  "auth.captcha_required": "请输入图形验证码",
  "auth.captcha_mismatch": "图形验证码不正确或已失效，请重新输入",
  "auth.captcha_unavailable": "图形验证码加载失败，请点击刷新",
  "auth.email_code_unavailable": "邮箱验证码服务尚未启用，请先使用密码登录",
  "auth.refresh_captcha": "点击更换验证码",
  "auth.terms_title": "InkLore 服务条款",
  "auth.terms_subtitle": "最后更新时间：2026 年 9 月",
  "auth.privacy_title": "InkLore 隐私政策与指引",
  "auth.privacy_subtitle": "最后更新时间：2026 年 9 月",
  "auth.legal_tab_terms": "服务条款",
  "auth.legal_tab_privacy": "隐私政策",
  "auth.legal_agree_btn": "我已阅读并同意",
  "auth.legal_close_btn": "关闭",
  "auth.legal_effective_date": "生效日期：2026年9月20日",

  // 协议与第三方
  "auth.agree_terms": "我已阅读并同意",
  "auth.terms_service": "《服务条款》",
  "auth.and": "和",
  "auth.privacy_policy": "《隐私政策》",
  "auth.terms_required": "请先阅读并同意服务条款与隐私政策",
  "auth.or_oauth": "或通过以下方式快速登录",
  "auth.oauth_github": "GitHub",
  "auth.oauth_google": "Google",
  "auth.oauth_wechat": "微信",
  "auth.oauth_coming_soon": "{0} 登录通道即将开放，请先使用邮箱验证登录",

  // 状态反馈
  "auth.login_success": "登录成功，欢迎回来！",
  "auth.register_success": "注册成功，正在进入工作台...",
  "auth.reset_success": "密码重置成功，请使用新密码登录",
  "auth.logout": "退出登录",
  "auth.user_profile": "个人中心",

  // 密码显示与主题切换
  "auth.show_password": "显示密码",
  "auth.hide_password": "隐藏密码",
  "auth.theme_light": "切换至浅色模式",
  "auth.theme_dark": "切换至深色模式",

  // 安全提示
  "auth.security_notice_admin": "安全提示：本系统受端到端安全审计保护，请妥善保管账号凭据。",
  "auth.security_notice_creator": "创作安全提示：正文版本自动增量暂存，跨章节事实校验，保障创作资产安全。",

  // 管理端专属
  "auth.admin_login_title": "系统管理端登录",
  "auth.admin_login_subtitle": "请输入管理员账号登录。",
  "auth.admin_hero_title": "长篇小说智能协同与模型治理平台",
  "auth.admin_hero_subtitle": "统一调度多模型网关，智能编排长篇创作工作流，提供出版级内容安全与全流程审计保障。",
  "auth.admin_feature_1_title": "高可用模型调度",
  "auth.admin_feature_1_desc": "统一接入主流大模型矩阵，支持智能选路、动态限流与故障自动平滑降级",
  "auth.admin_feature_2_title": "智能体协同引擎",
  "auth.admin_feature_2_desc": "深度理解长篇语境与剧情脉络，支持复杂创作工作流与人机无缝协作",
  "auth.admin_feature_3_title": "出版级合规与审计",
  "auth.admin_feature_3_desc": "提供全流程内容安全审查，操作行为可溯，全面保障创作资产安全",

  // 创作者端专属
  "auth.creator_hero_title": "沉浸式长篇小说智能协同创作工坊",
  "auth.creator_hero_subtitle": "融合大纲剧情蓝图、跨章节事实图谱与出版级正文编辑器，与 AI 协同构筑宏大故事世界。",
  "auth.creator_feature_1_title": "出版级正文编辑器",
  "auth.creator_feature_1_desc": "版本快照比对、差异合并、纯净排版与秒级自动防丢暂存",
  "auth.creator_feature_2_title": "故事大纲与剧情蓝图",
  "auth.creator_feature_2_desc": "总纲/卷纲/章节细纲，伏笔线索追踪与叙事节奏把控",
  "auth.creator_feature_3_title": "设定与跨章节事实图谱",
  "auth.creator_feature_3_desc": "角色关系网络、世界观设定与严格一致的事实账本校验",

  // 认证异常与错误提示
  "errors.auth.invalid_credentials": "邮箱或密码错误，请核对后重试",
  "errors.auth.account_unavailable": "该邮箱已被注册，请直接登录或使用其他邮箱",
  "errors.auth.rate_limited": "请求过于频繁，请稍候再试",
  "errors.auth.rate_limited_retry": "请求过于频繁，请在 {0} 秒后再试",
  "errors.auth.invalid_token": "凭据已失效或已过期，请重新登录",
  "errors.auth.unauthenticated": "未登录或登录状态已失效，请重新登录",
  "errors.auth.forbidden": "无权执行当前操作",
  "errors.auth.network_error": "网络连接失败，请检查网络或后端服务是否正常",
  "errors.auth.validation_failed": "输入项格式不合规（密码需不少于 6 位，邮箱格式需有效）",
  "errors.platform.internal_error": "系统服务发生内部错误，请稍后重试",
  "errors.auth.generic_failed": "认证操作失败，请稍后重试",
};

export const authEnUS: Record<string, string> = {
  "auth.operation_unavailable": "This operation is unavailable. Please try again later.",
  // Brand & Common
  "auth.creator_brand_title": "InkLore · Creator Studio",
  "auth.creator_brand_label": "Creator Studio",
  "auth.admin_brand_label": "System Console",
  "auth.creator_brand_subtitle": "Long-form fiction studio",
  "auth.platform_contract_badge": "InkLore Studio",
  "auth.system_status_healthy": "Systems Operational",
  "auth.footer_copyright": "© 2026 InkLore. All rights reserved.",
  "auth.footer_slogan": "Long-form fiction studio",

  // Page Titles & Guidance
  "auth.login_title": "Welcome Back",
  "auth.login_subtitle": "Sign in to continue writing.",
  "auth.register_title": "Create Account",
  "auth.register_subtitle": "Create an account to begin writing.",
  "auth.forgot_title": "Reset Password",
  "auth.forgot_subtitle": "Enter your email to receive a verification code and reset password",

  // Mode Tabs
  "auth.tab_login": "Sign In",
  "auth.tab_pwd_login": "Password",
  "auth.tab_code_login": "Verification Code",
  "auth.tab_register": "Sign Up",
  "auth.tab_forgot": "Reset Password",

  // Form Fields
  "auth.email": "Email Address",
  "auth.email_ph": "Enter your email address",
  "auth.login_identifier": "Email or username",
  "auth.login_identifier_ph": "Enter your email or username",
  "auth.login_identifier_required": "Enter a valid email address or the Admin username root",
  "auth.email_required": "Please enter a valid email address",
  "auth.password": "Password",
  "auth.password_ph": "Enter password (at least 6 characters)",
  "auth.password_required": "Password must be at least 6 characters",
  "auth.password_confirm": "Confirm Password",
  "auth.password_confirm_ph": "Enter password again",
  "auth.password_mismatch": "Passwords do not match",
  "auth.pen_name": "Display Name / Pen Name",
  "auth.pen_name_ph": "e.g. Arthur Conan, Jane Austen",
  "auth.pen_name_required": "Display name or pen name is required",
  "auth.new_password": "New Password",
  "auth.new_password_ph": "Enter new password (at least 6 characters)",
  "auth.new_password_confirm": "Confirm New Password",
  "auth.new_password_confirm_ph": "Enter new password again",

  // Interactions & Links
  "auth.remember_me": "Remember me",
  "auth.forgot_password_link": "Forgot password?",
  "auth.back_to_login": "Back to Sign In",
  "auth.submit_login": "Sign In",
  "auth.submit_register": "Create Account",
  "auth.submit_reset": "Reset Password",
  "auth.submitting": "Submitting...",
  "auth.has_account": "Already have an account?",
  "auth.no_account": "Don't have an account?",
  "auth.sign_up_now": "Sign Up",
  "auth.sign_in_now": "Sign In",

  // Verification & Captcha
  "auth.verify_code": "Security Code",
  "auth.verify_code_optional": "Email code (optional, verifies your address)",
  "auth.verify_code_ph": "Enter 6-digit code",
  "auth.verify_code_required": "Valid verification code is required",
  "auth.send_code": "Get Code",
  "auth.resend_code_cooldown": "Resend in {0}s",
  "auth.send_code_success": "The mail service accepted the code message. The code expires in 5 minutes. This response does not confirm that an account exists.",
  "auth.captcha": "Security Captcha",
  "auth.captcha_ph": "Enter captcha",
  "auth.captcha_required": "Please enter the captcha",
  "auth.captcha_mismatch": "Incorrect or expired captcha, please try again",
  "auth.captcha_unavailable": "Captcha unavailable. Click to retry.",
  "auth.email_code_unavailable": "Email verification is not configured. Please use password login.",
  "auth.refresh_captcha": "Click to refresh",
  "auth.terms_title": "InkLore Terms of Service",
  "auth.terms_subtitle": "Last updated: September 2026",
  "auth.privacy_title": "InkLore Privacy Policy",
  "auth.privacy_subtitle": "Last updated: September 2026",
  "auth.legal_tab_terms": "Terms of Service",
  "auth.legal_tab_privacy": "Privacy Policy",
  "auth.legal_agree_btn": "I Have Read and Agree",
  "auth.legal_close_btn": "Close",
  "auth.legal_effective_date": "Effective Date: September 20, 2026",

  // Legal & OAuth
  "auth.agree_terms": "I have read and agree to the",
  "auth.terms_service": "Terms of Service",
  "auth.and": "and",
  "auth.privacy_policy": "Privacy Policy",
  "auth.terms_required": "Please agree to the Terms of Service and Privacy Policy",
  "auth.or_oauth": "Or continue with",
  "auth.oauth_github": "GitHub",
  "auth.oauth_google": "Google",
  "auth.oauth_wechat": "WeChat",
  "auth.oauth_coming_soon": "{0} sign-in will be available soon. Please use email.",

  // Status Feedback
  "auth.login_success": "Login successful, welcome back!",
  "auth.register_success": "Registration successful, entering workspace...",
  "auth.reset_success": "Password reset successfully. Please sign in with your new password.",
  "auth.logout": "Log Out",
  "auth.user_profile": "Profile",

  // Password & Theme Toggle
  "auth.show_password": "Show password",
  "auth.hide_password": "Hide password",
  "auth.theme_light": "Switch to Light Mode",
  "auth.theme_dark": "Switch to Dark Mode",

  // Security Notes
  "auth.security_notice_admin": "Security Notice: Protected by end-to-end security audit. Keep your credentials safe.",
  "auth.security_notice_creator": "Creation Security: Automated revision snapshots and fact checking safeguard your creative assets.",

  // Admin Specific
  "auth.admin_login_title": "Admin Console Login",
  "auth.admin_login_subtitle": "Sign in with your administrator account.",
  "auth.admin_hero_title": "Novel AI Collaboration & Model Governance",
  "auth.admin_hero_subtitle": "Unified multi-model gateway dispatching, intelligent creative workflows, publication-grade safety and comprehensive audit trails.",
  "auth.admin_feature_1_title": "High-Availability Model Gateway",
  "auth.admin_feature_1_desc": "Unified access to LLM matrices with smart routing, rate limiting and graceful fallback",
  "auth.admin_feature_2_title": "Agent Orchestration Engine",
  "auth.admin_feature_2_desc": "Context-aware long-form reasoning powering multi-step collaborative creative workflows",
  "auth.admin_feature_3_title": "Publication-Grade Compliance & Audit",
  "auth.admin_feature_3_desc": "Comprehensive content safety inspections and fully traceable audit records",

  // Creator Specific
  "auth.creator_hero_title": "Immersive Long-Form Novel Collaborative Writing Studio",
  "auth.creator_hero_subtitle": "Integrating story outlines, cross-chapter worldview graphs, and publication-grade editor with AI agents.",
  "auth.creator_feature_1_title": "Publication-Grade Editor",
  "auth.creator_feature_1_desc": "Snapshot diffing, pure typography, and automated recovery backup",
  "auth.creator_feature_2_title": "Story Outlines & Blueprints",
  "auth.creator_feature_2_desc": "Arcs, volumes, chapter beats, foreshadowing trackers and narrative pacing",
  "auth.creator_feature_3_title": "Worldview & Fact Consistency",
  "auth.creator_feature_3_desc": "Character relation networks, world lore, and fact validation",

  // Errors
  "errors.auth.invalid_credentials": "Incorrect email or password. Please try again.",
  "errors.auth.account_unavailable": "This email is already registered. Please log in or use another email.",
  "errors.auth.rate_limited": "Too many requests. Please try again later.",
  "errors.auth.rate_limited_retry": "Too many requests. Please try again in {0} seconds.",
  "errors.auth.invalid_token": "Credential is invalid or expired. Please log in again.",
  "errors.auth.unauthenticated": "Unauthenticated or session expired. Please log in again.",
  "errors.auth.forbidden": "Permission denied for this operation.",
  "errors.auth.network_error": "Network connection failed. Please check your connection or backend server.",
  "errors.auth.validation_failed": "Validation failed (password must be at least 6 characters, valid email required).",
  "errors.platform.internal_error": "Internal server error occurred. Please try again later.",
  "errors.auth.generic_failed": "Authentication failed. Please try again later.",
};
