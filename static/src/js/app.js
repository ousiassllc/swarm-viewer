// 画面更新ポーリングの可視性制御（docs/api/endpoints.md §4.5）と、トーストの自動消去。
// CSP の script-src 'self' で読み込む外部ファイル（インラインではない）。eval は使わない。
// htmx.min.js より後に読み込むこと。
(() => {
  const SNAPSHOT_SUFFIX = "/snapshot";

  // リクエスト先のパス（クエリ・フラグメントを除く）が `…/snapshot` か。
  const isSnapshotRequest = (detail) => {
    const raw =
      detail.pathInfo?.requestPath ?? detail.requestConfig?.path ?? "";
    const path = String(raw).split(/[?#]/, 1)[0];
    return path.endsWith(SNAPSHOT_SUFFIX);
  };

  // 非表示・最小化中は snapshot の取得を取り消す（FR-POLL-3）。
  document.addEventListener("htmx:beforeRequest", (event) => {
    if (
      document.visibilityState === "hidden" &&
      isSnapshotRequest(event.detail)
    ) {
      event.preventDefault();
    }
  });

  // トーストは 3 秒で消す（ガイド O3）。#toasts は OOB で差し替わる（同時に 1 つ）ので、
  // 差し替えのたびにタイマーを張り直す。中身が空の差し替え（初期状態）では何もしない。
  const TOAST_ID = "toasts";
  const TOAST_MS = 3000;
  let toastTimer;
  document.addEventListener("htmx:oobAfterSwap", (event) => {
    const el = event.detail.target;
    if (!el || el.id !== TOAST_ID) {
      return;
    }
    clearTimeout(toastTimer);
    if (el.childElementCount === 0) {
      return;
    }
    toastTimer = setTimeout(() => {
      // OOB で差し替わった後の要素を引き直す（古い参照は DOM から外れている）。
      document.getElementById(TOAST_ID)?.replaceChildren();
    }, TOAST_MS);
  });

  // 表示に戻ったら即時に取得する（NFR-AVL-4）。
  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState === "visible") {
      htmx.trigger(document.body, "refresh");
    }
  });
})();
