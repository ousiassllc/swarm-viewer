// app.js の挙動テスト（`bun test static/src/js`）。leave-fade 拡張（差し替えで消えるアラート・モーダルだけに .is-leaving を付けて swap を遅らせる）・
// 既出アラートの .is-settled・トーストの自動消去・ボタンのリップル・非表示中のポーリング取り消しを、happy-dom 上でスクリプトを実行して確かめる。
import { describe, expect, test } from "bun:test";
import { loadScript } from "./dom_harness.js";

const TOKENS = { "--motion-fade-out": "240ms", "--toast-duration": "3s" };

const load = (html, options = {}) =>
  loadScript("app.js", { html, tokens: TOKENS, ...options });

// htmx:beforeSwap を leave-fade 拡張に渡し、拡張が書き換えた detail を返す。
const beforeSwap = (env, detail) => {
  const full = { shouldSwap: true, ...detail };
  const result = env.extensions["leave-fade"].onEvent("htmx:beforeSwap", {
    detail: full,
  });
  return { detail: full, result };
};

const isLeaving = (el) => el.classList.contains("is-leaving");

const REGION_HTML = `
  <section id="region">
    <p id="a" data-alert>接続できません</p>
    <p id="b" data-alert>認証に失敗しました</p>
  </section>
  <p id="outside" data-alert>範囲外のアラート</p>`;

describe("leave-fade 拡張", () => {
  test("応答に同じ文言があるアラートには付けず、無いアラートにだけ is-leaving を付けて swap を待つ", () => {
    const env = load(REGION_HTML);
    const { document } = env;
    const { detail, result } = beforeSwap(env, {
      target: document.getElementById("region"),
      serverResponse: `<section id="region"><p data-alert> 接続できません </p></section>`,
    });

    expect(result).toBe(true);
    expect(isLeaving(document.getElementById("a"))).toBe(false);
    expect(isLeaving(document.getElementById("b"))).toBe(true);
    // 差し替え先の外は触らない。
    expect(isLeaving(document.getElementById("outside"))).toBe(false);
    expect(detail.swapOverride).toBe("innerHTML swap:240ms");
  });

  test("消えるものが無ければ swap を遅らせない", () => {
    const env = load(REGION_HTML);
    const { detail } = beforeSwap(env, {
      target: env.document.getElementById("region"),
      serverResponse: `<p data-alert>接続できません</p><p data-alert>認証に失敗しました</p>`,
    });

    expect(detail.swapOverride).toBeUndefined();
    expect(isLeaving(env.document.getElementById("a"))).toBe(false);
    expect(isLeaving(env.document.getElementById("b"))).toBe(false);
  });

  test("minify された秒表記（.24s）でも、swap の待ちは同じ 240ms", () => {
    const env = load(REGION_HTML, { tokens: { "--motion-fade-out": ".24s" } });
    const { detail } = beforeSwap(env, {
      target: env.document.getElementById("region"),
      serverResponse: "",
    });

    expect(detail.swapOverride).toBe("innerHTML swap:240ms");
  });

  test("既存の swap の指定は残し、swap: だけを置き換える", () => {
    const env = load(REGION_HTML);
    const { detail } = beforeSwap(env, {
      target: env.document.getElementById("region"),
      serverResponse: "",
      swapOverride: "outerHTML swap:1s",
    });

    expect(detail.swapOverride).toBe("outerHTML swap:240ms");
  });

  test("OOB（hx-swap-oob）の id が指す要素の中のアラートも対象にする", () => {
    const env = load(`
      <main id="main"><p id="m" data-alert>主の文言</p></main>
      <div id="side"><p id="s1" data-alert>残る</p><p id="s2" data-alert>消える</p></div>`);
    beforeSwap(env, {
      target: env.document.getElementById("main"),
      serverResponse: `<p data-alert>主の文言</p><div id="side" hx-swap-oob="true"><p data-alert>残る</p></div>`,
    });

    expect(isLeaving(env.document.getElementById("m"))).toBe(false);
    expect(isLeaving(env.document.getElementById("s1"))).toBe(false);
    expect(isLeaving(env.document.getElementById("s2"))).toBe(true);
  });

  test("応答にモーダルが無ければ開いているモーダルを退出させ、あれば残す", () => {
    const html = `<div id="region"><dialog id="m" data-modal open></dialog></div>`;
    const gone = load(html);
    beforeSwap(gone, {
      target: gone.document.getElementById("region"),
      serverResponse: "<p>ただの本文</p>",
    });
    expect(isLeaving(gone.document.getElementById("m"))).toBe(true);

    const kept = load(html);
    beforeSwap(kept, {
      target: kept.document.getElementById("region"),
      serverResponse: `<dialog data-modal open></dialog>`,
    });
    expect(isLeaving(kept.document.getElementById("m"))).toBe(false);
  });

  test("reduced-motion・swap しない応答・差し替え先が無いときは何もしない", () => {
    const reduced = load(REGION_HTML, { reducedMotion: true });
    const a = beforeSwap(reduced, {
      target: reduced.document.getElementById("region"),
      serverResponse: "",
    });
    expect(a.detail.swapOverride).toBeUndefined();
    expect(isLeaving(reduced.document.getElementById("a"))).toBe(false);

    const env = load(REGION_HTML);
    const noSwap = beforeSwap(env, {
      shouldSwap: false,
      target: env.document.getElementById("region"),
      serverResponse: "",
    });
    expect(noSwap.detail.swapOverride).toBeUndefined();
    expect(isLeaving(env.document.getElementById("a"))).toBe(false);

    expect(beforeSwap(env, { serverResponse: "" }).result).toBe(true);
  });
});

describe("既出アラートの is-settled", () => {
  test("読み込み時から画面にある文言のアラートだけ、差し替え後に is-settled を付ける", () => {
    const env = load(`<div id="box"><p data-alert>既出</p></div>`);
    const { document } = env;
    document.getElementById("box").innerHTML =
      `<p id="old" data-alert>既出</p><p id="new" data-alert>新しい</p>`;
    document.dispatchEvent(new env.window.Event("htmx:afterSwap"));

    expect(
      document.getElementById("old").classList.contains("is-settled"),
    ).toBe(true);
    expect(
      document.getElementById("new").classList.contains("is-settled"),
    ).toBe(false);
  });
});

describe("トースト", () => {
  const TOAST_HTML = `<div id="toasts"><p id="toast">保存しました</p></div>`;
  const swapped = (env) =>
    env.document.dispatchEvent(
      Object.assign(new env.window.Event("htmx:oobAfterSwap"), {
        detail: { target: env.document.getElementById("toasts") },
      }),
    );

  test("--toast-duration（3 秒）後に is-leaving を付け、animationend で消す", () => {
    const env = load(TOAST_HTML);
    swapped(env);
    const [timer] = [...env.timers.pending.values()];
    expect(timer.ms).toBe(3000);

    env.timers.runAll();
    const toast = env.document.getElementById("toast");
    expect(isLeaving(toast)).toBe(true);
    expect(env.document.getElementById("toasts").childElementCount).toBe(1);

    toast.dispatchEvent(new env.window.Event("animationend"));
    expect(env.document.getElementById("toasts").childElementCount).toBe(0);
  });

  test("reduced-motion はフェードせずに消す", () => {
    const env = load(TOAST_HTML, { reducedMotion: true });
    swapped(env);
    env.timers.runAll();

    expect(env.document.getElementById("toasts").childElementCount).toBe(0);
  });

  test("中身が空の差し替えではタイマーを張らない", () => {
    const env = load(`<div id="toasts"></div>`);
    swapped(env);

    expect(env.timers.pending.size).toBe(0);
  });
});

describe("ボタンのリップル", () => {
  const press = (env, button) => {
    const event = new env.window.MouseEvent("pointerdown", {
      bubbles: true,
      clientX: 5,
      clientY: 5,
    });
    button.dispatchEvent(event);
  };

  test("押下でリップルを足し、animationend で消す", () => {
    const env = load(`<button id="b">押す</button>`);
    const button = env.document.getElementById("b");

    press(env, button);
    const ripple = button.querySelector(".ripple");
    expect(ripple).not.toBeNull();
    expect(ripple.getAttribute("aria-hidden")).toBe("true");
    // 位置と大きさは CSSOM で決める（style 属性をマークアップに書かない）。
    expect(ripple.style.left).not.toBe("");

    ripple.dispatchEvent(new env.window.Event("animationend"));
    expect(button.querySelector(".ripple")).toBeNull();
  });

  test("無効なボタン・reduced-motion では付けない", () => {
    const disabled = load(`<button id="b" disabled>押す</button>`);
    press(disabled, disabled.document.getElementById("b"));
    expect(disabled.document.querySelector(".ripple")).toBeNull();

    const reduced = load(`<button id="b">押す</button>`, {
      reducedMotion: true,
    });
    press(reduced, reduced.document.getElementById("b"));
    expect(reduced.document.querySelector(".ripple")).toBeNull();
  });
});

describe("非表示中のポーリング", () => {
  const request = (env, path) => {
    const event = new env.window.CustomEvent("htmx:beforeRequest", {
      bubbles: true,
      cancelable: true,
      detail: { pathInfo: { requestPath: path } },
    });
    env.document.body.dispatchEvent(event);
    return event.defaultPrevented;
  };

  test("非表示のあいだ、snapshot の取得だけを取り消す", () => {
    const env = load("");
    Object.defineProperty(env.document, "visibilityState", {
      value: "hidden",
    });

    expect(request(env, "/connections/1/snapshot?x=1")).toBe(true);
    expect(request(env, "/connections/1/sessions")).toBe(false);
  });

  test("表示中は取り消さない", () => {
    const env = load("");
    Object.defineProperty(env.document, "visibilityState", {
      value: "visible",
    });

    expect(request(env, "/connections/1/snapshot")).toBe(false);
  });
});
