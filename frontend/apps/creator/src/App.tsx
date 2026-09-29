import React, { useState, useEffect, useCallback } from "react";
import { ThemeProvider } from "@inklore/ui";
import { I18nProvider } from "@inklore/i18n";
import { NavigationRail, NavView } from "./components/NavigationRail";
import { TopNavbar } from "./components/TopNavbar";
import { HomeRoute } from "./routes/HomeRoute";
import { EditorRoute } from "./routes/EditorRoute";
import { OutlineRoute } from "./routes/OutlineRoute";
import { WorldviewRoute } from "./routes/WorldviewRoute";
import { AnalysisRoute } from "./routes/AnalysisRoute";
import { AgentNovelGuideRoute } from "./routes/AgentNovelGuideRoute";
import { CreatorLoginRoute } from "./routes/CreatorLoginRoute";
import { CreatorSessionProvider, useCreatorSession } from "./context/CreatorSessionContext";

import { AUTH_VIEWS, type CreatorView, authPath, creatorPath, navigateCreator, normalizeCreatorLocation, readCreatorLocation, subscribeCreatorLocation } from "./navigation";
export type { CreatorView } from "./navigation";

function CreatorContent() {
  const { isAuthenticated, snapshot } = useCreatorSession();
  const [route, setRoute] = useState(() => { normalizeCreatorLocation(); return readCreatorLocation(); });
  const { view: currentView, returnTo, novelId: activeNovelId } = route;
  const [homeIdea, setHomeIdea] = useState("");
  const [navCollapsed, setNavCollapsed] = useState(false);
  const [activeMode, setActiveMode] = useState<"outline" | "writing" | "zen">("writing");

  useEffect(() => subscribeCreatorLocation(() => setRoute(readCreatorLocation())), []);

  const navigateTo = useCallback((view: CreatorView, target?: string, replace = false) => {
    navigateCreator(AUTH_VIEWS.includes(view) ? authPath(view, target) : creatorPath(view), replace);
  }, []);

  // 核心鉴权守卫：当未登录且尝试访问受保护页面时，强重定向至登录页并保留 returnTo 目标
  useEffect(() => {
    if (snapshot.status === "bootstrapping") return;

    if (!isAuthenticated) {
      if (currentView !== "home" && !AUTH_VIEWS.includes(currentView)) {
        navigateTo("login", window.location.pathname + window.location.search, true);
      }
    } else {
      if (currentView === "editor" && !activeNovelId) navigateCreator("/", true);
      // 认证完成后替换登录历史项，返回经过白名单检查的来源路径。
      if (AUTH_VIEWS.includes(currentView)) {
        navigateCreator(returnTo || "/", true);
      }
    }
  }, [isAuthenticated, snapshot.status, currentView, returnTo, activeNovelId, navigateTo]);

  const handleOpenNovel = (novelId: string) => {
    navigateCreator(`/editor?${new URLSearchParams({ novelId })}`);
  };

  // 首页公开可见；个人数据由首页内部按当前会话加载。
  if (currentView === "home") {
    return <HomeRoute onOpenNovel={handleOpenNovel} onNavigate={navigateTo} idea={homeIdea} onIdeaChange={setHomeIdea} />;
  }

  // 会话初始化引导中：展示居中优雅加载指示器，避免未鉴权闪烁
  if (snapshot.status === "bootstrapping") {
    return (
      <div className="h-screen w-screen flex flex-col items-center justify-center bg-neutral-50 dark:bg-neutral-950 text-neutral-600 dark:text-neutral-400">
        <div className="w-8 h-8 border-2 border-amber-500 border-t-transparent rounded-full animate-spin" />
        <p className="mt-4 text-xs font-mono tracking-wider">Loading...</p>
      </div>
    );
  }

  // 未登录或处于鉴权路由：直接渲染全屏鉴权页面 (login / register / forgot-password)
  if (!isAuthenticated || AUTH_VIEWS.includes(currentView)) {
    const authMode = (AUTH_VIEWS.includes(currentView) ? currentView : "login") as "login" | "register" | "forgot-password";
    return (
      <CreatorLoginRoute
        mode={authMode}
        onNavigateMode={(m) => navigateTo(m, returnTo)}
      />
    );
  }

  if (currentView === "agent-guide") {
    return <AgentNovelGuideRoute initialIdea={homeIdea} onBack={() => navigateTo("home")} onCreated={novel => handleOpenNovel(novel.id)} />;
  }

  // 当处于编辑器视图时，彻底独立全屏，绝不渲染外层导航侧栏与外层顶栏！
  if (currentView === "editor" && activeNovelId) {
    return (
      <EditorRoute
        key={`${snapshot.sessionEpoch}:${activeNovelId}`}
        novelId={activeNovelId}
        onBack={() => navigateTo("home")}
      />
    );
  }


  return (
    <div className="flex h-screen w-screen overflow-hidden bg-neutral-50 dark:bg-neutral-950 select-none">
      {/* Left Navigation Rail */}
      <NavigationRail
        currentView={currentView as NavView}
        onSelectView={(v) => {
          navigateTo(v);
          if (v === "outline") setActiveMode("outline");
        }}
        collapsed={navCollapsed}
        onToggleCollapse={() => setNavCollapsed(!navCollapsed)}
      />

      {/* Main Content Pane */}
      <div className="flex-1 flex flex-col min-w-0 h-full overflow-hidden">
        <TopNavbar
          novelTitle="墨境创作者工作台"
          chapterTitle="书架总览"
          wordCount={0}
          isSaved={true}
          activeMode={activeMode}
          onChangeMode={(m) => {
            setActiveMode(m);
            if (m === "outline") navigateTo("outline");
          }}
        />

        <div className="flex-1 flex overflow-hidden">
          {currentView === "outline" && <OutlineRoute novelId={activeNovelId} onOpenChapter={(chapterId) => navigateCreator(`/editor?${new URLSearchParams({ novelId: activeNovelId || "", chapterId })}`)} />}
          {currentView === "worldview" && <WorldviewRoute novelId={activeNovelId} />}
          {currentView === "analysis" && <AnalysisRoute novelId={activeNovelId} />}
        </div>
      </div>
    </div>
  );
}
export function CreatorApp() {
  return (
    <I18nProvider>
    <ThemeProvider audience="creator" defaultMode="light">
      <CreatorSessionProvider>
        <CreatorContent />
      </CreatorSessionProvider>
    </ThemeProvider>
    </I18nProvider>
  );
}
