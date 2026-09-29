import { useMemo, useState } from "react";
import { useInfiniteQuery } from "@tanstack/react-query";
import { ApiClient, getApiSDK } from "@inklore/transport";
import { ApiError, type Novel, type PagedList } from "@inklore/contracts";

export type ShelfStatus = "active" | "archived";

export function useHomeBookshelf(enabled: boolean) {
  const session = getApiSDK().getSession()!;
  const client = useMemo(() => new ApiClient({ session }), [session]);
  const [status, setStatus] = useState<ShelfStatus>("active");
  const [search, setSearch] = useState("");
  const query = useInfiniteQuery({
    queryKey: ["creator-home-books", session.getScope(), status],
    enabled,
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ pageParam, signal }) => {
      const response = await client.request<unknown, PagedList<Novel>>({
        operationId: "content_listNovels", method: "GET", path: "/v1/content/novels",
        query: { limit: 12, cursor: pageParam, status }, signal,
      });
      const page = response.data;
      if (!page || !Array.isArray(page.items) || typeof page.has_more !== "boolean" ||
          (page.has_more && !page.next_cursor)) {
        throw new ApiError({ httpStatus: 502, code: "platform.invalid_response" });
      }
      return page;
    },
    getNextPageParam: (page) => page.has_more ? page.next_cursor ?? undefined : undefined,
    retry: false,
    staleTime: 30_000,
  });
  const forbidden = query.error instanceof ApiError && [403, 404].includes(query.error.httpStatus);
  const novels = forbidden ? [] : Array.from(new Map(
    (query.data?.pages.flatMap(page => page.items) ?? []).map(novel => [novel.id, novel]),
  ).values());
  const needle = search.trim().toLocaleLowerCase();
  const filtered = novels.filter(novel => !needle ||
    `${novel.title}\n${novel.category}\n${novel.summary}`.toLocaleLowerCase().includes(needle));
  return { client, query, status, setStatus, search, setSearch, novels, filtered, forbidden };
}
