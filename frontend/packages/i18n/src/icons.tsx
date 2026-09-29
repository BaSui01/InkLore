/**
 * @inklore/i18n - 统一 Lucide 矢量图标库
 * 严格遵循纯净排版与零 Emoji 铁律，全平台严禁出现 Unicode Emoji 字符。
 */

import React from "react";
import {
  CheckCircle2,
  AlertTriangle,
  AlertCircle,
  Info,
  Loader2,
  Save,
  Edit3,
  Trash2,
  Plus,
  RefreshCw,
  Copy,
  Download,
  Upload,
  X,
  ArrowLeft,
  ArrowRight,
  Search,
  Filter,
  BookOpen,
  FileText,
  Layers,
  Users,
  GitBranch,
  Cpu,
  ShieldCheck,
  Key,
  Database,
  Activity,
  Terminal,
  Lock,
  LogOut,
  Sliders,
  History,
  Check,
  RotateCcw,
  Maximize2,
  Minimize2,
  LucideProps,
} from "lucide-react";

export interface AppIconProps extends LucideProps {
  className?: string;
}

/** 状态图标 */
export const StatusSuccessIcon: React.FC<AppIconProps> = (props) => (
  <CheckCircle2 size={16} aria-hidden="true" {...props} />
);

export const StatusWarningIcon: React.FC<AppIconProps> = (props) => (
  <AlertTriangle size={16} aria-hidden="true" {...props} />
);

export const StatusDangerIcon: React.FC<AppIconProps> = (props) => (
  <AlertCircle size={16} aria-hidden="true" {...props} />
);

export const StatusInfoIcon: React.FC<AppIconProps> = (props) => (
  <Info size={16} aria-hidden="true" {...props} />
);

export const LoadingSpinner: React.FC<AppIconProps> = ({ className, ...props }) => (
  <Loader2 size={16} className={`animate-spin ${className || ""}`} aria-hidden="true" {...props} />
);

/** 动作图标 */
export const ActionSaveIcon: React.FC<AppIconProps> = (props) => <Save size={16} aria-hidden="true" {...props} />;
export const ActionEditIcon: React.FC<AppIconProps> = (props) => <Edit3 size={16} aria-hidden="true" {...props} />;
export const ActionDeleteIcon: React.FC<AppIconProps> = (props) => <Trash2 size={16} aria-hidden="true" {...props} />;
export const ActionCreateIcon: React.FC<AppIconProps> = (props) => <Plus size={16} aria-hidden="true" {...props} />;
export const ActionRefreshIcon: React.FC<AppIconProps> = (props) => <RefreshCw size={16} aria-hidden="true" {...props} />;
export const ActionCopyIcon: React.FC<AppIconProps> = (props) => <Copy size={16} aria-hidden="true" {...props} />;
export const ActionDownloadIcon: React.FC<AppIconProps> = (props) => <Download size={16} aria-hidden="true" {...props} />;
export const ActionUploadIcon: React.FC<AppIconProps> = (props) => <Upload size={16} aria-hidden="true" {...props} />;
export const ActionCloseIcon: React.FC<AppIconProps> = (props) => <X size={16} aria-hidden="true" {...props} />;
export const ActionBackIcon: React.FC<AppIconProps> = (props) => <ArrowLeft size={16} aria-hidden="true" {...props} />;
export const ActionNextIcon: React.FC<AppIconProps> = (props) => <ArrowRight size={16} aria-hidden="true" {...props} />;
export const ActionSearchIcon: React.FC<AppIconProps> = (props) => <Search size={16} aria-hidden="true" {...props} />;
export const ActionFilterIcon: React.FC<AppIconProps> = (props) => <Filter size={16} aria-hidden="true" {...props} />;
export const ActionCheckIcon: React.FC<AppIconProps> = (props) => <Check size={16} aria-hidden="true" {...props} />;
export const ActionRollbackIcon: React.FC<AppIconProps> = (props) => <RotateCcw size={16} aria-hidden="true" {...props} />;
export const ActionLogoutIcon: React.FC<AppIconProps> = (props) => <LogOut size={16} aria-hidden="true" {...props} />;

/** 领域与导航图标 */
export const NovelIcon: React.FC<AppIconProps> = (props) => <BookOpen size={16} aria-hidden="true" {...props} />;
export const ChapterIcon: React.FC<AppIconProps> = (props) => <FileText size={16} aria-hidden="true" {...props} />;
export const VolumeIcon: React.FC<AppIconProps> = (props) => <Layers size={16} aria-hidden="true" {...props} />;
export const CharacterIcon: React.FC<AppIconProps> = (props) => <Users size={16} aria-hidden="true" {...props} />;
export const WorkflowIcon: React.FC<AppIconProps> = (props) => <GitBranch size={16} aria-hidden="true" {...props} />;
export const ModelIcon: React.FC<AppIconProps> = (props) => <Cpu size={16} aria-hidden="true" {...props} />;
export const ShieldIcon: React.FC<AppIconProps> = (props) => <ShieldCheck size={16} aria-hidden="true" {...props} />;
export const KeyIcon: React.FC<AppIconProps> = (props) => <Key size={16} aria-hidden="true" {...props} />;
export const DatabaseIcon: React.FC<AppIconProps> = (props) => <Database size={16} aria-hidden="true" {...props} />;
export const MetricsIcon: React.FC<AppIconProps> = (props) => <Activity size={16} aria-hidden="true" {...props} />;
export const TerminalIcon: React.FC<AppIconProps> = (props) => <Terminal size={16} aria-hidden="true" {...props} />;
export const SettingsIcon: React.FC<AppIconProps> = (props) => <Sliders size={16} aria-hidden="true" {...props} />;
export const HistoryIcon: React.FC<AppIconProps> = (props) => <History size={16} aria-hidden="true" {...props} />;
