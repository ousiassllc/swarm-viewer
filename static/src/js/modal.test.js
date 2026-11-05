// modal.js の挙動テスト（`bun test static/src/js`）。確認ダイアログ（htmx:confirm）・Esc・外側クリック・data-dismiss・退出フェードを、
// happy-dom 上でスクリプトを実行して確かめる。ソースの文字列は見ない。
import { describe, expect, test } from "bun:test";
import { loadScript } from "./dom_harness.js";

const CONFIRM_HTML = `
  <button id="delete" data-confirm-title="接続を削除" data-confirm-label="削除する">削除</button>
  <dialog id="confirm-dialog" data-modal>
    <header>
      <h2 data-confirm-title>確認</h2>
      <button type="button" data-modal-close>×</button>
    </header>
    <p data-confirm-message></p>
    <footer>
      <button type="button" id="cancel" data-modal-result="cancel">キャンセル</button>
      <button type="button" id="confirm" data-modal-result="confirm" data-confirm-action>実行</button>
    </footer>
  </dialog>`;

const TOKENS = { "--motion-fade-out": "240ms" };

// hx-confirm の要素から htmx:confirm を起こす。返すのは preventDefault されたか（htmx が標準の confirm を出さないか）と、issueRequest の呼び出し記録。
const requestConfirm = (env, question = "本当に削除しますか？") => {
  const issued = [];
  const event = new env.window.CustomEvent("htmx:confirm", {
    bubbles: true,
    cancelable: true,
    detail: {
      question,
      elt: env.document.getElementById("delete"),
      issueRequest: (...args) => issued.push(args),
    },
  });
  env.document.getElementById("delete").dispatchEvent(event);
  return { prevented: event.defaultPrevented, issued };
};

// 退出フェードの animationend（dialog 自身が起こしたもの）。
const endFade = (env, dialog) =>
  dialog.dispatchEvent(new env.window.Event("animationend", { bubbles: true }));

const pressEsc = (env, dialog) => {
  const cancel = new env.window.Event("cancel", { cancelable: true });
  dialog.dispatchEvent(cancel);
  return cancel.defaultPrevented;
};

const confirmEnv = (options = {}) =>
  loadScript("modal.js", { html: CONFIRM_HTML, tokens: TOKENS, ...options });

describe("確認ダイアログ（htmx:confirm）", () => {
  test("標準の確認を止め、文言を textContent で入れ、リクエストはまだ送らない", () => {
    const env = confirmEnv();
    const { prevented, issued } = requestConfirm(env, "<b>削除</b>しますか？");
    const dialog = env.document.getElementById("confirm-dialog");

    expect(prevented).toBe(true);
    expect(dialog.open).toBe(true);
    expect(dialog.querySelector("[data-confirm-title]").textContent).toBe(
      "接続を削除",
    );
    expect(dialog.querySelector("[data-confirm-action]").textContent).toBe(
      "削除する",
    );
    // HTML として解釈しない。
    const message = dialog.querySelector("[data-confirm-message]");
    expect(message.textContent).toBe("<b>削除</b>しますか？");
    expect(message.children.length).toBe(0);
    expect(issued).toEqual([]);
  });

  test("hx-confirm の値が無いときは何もしない（htmx に任せる）", () => {
    const env = confirmEnv();
    const { prevented, issued } = requestConfirm(env, "");

    expect(prevented).toBe(false);
    expect(env.document.getElementById("confirm-dialog").open).toBe(false);
    expect(issued).toEqual([]);
  });

  test("「確定」はフェードアウトが終わって閉じたときに、issueRequest(true) を 1 回だけ呼ぶ", () => {
    const env = confirmEnv();
    const { issued } = requestConfirm(env);
    const dialog = env.document.getElementById("confirm-dialog");

    env.document.getElementById("confirm").click();
    expect(dialog.classList.contains("is-leaving")).toBe(true);
    expect(dialog.open).toBe(true);
    expect(issued).toEqual([]);

    endFade(env, dialog);
    expect(dialog.open).toBe(false);
    expect(issued).toEqual([[true]]);
  });

  test("「確定」は reduced-motion なら即時に閉じて送る", () => {
    const env = confirmEnv({ reducedMotion: true });
    const { issued } = requestConfirm(env);

    env.document.getElementById("confirm").click();
    expect(env.document.getElementById("confirm-dialog").open).toBe(false);
    expect(issued).toEqual([[true]]);
  });

  test("「キャンセル」は何も送らず閉じる", () => {
    const env = confirmEnv();
    const { issued } = requestConfirm(env);
    const dialog = env.document.getElementById("confirm-dialog");

    env.document.getElementById("cancel").click();
    endFade(env, dialog);
    expect(dialog.open).toBe(false);
    expect(dialog.returnValue).toBe("cancel");
    expect(issued).toEqual([]);
  });

  test("Esc は既定の即時 close を止め、何も送らずにフェードして閉じる", () => {
    const env = confirmEnv();
    const { issued } = requestConfirm(env);
    const dialog = env.document.getElementById("confirm-dialog");

    expect(pressEsc(env, dialog)).toBe(true);
    expect(dialog.classList.contains("is-leaving")).toBe(true);
    endFade(env, dialog);
    expect(dialog.open).toBe(false);
    expect(issued).toEqual([]);
  });

  test("ヘッダーの × と外側クリックは「キャンセル」と同じで、何も送らない", () => {
    const env = confirmEnv({ reducedMotion: true });
    const dialog = env.document.getElementById("confirm-dialog");

    const closed = requestConfirm(env);
    dialog.querySelector("[data-modal-close]").click();
    expect(dialog.open).toBe(false);

    const outside = requestConfirm(env);
    // happy-dom の getBoundingClientRect は 0×0 なので、(10, 10) はダイアログの外。
    dialog.dispatchEvent(
      new env.window.MouseEvent("click", {
        bubbles: true,
        clientX: 10,
        clientY: 10,
      }),
    );
    expect(dialog.open).toBe(false);
    expect(closed.issued).toEqual([]);
    expect(outside.issued).toEqual([]);
  });

  test("閉じたあとに別の確認を開いて「確定」しても、前のリクエストは送らない", () => {
    const env = confirmEnv({ reducedMotion: true });
    const dialog = env.document.getElementById("confirm-dialog");

    const first = requestConfirm(env);
    env.document.getElementById("cancel").click();
    const second = requestConfirm(env);
    env.document.getElementById("confirm").click();

    expect(dialog.open).toBe(false);
    expect(first.issued).toEqual([]);
    expect(second.issued).toEqual([[true]]);
  });
});

const DISMISS_HTML = `
  <dialog id="draft" data-modal data-dismiss="#discard" open>
    <header><button type="button" data-modal-close>×</button></header>
    <button type="button" id="discard">破棄</button>
    <button type="button" id="save" data-modal-result="confirm">保存</button>
  </dialog>
  <dialog id="plain" data-modal open></dialog>`;

describe("data-dismiss", () => {
  const countClicks = (env, id) => {
    const counter = { count: 0 };
    env.document
      .getElementById(id)
      .addEventListener("click", () => counter.count++);
    return counter;
  };

  test("Esc・ヘッダーの ×・外側クリックは、data-dismiss が指すボタンを押す", () => {
    const env = loadScript("modal.js", { html: DISMISS_HTML, tokens: TOKENS });
    const dialog = env.document.getElementById("draft");
    const discard = countClicks(env, "discard");

    pressEsc(env, dialog);
    expect(discard.count).toBe(1);

    dialog.querySelector("[data-modal-close]").click();
    expect(discard.count).toBe(2);

    dialog.dispatchEvent(
      new env.window.MouseEvent("click", {
        bubbles: true,
        clientX: 10,
        clientY: 10,
      }),
    );
    expect(discard.count).toBe(3);
  });

  test("data-dismiss の無いモーダルは、Esc でそのまま（フェードして）閉じる", () => {
    const env = loadScript("modal.js", { html: DISMISS_HTML, tokens: TOKENS });
    const plain = env.document.getElementById("plain");

    pressEsc(env, plain);
    expect(plain.classList.contains("is-leaving")).toBe(true);
    endFade(env, plain);
    expect(plain.open).toBe(false);
  });

  test("data-modal-result のボタンは data-dismiss を通らず、その値で閉じる", () => {
    const env = loadScript("modal.js", {
      html: DISMISS_HTML,
      reducedMotion: true,
    });
    const dialog = env.document.getElementById("draft");
    const discard = countClicks(env, "discard");

    env.document.getElementById("save").click();
    expect(dialog.open).toBe(false);
    expect(dialog.returnValue).toBe("confirm");
    expect(discard.count).toBe(0);
  });
});

describe("退出フェード", () => {
  const leave = () => {
    const env = confirmEnv();
    requestConfirm(env);
    const dialog = env.document.getElementById("confirm-dialog");
    env.document.getElementById("cancel").click();
    return { env, dialog };
  };

  test("animationend が来なくても、--motion-fade-out より少し長いタイマーで閉じる", () => {
    const { env, dialog } = leave();

    const timers = [...env.timers.pending.values()];
    expect(timers).toHaveLength(1);
    // --motion-fade-out（240ms）より長く、ただし余裕は 1 秒未満。
    expect(timers[0].ms).toBeGreaterThan(240);
    expect(timers[0].ms).toBeLessThan(1240);
    expect(dialog.open).toBe(true);

    env.timers.runAll();
    expect(dialog.open).toBe(false);
    expect(dialog.classList.contains("is-leaving")).toBe(false);
  });

  test("minify された秒表記（.24s）でも同じ時間を読む", () => {
    const env = confirmEnv({ tokens: { "--motion-fade-out": ".24s" } });
    requestConfirm(env);
    env.document.getElementById("cancel").click();

    const [timer] = [...env.timers.pending.values()];
    expect(timer.ms).toBeGreaterThan(240);
    expect(timer.ms).toBeLessThan(1240);
  });

  test("animationend が先に来たらタイマーを解除する", () => {
    const { env, dialog } = leave();

    endFade(env, dialog);
    expect(dialog.open).toBe(false);
    expect(env.timers.pending.size).toBe(0);
  });

  test("中身・::backdrop の animationend では閉じない", () => {
    const { env, dialog } = leave();
    const AnimationEvent = env.window.Event;

    const child = dialog.querySelector("[data-confirm-message]");
    child.dispatchEvent(new AnimationEvent("animationend", { bubbles: true }));
    const backdrop = new AnimationEvent("animationend", { bubbles: true });
    backdrop.pseudoElement = "::backdrop";
    dialog.dispatchEvent(backdrop);

    expect(dialog.open).toBe(true);
    expect(dialog.classList.contains("is-leaving")).toBe(true);
  });

  test("退出中にもう一度閉じる操作をしても、タイマーは増えない", () => {
    const { env, dialog } = leave();

    pressEsc(env, dialog);
    env.document.getElementById("cancel").click();
    expect(env.timers.pending.size).toBe(1);
  });
});
