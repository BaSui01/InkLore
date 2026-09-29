import { AlertCircle, RefreshCw } from "lucide-react";
import { ApiError } from "@inklore/contracts";
import { useI18n } from "@inklore/i18n";
import { Button } from "@inklore/ui";

export function HomeNotice({ error, stale, retry, busy }: {
  error: unknown; stale?: boolean; retry?: () => void; busy?: boolean;
}) {
  const { t } = useI18n();
  const apiError = error instanceof ApiError ? error : null;
  const denied = apiError && [403, 404].includes(apiError.httpStatus);
  return <div className="home-notice" role="alert">
    <AlertCircle size={20} aria-hidden="true" />
    <div><strong>{t(denied ? "home.error.denied" : stale ? "home.error.stale" : "home.error.title")}</strong>
      <p>{t("home.error.hint")}</p>
      {apiError && <small>{apiError.code}{apiError.request_id && ` · ${apiError.request_id}`}</small>}
    </div>
    {retry && !denied && <Button type="button" className="home-button home-button-secondary" onClick={retry} disabled={busy} icon={<RefreshCw size={16} />}>{t("home.retry")}</Button>}
  </div>;
}
