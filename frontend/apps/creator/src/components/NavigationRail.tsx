import React from "react";
import { BookOpen, Compass, Feather, BarChart3, Settings, Moon, Sun, ChevronLeft, ChevronRight, User, LogOut } from "lucide-react";
import { useTheme } from "@inklore/ui";
import { useCreatorSession } from "../context/CreatorSessionContext";

export type NavView = "home" | "editor" | "outline" | "worldview" | "analysis";

export interface NavigationRailProps {
  currentView: NavView;
  onSelectView: (view: NavView) => void;
  collapsed: boolean;
  onToggleCollapse: () => void;
}

export function NavigationRail({
  currentView,
  onSelectView,
  collapsed,
  onToggleCollapse,
}: NavigationRailProps) {
  const { mode, setMode } = useTheme();
  const { isAuthenticated, principal, logout, openAuthPage } = useCreatorSession();

  const navItems: { id: NavView; label: string; icon: React.ReactNode }[] = [
    { id: "home", label: "首页", icon: <Compass className="w-5 h-5" /> },
    { id: "editor", label: "写作工作台", icon: <Feather className="w-5 h-5" /> },
    { id: "outline", label: "故事大纲", icon: <BookOpen className="w-5 h-5" /> },
    { id: "worldview", label: "设定与图谱", icon: <Settings className="w-5 h-5" /> },
    { id: "analysis", label: "节奏分析", icon: <BarChart3 className="w-5 h-5" /> },
  ];

  return (
    <aside
      className={`flex flex-col shrink-0 border-r border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 transition-all duration-300 z-30 select-none ${
        collapsed ? "w-16" : "w-52"
      }`}
    >
      {/* Brand Header */}
      <div className="h-14 flex items-center px-4 gap-3 border-b border-neutral-100 dark:border-neutral-800/80">
        <div className="w-8 h-8 rounded-lg bg-amber-600 flex items-center justify-center text-white font-bold shrink-0 shadow-sm">
          <Feather className="w-4 h-4" />
        </div>
        {!collapsed && (
          <span className="font-semibold text-base text-neutral-900 dark:text-neutral-100 truncate">
            墨境 · 创作者
          </span>
        )}
      </div>

      {/* Navigation items */}
      <nav className="flex-1 py-4 px-2 space-y-1 overflow-y-auto">
        {navItems.map((item) => {
          const active = currentView === item.id;
          return (
            <button
              key={item.id}
              onClick={() => onSelectView(item.id)}
              className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-colors ${
                active
                  ? "bg-amber-50 dark:bg-amber-950/40 text-amber-600 dark:text-amber-400 font-semibold"
                  : "text-neutral-600 dark:text-neutral-400 hover:bg-neutral-100 dark:hover:bg-neutral-800 hover:text-neutral-900 dark:hover:text-neutral-100"
              }`}
              title={collapsed ? item.label : undefined}
            >
              <span className="shrink-0">{item.icon}</span>
              {!collapsed && <span className="truncate">{item.label}</span>}
            </button>
          );
        })}
      </nav>

      {/* Bottom Footer Actions */}
      <div className="p-2 border-t border-neutral-100 dark:border-neutral-800/80 space-y-1.5">
        {/* User Account / Login Button */}
        {isAuthenticated ? (
          <div className="flex items-center justify-between px-2.5 py-1.5 rounded-lg bg-neutral-100/70 dark:bg-neutral-800/70 text-xs">
            <div className="flex items-center gap-2 min-w-0">
              <div className="w-6 h-6 rounded-full bg-amber-500/20 text-amber-600 dark:text-amber-400 flex items-center justify-center shrink-0 font-medium">
                <User className="w-3.5 h-3.5" />
              </div>
              {!collapsed && (
                <span className="font-medium text-neutral-800 dark:text-neutral-200 truncate">
                  {principal?.display_name || "墨境作者"}
                </span>
              )}
            </div>
            {!collapsed && (
              <button
                type="button"
                onClick={() => logout()}
                className="text-neutral-400 hover:text-red-500 transition-colors p-1"
                title="退出登录"
              >
                <LogOut className="w-3.5 h-3.5" />
              </button>
            )}
          </div>
        ) : (
          <button
            type="button"
            onClick={() => openAuthPage("login")}
            className="w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium text-amber-600 dark:text-amber-400 hover:bg-amber-50 dark:hover:bg-amber-950/30 transition-colors"
            title="登录 / 注册"
          >
            <User className="w-5 h-5 shrink-0" />
            {!collapsed && <span>登录 / 注册</span>}
          </button>
        )}

        {/* Dark/Light mode toggle */}
        <button
          onClick={() => setMode(mode === "dark" ? "light" : "dark")}
          className="w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm text-neutral-600 dark:text-neutral-400 hover:bg-neutral-100 dark:hover:bg-neutral-800 transition-colors"
          title={mode === "dark" ? "切换到浅色模式" : "切换到深色模式"}
        >
          {mode === "dark" ? (
            <Sun className="w-5 h-5 text-amber-400 shrink-0" />
          ) : (
            <Moon className="w-5 h-5 shrink-0" />
          )}
          {!collapsed && <span>{mode === "dark" ? "浅色模式" : "深色模式"}</span>}
        </button>

        {/* Collapse button */}
        <button
          onClick={onToggleCollapse}
          className="w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm text-neutral-500 hover:bg-neutral-100 dark:hover:bg-neutral-800 transition-colors"
        >
          {collapsed ? (
            <ChevronRight className="w-5 h-5 shrink-0" />
          ) : (
            <>
              <ChevronLeft className="w-5 h-5 shrink-0" />
              <span>收起侧栏</span>
            </>
          )}
        </button>
      </div>
    </aside>
  );
}
