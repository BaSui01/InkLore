import { useEffect, useMemo, useState } from "react";
import type { Novel } from "@inklore/contracts";
import { ApiClient, getApiSDK } from "@inklore/transport";
import { useCreatorSession } from "../context/CreatorSessionContext";
import { AgentNovelGuidePage } from "./AgentNovelGuidePage";
import "../components/home/home.css";

export function AgentNovelGuideRoute({ initialIdea, onBack, onCreated }: {
  initialIdea: string;
  onBack: () => void;
  onCreated: (novel: Novel) => void;
}) {
  const { snapshot } = useCreatorSession();
  const client = useMemo(() => {
    const session = getApiSDK().getSession();
    if (!session) throw new Error("Creator session is unavailable");
    return new ApiClient({ session });
  }, [snapshot.sessionEpoch]);
  const [offline, setOffline] = useState(!navigator.onLine);

  useEffect(() => {
    const update = () => setOffline(!navigator.onLine);
    window.addEventListener("online", update);
    window.addEventListener("offline", update);
    return () => { window.removeEventListener("online", update); window.removeEventListener("offline", update); };
  }, []);

  return <AgentNovelGuidePage client={client} initialIdea={initialIdea} offline={offline} onBack={onBack} onCreated={onCreated} />;
}
