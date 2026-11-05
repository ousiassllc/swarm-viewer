// 確認ダイアログ（hx-confirm）とモーダル。app.js と同じく CSP の script-src 'self' で読み込む外部ファイル（インラインではない）。eval は使わない。
// htmx の htmx:confirm・htmx:afterSwap を受けるので、htmx.min.js より後に読み込むこと。
(() => {
  // OS の「視差効果を減らす」設定（ガイド M3）。変わりうるので、使うたびに引く。
  const prefersReducedMotion = () =>
    window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  // 時間は CSS のトークンが唯一の出どころで、JS は数値を持たず読む（ガイド M1）。minify で ms が s になるので単位を見る。app.js の同名の関数と同じ。
  const cssDurationMs = (name) => {
    const root = getComputedStyle(document.documentElement);
    const value = root.getPropertyValue(name);
    return Number.parseFloat(value) * (value.includes("ms") ? 1 : 1000);
  };
  // 退出フェードの animationend が来ないときのフォールバックを、フェードの時間にどれだけ足して待つか。
  const FADE_FALLBACK_MARGIN_MS = 100;

  // モーダルは、ネイティブの <dialog>（showModal() でフォーカスの閉じ込め・Esc・::backdrop が付く）に載せる（ガイド O1・O2）。
  // - 外側（スクラム）クリックと Esc は「閉じる」要求。data-dismiss があればそのボタンを押し（ペアリング確認では「破棄」）、
  //   無ければ何も送らず閉じる。
  // - 閉じるときは .is-leaving のフェードアウトが終わる animationend で close() する（reduced-motion は即時）。
  // - data-modal-result のボタンはその値を returnValue にして閉じる。data-modal-close（ヘッダーの ×）は外側クリック・Esc と同じ要求。
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
    // animationend が来ない場合（アニメーションが走らない・タブが裏で止まる）に備えて、フェードアウトの時間より少し長いタイマーでも閉じる。
    // animationend が先に来たらタイマーは解除する。
    let fallbackTimer;
    const finish = () => {
      clearTimeout(fallbackTimer);
      dialog.removeEventListener("animationend", onEnd);
      dialog.classList.remove("is-leaving");
      dialog.close(result);
    };
    const onEnd = (event) => {
      // 中身（スピナー・リップル）の animationend・::backdrop 側の animationend では閉じない。
      if (event.target !== dialog || event.pseudoElement) {
        return;
      }
      finish();
    };
    dialog.addEventListener("animationend", onEnd);
    fallbackTimer = setTimeout(
      finish,
      cssDurationMs("--motion-fade-out") + FADE_FALLBACK_MARGIN_MS,
    );
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
    const closer = event.target.closest?.("[data-modal-close]");
    if (closer) {
      dismissModal(closer.closest("dialog"));
      return;
    }
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
})();
