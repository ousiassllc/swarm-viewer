// 画面更新ポーリングの可視性制御（docs/api/endpoints.md §4.5）、トーストの自動消去、アラートのフェード、ボタンのリップル。
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

  // OS の「視差効果を減らす」設定（ガイド M3）。変わりうるので、使うたびに引く。
  const prefersReducedMotion = () =>
    window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  // トーストは 3 秒で消す（ガイド O3）。#toasts は OOB で差し替わる（同時に 1 つ）ので、
  // 差し替えのたびにタイマーを張り直す。中身が空の差し替え（初期状態）では何もしない。
  // htmx:oobAfterSwap の detail.target は差し替え前の（DOM から外れた）要素なので、id の判定にだけ使い、
  // 中身は DOM から引き直した新しい要素で見る。
  // 出現のフェードインは CSS（.fade-in）、退出は .is-leaving のフェードアウト（240ms）が終わる animationend で消す（ガイド M5）。
  // reduced-motion ではアニメーションが走らず animationend が来ないので、すぐ消す。
  const TOAST_ID = "toasts";
  const TOAST_MS = 3000;
  let toastTimer;

  const dismissToast = () => {
    const box = document.getElementById(TOAST_ID);
    const toast = box?.firstElementChild;
    if (!box || !toast) {
      return;
    }
    if (prefersReducedMotion()) {
      box.replaceChildren();
      return;
    }
    toast.classList.add("is-leaving");
    toast.addEventListener(
      "animationend",
      () => {
        // 退出中に次のトーストへ差し替わっていたら、それを消さない。
        if (box.firstElementChild === toast) {
          box.replaceChildren();
        }
      },
      { once: true },
    );
  };

  document.addEventListener("htmx:oobAfterSwap", (event) => {
    if (event.detail.target?.id !== TOAST_ID) {
      return;
    }
    clearTimeout(toastTimer);
    if (document.getElementById(TOAST_ID)?.childElementCount === 0) {
      return;
    }
    toastTimer = setTimeout(dismissToast, TOAST_MS);
  });

  // アラート（[data-alert]）は出現でフェードインする（CSS の .fade-in。ガイド M5）。
  // 画面更新のポーリングは領域ごと差し替えるので、同じ文言のアラートが残っているだけのときは
  // .is-settled を付けて、10 秒ごとにフェードが繰り返されないようにする。
  // 「いま画面にある文言」は htmx:afterSettle のたびに記録し直す（読み込み直後は最初の記録を作る）。
  const alertKey = (el) => el.textContent.trim();
  let shownAlerts = new Set();
  const recordShownAlerts = () => {
    shownAlerts = new Set(
      Array.from(document.querySelectorAll("[data-alert]"), alertKey),
    );
  };
  const settleRepeatedAlerts = () => {
    for (const el of document.querySelectorAll("[data-alert]")) {
      if (shownAlerts.has(alertKey(el))) {
        el.classList.add("is-settled");
      }
    }
  };
  recordShownAlerts();
  document.addEventListener("htmx:afterSwap", settleRepeatedAlerts);
  document.addEventListener("htmx:oobAfterSwap", settleRepeatedAlerts);
  document.addEventListener("htmx:afterSettle", recordShownAlerts);

  // ボタンの押下は小さなリップル（クリック点から円が広がって消える 280ms。ガイド M4）。
  // 円の大きさと位置はクリック点で決まるので CSSOM（element.style）で指定する（CSP の style-src は
  // マークアップ中の style 属性だけを止め、CSSOM の操作は止めない）。見た目は CSS の .ripple。reduced-motion では付けない。
  document.addEventListener("pointerdown", (event) => {
    const button = event.target.closest?.("button");
    if (!button || button.disabled || prefersReducedMotion()) {
      return;
    }
    const rect = button.getBoundingClientRect();
    const size = Math.max(rect.width, rect.height) * 2;
    const ripple = document.createElement("span");
    ripple.className = "ripple";
    ripple.style.width = `${size}px`;
    ripple.style.height = `${size}px`;
    ripple.style.left = `${event.clientX - rect.left - size / 2}px`;
    ripple.style.top = `${event.clientY - rect.top - size / 2}px`;
    ripple.setAttribute("aria-hidden", "true");
    ripple.addEventListener("animationend", () => ripple.remove(), {
      once: true,
    });
    button.append(ripple);
  });

  // 表示に戻ったら即時に取得する（NFR-AVL-4）。
  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState === "visible") {
      htmx.trigger(document.body, "refresh");
    }
  });
})();
