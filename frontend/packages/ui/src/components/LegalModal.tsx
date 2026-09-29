/**
 * @inklore/ui - 服务条款与隐私政策弹窗 (LegalModal)
 * 纯净排版与零 Emoji 铁律，全面国际化
 * 遵循设计系统语义 Token，自适应亮暗主题
 * 包含出版级长篇小说 AI 协同创作与模型治理平台的完整法律条款
 */

import React, { useState, useEffect } from "react";
import { Modal } from "./Modal";
import { Button } from "./Button";
import { useTranslation } from "@inklore/i18n";
import { Shield, FileText, Lock, CheckCircle2 } from "lucide-react";

export type LegalTab = "terms" | "privacy";

export interface LegalModalProps {
  open: boolean;
  onClose: () => void;
  defaultTab?: LegalTab;
  onAgree?: () => void;
}

export function LegalModal({
  open,
  onClose,
  defaultTab = "terms",
  onAgree,
}: LegalModalProps) {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState<LegalTab>(defaultTab);

  useEffect(() => {
    if (open) {
      setActiveTab(defaultTab);
    }
  }, [open, defaultTab]);

  const handleAgree = () => {
    onAgree?.();
    onClose();
  };

  const termsContent = (
    <div className="space-y-6 text-xs sm:text-sm text-[var(--text-secondary)] leading-relaxed">
      <div className="p-4 rounded-xl bg-[var(--bg-subtle)] border border-[var(--border-decorative)] text-[var(--text-secondary)] text-xs">
        <div className="font-semibold flex items-center gap-2 mb-1 text-[var(--text-primary)]">
          <Shield className="w-4 h-4 text-[var(--action-primary)] shrink-0" />
          <span>核心知识产权声明：作者享有 100% 创作资产所有权</span>
        </div>
        <p className="leading-relaxed">
          InkLore 平台恪守创作者第一原则。您在平台上构思、撰写的大纲、角色档案、世界观设定、章节正文及经由 AI 协同生成的文学作品，其全部知识产权归作者本人所有。平台绝不擅自使用您的私有创作手稿训练任何公开基础模型。
        </p>
      </div>

      <section className="space-y-2">
        <h4 className="text-sm sm:text-base font-bold text-[var(--text-primary)] flex items-center gap-2">
          <span className="w-6 h-6 rounded-lg bg-[var(--bg-subtle)] border border-[var(--border-decorative)] flex items-center justify-center text-xs font-mono text-[var(--action-primary)] shrink-0">01</span>
          一、协议范围与账号服务规范
        </h4>
        <p>
          1.1 本协议是创作者/用户（以下简称“您”）与 InkLore 平台（以下简称“平台”）之间关于使用平台所提供的长篇小说 AI 辅助创作、世界观设定、大纲规划及模型治理服务所订立的有效合约。
        </p>
        <p>
          1.2 您在注册时须提供真实、准确的邮箱与笔名。笔名一经注册不可冒用他人知名作家身份。账号凭据由您妥善保管，因自身保管不当造成的会话泄露与数据变动，由用户自行承担责任。
        </p>
        <p>
          1.3 平台严格物理隔离创作者端（Creator Studio）与系统管理端（Admin Console）。创作者端仅面向创作者本人的创作资产，严禁尝试越权访问管理端接口或尝试绕过租户身份认证。
        </p>
      </section>

      <section className="space-y-2">
        <h4 className="text-sm sm:text-base font-bold text-[var(--text-primary)] flex items-center gap-2">
          <span className="w-6 h-6 rounded-lg bg-[var(--bg-subtle)] border border-[var(--border-decorative)] flex items-center justify-center text-xs font-mono text-[var(--action-primary)] shrink-0">02</span>
          二、AI 协同创作与版权归属保障
        </h4>
        <p>
          2.1 <strong>作品所有权：</strong>您在平台创建的全部小说、分卷、章节正文、大纲脉络、角色卡片及世界观事实图谱，其著作权完全属于您。平台不享有您作品的任何版权、改编权或衍生权益。
        </p>
        <p>
          2.2 <strong>模型生成物权利：</strong>您通过平台智能体（Agent）及模型调度网关协同生成的剧情片段、改写建议及灵感卡片，经您确认采纳并纳入章节正文后，视为您的创作组成部分，版权完全归您所有。
        </p>
        <p>
          2.3 <strong>手稿隐私与模型训练隔离：</strong>平台承诺，绝不会将您的私有小说正文、大纲与设定数据作为公开基础模型的通用训练语料。模型调用采用请求级实时推理，上下文在生成完成后立即释放。
        </p>
      </section>

      <section className="space-y-2">
        <h4 className="text-sm sm:text-base font-bold text-[var(--text-primary)] flex items-center gap-2">
          <span className="w-6 h-6 rounded-lg bg-[var(--bg-subtle)] border border-[var(--border-decorative)] flex items-center justify-center text-xs font-mono text-[var(--action-primary)] shrink-0">03</span>
          三、内容合规与出版级安全准则
        </h4>
        <p>
          3.1 您承诺在使用本平台创作内容时，严格遵守国家法律法规、互联网信息管理规定及社会主义核心价值观，不得创作、存储或传播含有违法违规、暴力、侵权或违背公序良俗的内容。
        </p>
        <p>
          3.2 平台内置出版级合规与质检模块（Compliance Domain），对涉嫌违禁词汇或敏感事实进行智能预警与改写建议。合规模块仅提供检测与净化建议，正文修改与发布落库均由作者最终确认。
        </p>
        <p>
          3.3 严禁利用平台提供的智能体或模型调度接口实施自动化批量灌水、抄袭洗稿或发起针对第三方系统的不当攻击。
        </p>
      </section>

      <section className="space-y-2">
        <h4 className="text-sm sm:text-base font-bold text-[var(--text-primary)] flex items-center gap-2">
          <span className="w-6 h-6 rounded-lg bg-[var(--bg-subtle)] border border-[var(--border-decorative)] flex items-center justify-center text-xs font-mono text-[var(--action-primary)] shrink-0">04</span>
          四、服务可用性与创作资产安全
        </h4>
        <p>
          4.1 <strong>秒级版本暂存与快照比对：</strong>平台编辑器提供自动化增量快照存储与事实账本校验，尽可能降低意外断电或网络波动导致的丢稿风险。
        </p>
        <p>
          4.2 <strong>多格式导出权利：</strong>您拥有随时全量导出自身作品资产（包括正文 TXT/Markdown、大纲树、世界观图谱）的完整权利，平台不设置任何人为数据导出壁垒。
        </p>
        <p>
          4.3 平台尽最大商业努力保障 7x24 小时高可用服务。如遇机房故障、电力中断或不可抗力导致的服务中断，平台将全力抢修并启动灾备恢复机制。
        </p>
      </section>

      <section className="space-y-2">
        <h4 className="text-sm sm:text-base font-bold text-[var(--text-primary)] flex items-center gap-2">
          <span className="w-6 h-6 rounded-lg bg-[var(--bg-subtle)] border border-[var(--border-decorative)] flex items-center justify-center text-xs font-mono text-[var(--action-primary)] shrink-0">05</span>
          五、协议变更与争议解决
        </h4>
        <p>
          5.1 平台保留根据业务演进与法律法规变化对本协议进行适度修订的权利。更新后的条款将公布于平台并在登录或操作界面给予显著提醒。
        </p>
        <p>
          5.2 若您对修改后的条款存有异议，可随时停止使用或申请注销账号；继续使用平台服务即视为您已充分知悉并同意接受修订后的条款。
        </p>
      </section>
    </div>
  );

  const privacyContent = (
    <div className="space-y-6 text-xs sm:text-sm text-[var(--text-secondary)] leading-relaxed">
      <div className="p-4 rounded-xl bg-[var(--bg-subtle)] border border-[var(--border-decorative)] text-[var(--text-secondary)] text-xs">
        <div className="font-semibold flex items-center gap-2 mb-1 text-[var(--text-primary)]">
          <Lock className="w-4 h-4 text-[var(--action-primary)] shrink-0" />
          <span>隐私安全承诺：全链路加密传输与零本地明文存储</span>
        </div>
        <p className="leading-relaxed">
          InkLore 采用端到端数据传输加密（TLS 1.3）与落库加密存储技术。用户身份凭据采用加盐散列保护，认证令牌通过 HttpOnly SameSite=Strict Cookie 维护，严禁前端明文存储任何敏感密钥。
        </p>
      </div>

      <section className="space-y-2">
        <h4 className="text-sm sm:text-base font-bold text-[var(--text-primary)] flex items-center gap-2">
          <span className="w-6 h-6 rounded-lg bg-[var(--bg-subtle)] border border-[var(--border-decorative)] flex items-center justify-center text-xs font-mono text-[var(--action-primary)] shrink-0">01</span>
          一、我们如何收集与使用您的信息
        </h4>
        <p>
          1.1 <strong>注册与认证信息：</strong>当您创建账号时，我们收集您的电子邮箱、笔名及加密后的密码散列。邮箱用于账号登录、密码重置及关键安全验证码投递。
        </p>
        <p>
          1.2 <strong>创作资产数据：</strong>为您提供小说创作与事实维护服务，我们存储您提交的作品元数据、章节正文历史版本、大纲节拍及设定三元组。
        </p>
        <p>
          1.3 <strong>安全与审计日志：</strong>根据网络安全法要求，系统自动记录用户登录 IP（脱敏处理）、操作时间戳（遵循统一北京时间标准）及关键安全事件，用于防御黑客攻击与账户防盗。
        </p>
      </section>

      <section className="space-y-2">
        <h4 className="text-sm sm:text-base font-bold text-[var(--text-primary)] flex items-center gap-2">
          <span className="w-6 h-6 rounded-lg bg-[var(--bg-subtle)] border border-[var(--border-decorative)] flex items-center justify-center text-xs font-mono text-[var(--action-primary)] shrink-0">02</span>
          二、创作者作品数据与 AI 提示词隐私专款
        </h4>
        <p>
          2.1 <strong>多租户物理/逻辑隔离：</strong>平台在数据库层严格遵循租户与用户范围隔离契约（Tenant & User Scope），除您授权的协同成员外，其他任何用户无权读取您的私有创作数据。
        </p>
        <p>
          2.2 <strong>提示词动态裁剪与脱敏：</strong>当通过 Agent 调度大模型时，后端组装器仅提取与当前章节关联的大纲与事实图谱上下文，动态脱敏用户信息后发送至模型网关，绝不附加任何用户个人身份数据。
        </p>
        <p>
          2.3 <strong>模型供应商交互规范：</strong>平台接入的所有主流大模型供应商（OpenAI、DeepSeek、Anthropic 等）均签署企业级数据隐私协议，确保您的推理数据不会被留存用于模型微调或公共分发。
        </p>
      </section>

      <section className="space-y-2">
        <h4 className="text-sm sm:text-base font-bold text-[var(--text-primary)] flex items-center gap-2">
          <span className="w-6 h-6 rounded-lg bg-[var(--bg-subtle)] border border-[var(--border-decorative)] flex items-center justify-center text-xs font-mono text-[var(--action-primary)] shrink-0">03</span>
          三、Cookie 与会话凭据管理
        </h4>
        <p>
          3.1 平台使用必要的 HttpOnly 安全 Cookie 来存储会话刷新令牌（Refresh Token），防止跨站脚本（XSS）窃取凭据。
        </p>
        <p>
          3.2 平台不使用任何第三方跟踪型 Cookie，不向任何第三方商业广告网络提供用户行为画像。
        </p>
      </section>

      <section className="space-y-2">
        <h4 className="text-sm sm:text-base font-bold text-[var(--text-primary)] flex items-center gap-2">
          <span className="w-6 h-6 rounded-lg bg-[var(--bg-subtle)] border border-[var(--border-decorative)] flex items-center justify-center text-xs font-mono text-[var(--action-primary)] shrink-0">04</span>
          四、您的隐私权利与数据可携带性
        </h4>
        <p>
          4.1 <strong>查阅与导出：</strong>您可随时在工作台查阅并导出您的所有小说文件、大纲数据与图谱事实。
        </p>
        <p>
          4.2 <strong>账号注销与彻底删除：</strong>若您决定不再使用 InkLore 服务，您可申请注销账号。注销生效后，系统将依据相关法律规定，彻底删除或匿名化处理您的个人资料与创作手稿。
        </p>
      </section>
    </div>
  );

  return (
    <Modal
      open={open}
      onClose={onClose}
      maxWidth="2xl"
      title={
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-xl bg-[var(--bg-subtle)] border border-[var(--border-decorative)] flex items-center justify-center text-[var(--action-primary)]">
            {activeTab === "terms" ? (
              <FileText className="w-4 h-4" />
            ) : (
              <Lock className="w-4 h-4" />
            )}
          </div>
          <div>
            <div className="text-base font-bold text-[var(--text-primary)]">
              {activeTab === "terms" ? t("auth.terms_title") : t("auth.privacy_title")}
            </div>
            <div className="text-xs text-[var(--text-muted)] font-normal mt-0.5">
              {t("auth.legal_effective_date")}
            </div>
          </div>
        </div>
      }
      footer={
        <div className="flex flex-col-reverse sm:flex-row items-stretch sm:items-center justify-between w-full gap-2.5 sm:gap-4">
          <span className="text-[11px] sm:text-xs text-[var(--text-muted)] text-center sm:text-left">
            {activeTab === "terms" ? t("auth.terms_subtitle") : t("auth.privacy_subtitle")}
          </span>
          <div className="flex items-center justify-end gap-2 w-full sm:w-auto">
            <Button variant="outline" size="sm" onClick={onClose} className="flex-1 sm:flex-initial cursor-pointer">
              {t("auth.legal_close_btn")}
            </Button>
            {onAgree && (
              <Button
                variant="primary"
                size="sm"
                onClick={handleAgree}
                className="flex-1 sm:flex-initial cursor-pointer"
              >
                <CheckCircle2 className="w-4 h-4 mr-1.5" />
                {t("auth.legal_agree_btn")}
              </Button>
            )}
          </div>
        </div>
      }
    >
      {/* 顶部选项卡切换 */}
      <div className="p-1 rounded-xl bg-[var(--bg-subtle)] flex items-center gap-1 mb-4 sticky top-0 z-10 backdrop-blur-md bg-[var(--bg-surface)]/90 border border-[var(--border-decorative)]">
        <button
          type="button"
          onClick={() => setActiveTab("terms")}
          className={`flex-1 py-1.5 rounded-lg text-xs font-semibold transition-all cursor-pointer flex items-center justify-center gap-1.5 ${
            activeTab === "terms"
              ? "bg-[var(--bg-surface)] text-[var(--text-primary)] shadow-xs font-bold"
              : "text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
          }`}
        >
          <FileText className="w-3.5 h-3.5" />
          <span>{t("auth.legal_tab_terms")}</span>
        </button>
        <button
          type="button"
          onClick={() => setActiveTab("privacy")}
          className={`flex-1 py-1.5 rounded-lg text-xs font-semibold transition-all cursor-pointer flex items-center justify-center gap-1.5 ${
            activeTab === "privacy"
              ? "bg-[var(--bg-surface)] text-[var(--text-primary)] shadow-xs font-bold"
              : "text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
          }`}
        >
          <Lock className="w-3.5 h-3.5" />
          <span>{t("auth.legal_tab_privacy")}</span>
        </button>
      </div>

      {/* 文档内容展示 */}
      <div className="pr-1">{activeTab === "terms" ? termsContent : privacyContent}</div>
    </Modal>
  );
}

