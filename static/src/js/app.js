// 画面更新ポーリングの可視性制御（docs/api/endpoints.md §4.5）、トーストの自動消去、アラートのフェード、ボタンのリップル、確認ダイアログ（hx-confirm）とモーダル。
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

  // モーダルは、ネイティブの <dialog>（showModal() でフォーカスの閉じ込め・Esc・::backdrop が付く）に載せる（ガイド O1・O2）。
  // - 外側（スクラム）クリックと Esc は「閉じる」要求。data-dismiss があればそのボタンを押し（ペアリング確認では「破棄」）、
  //   無ければ何も送らず閉じる。
  // - 閉じるときは .is-leaving のフェードアウトが終わる animationend で close() する（reduced-motion は即時）。
  // - data-modal-result のボタンは、その値を returnValue にして閉じる。
  // - markup に open だけが付いた dialog（ペアリング確認）は、読み込み・差し替えのたびにモーダルへ引き上げる。
  const closeModal = (dialog, result = "") => {
    if (!dialog.open || dialog.classList.contains("is-leaving")) {
      return;
    }
    if (prefersReducedMotion()) {
      dialog.close(result);
      return;
    }
    dialog.classList.add("is-leaving");
    const onEnd = (event) => {
      // 中身（スピナー・リップル）の animationend・::backdrop 側の animationend では閉じない。
      if (event.target !== dialog || event.pseudoElement) {
        return;
      }
      dialog.removeEventListener("animationend", onEnd);
      dialog.classList.remove("is-leaving");
      dialog.close(result);
    };
    dialog.addEventListener("animationend", onEnd);
  };

  const dismissModal = (dialog) => {
    const target = dialog.dataset.dismiss
      ? document.querySelector(dialog.dataset.dismiss)
      : null;
    if (target) {
      target.click();
    } else {
      closeModal(dialog);
    }
  };

  const upgradeModals = () => {
    for (const dialog of document.querySelectorAll(
      "dialog[data-modal][open]",
    )) {
      if (!dialog.matches(":modal")) {
        dialog.close();
        dialog.showModal();
      }
    }
  };
  upgradeModals();
  document.addEventListener("htmx:afterSwap", upgradeModals);

  // ダイアログ自体が click の target になるのは、中身の外（::backdrop）か自分の余白を押したとき。座標で外側だけを見分ける。
  document.addEventListener("click", (event) => {
    const result = event.target.closest?.("[data-modal-result]");
    if (result) {
      closeModal(result.closest("dialog"), result.dataset.modalResult);
      return;
    }
    const dialog = event.target;
    if (
      !(dialog instanceof HTMLDialogElement) ||
      !dialog.matches("[data-modal]")
    ) {
      return;
    }
    const rect = dialog.getBoundingClientRect();
    const inside =
      event.clientX >= rect.left &&
      event.clientX <= rect.right &&
      event.clientY >= rect.top &&
      event.clientY <= rect.bottom;
    if (!inside) {
      dismissModal(dialog);
    }
  });

  // Esc。既定の即時 close を止めて、同じ閉じ方（フェードアウト・data-dismiss）に載せる。cancel はバブリングしないのでキャプチャで受ける。
  document.addEventListener(
    "cancel",
    (event) => {
      if (event.target.matches?.("dialog[data-modal]")) {
        event.preventDefault();
        dismissModal(event.target);
      }
    },
    true,
  );

  // hx-confirm は OS・WebView 任せの標準の確認ではなく、確認ダイアログ（#confirm-dialog）で取る（ガイド O2）。
  // 文言は hx-confirm の値、見出しとボタンのラベルは要素の data-confirm-title・data-confirm-label（無ければ既定）。
  // いずれも textContent で入れる。「確定」で閉じたときだけ、止めておいたリクエストを issueRequest(true) で送る。
  const CONFIRM_ID = "confirm-dialog";
  let issuePending = null;
  document.addEventListener("htmx:confirm", (event) => {
    const { question, elt, issueRequest } = event.detail;
    const dialog = document.getElementById(CONFIRM_ID);
    if (!question || !dialog) {
      return;
    }
    event.preventDefault();
    dialog.querySelector("[data-confirm-title]").textContent =
      elt.dataset.confirmTitle || "確認";
    dialog.querySelector("[data-confirm-message]").textContent = question;
    dialog.querySelector("[data-confirm-action]").textContent =
      elt.dataset.confirmLabel || "実行";
    issuePending = issueRequest;
    dialog.returnValue = "";
    dialog.showModal();
  });
  // close はバブリングしないのでキャプチャで受ける。
  document.addEventListener(
    "close",
    (event) => {
      if (event.target.id !== CONFIRM_ID) {
        return;
      }
      const issue = issuePending;
      issuePending = null;
      if (event.target.returnValue === "confirm") {
        issue?.(true);
      }
    },
    true,
  );

  // 表示に戻ったら即時に取得する（NFR-AVL-4）。
  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState === "visible") {
      htmx.trigger(document.body, "refresh");
    }
  });
})();
