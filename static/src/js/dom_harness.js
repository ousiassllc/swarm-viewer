// app.js・modal.js の挙動テスト（*.test.js。`bun test static/src/js`）の共通部品。ブラウザには載らない（static/dist/ へは配置しない）。
// happy-dom の Window ごとにスクリプトを実行し直す（テストごとに document のリスナーが積み上がらない）。
// タイマーは偽物に差し替え、時間を進めずに「いつ・何 ms で張ったか」を見る。htmx は拡張の登録だけを受ける偽物。
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { Window } from "happy-dom";

const fakeTimers = () => {
  const pending = new Map();
  let nextId = 1;
  return {
    pending,
    setTimeout: (fn, ms) => {
      const id = nextId++;
      pending.set(id, { fn, ms });
      return id;
    },
    clearTimeout: (id) => {
      pending.delete(id);
    },
    // 張られているタイマーを全部、張った順に実行する。
    runAll: () => {
      for (const [id, { fn }] of [...pending]) {
        pending.delete(id);
        fn();
      }
    },
  };
};

// name（static/src/js/ のファイル名）を、html を載せた新しい Window で実行する。
// reducedMotion は prefers-reduced-motion: reduce の値、tokens は :root に置く CSS のカスタムプロパティ。
export const loadScript = (
  name,
  { html = "", reducedMotion = false, tokens = {} } = {},
) => {
  const window = new Window({ url: "http://localhost/" });
  const { document } = window;
  document.body.innerHTML = html;
  for (const [key, value] of Object.entries(tokens)) {
    document.documentElement.style.setProperty(key, value);
  }
  window.matchMedia = () => ({ matches: reducedMotion });

  const timers = fakeTimers();
  const extensions = {};
  const htmx = {
    config: { defaultSwapStyle: "innerHTML" },
    defineExtension: (extName, extension) => {
      extensions[extName] = extension;
    },
    trigger: () => {},
  };

  const source = readFileSync(join(import.meta.dir, name), "utf8");
  new Function(
    "window",
    "document",
    "HTMLDialogElement",
    "DOMParser",
    "getComputedStyle",
    "setTimeout",
    "clearTimeout",
    "htmx",
    source,
  )(
    window,
    document,
    window.HTMLDialogElement,
    window.DOMParser,
    window.getComputedStyle.bind(window),
    timers.setTimeout,
    timers.clearTimeout,
    htmx,
  );
  return { window, document, timers, extensions };
};
