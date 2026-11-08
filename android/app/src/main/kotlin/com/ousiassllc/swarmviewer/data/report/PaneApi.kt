package com.ousiassllc.swarmviewer.data.report

import com.ousiassllc.swarmviewer.domain.AskCard
import com.ousiassllc.swarmviewer.domain.CloseResult
import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.DEFAULT_TAIL_LINES
import com.ousiassllc.swarmviewer.domain.DirList
import com.ousiassllc.swarmviewer.domain.EscapeResult
import com.ousiassllc.swarmviewer.domain.PaneTail
import com.ousiassllc.swarmviewer.domain.PickResult
import com.ousiassllc.swarmviewer.domain.SendResult
import com.ousiassllc.swarmviewer.domain.StartResult

/**
 * ペインを見る・操作するツールの呼び出し口（実装は [ReportClient]。サービス層はこの interface だけに依存し、テストでは差し替える）。
 * `session` は常に明示し、`pane` は report の `panes[].pos`。失敗は domain の `FetchException`。
 * 操作ツール（`pane_tail`・`ask_card` 以外）は操作の接続（URL の path が `/ops`）だけで、見るだけの接続では通信せず
 * `protocol`（`tool_not_allowed`）。
 */
interface PaneApi {
    /** `pane_tail`: 末尾 [lines] 行（1〜200）。 */
    suspend fun paneTail(
        conn: Connection,
        session: String,
        pane: String,
        lines: Int = DEFAULT_TAIL_LINES,
    ): PaneTail

    /** `ask_card`: 質問カード。 */
    suspend fun askCard(
        conn: Connection,
        session: String,
        pane: String,
    ): AskCard

    /** `ask_pick`（`n`）: 選択肢を番号で選ぶ。 */
    suspend fun askPick(
        conn: Connection,
        session: String,
        pane: String,
        n: Int,
    ): PickResult

    /** `ask_pick`（`text`）: 自由入力で答える。 */
    suspend fun askPick(
        conn: Connection,
        session: String,
        pane: String,
        text: String,
    ): PickResult

    /** `send_text`: 指示を送る。 */
    suspend fun sendText(
        conn: Connection,
        session: String,
        pane: String,
        text: String,
    ): SendResult

    /** `send_escape`: 中断。[confirm] が false なら通信せず `confirm_required`。 */
    suspend fun sendEscape(
        conn: Connection,
        session: String,
        pane: String,
        confirm: Boolean,
    ): EscapeResult

    /** `pane_start`: `shell` のペインでエージェントを起動する。 */
    suspend fun paneStart(
        conn: Connection,
        session: String,
        pane: String,
        agent: String,
        cwd: String,
        title: String,
    ): StartResult

    /** `pane_close`: ペインを終了する。 */
    suspend fun paneClose(
        conn: Connection,
        session: String,
        pane: String,
    ): CloseResult

    /** `dir_list`: フォルダの 1 階層。[path] が null なら `allowed_roots`。 */
    suspend fun dirList(
        conn: Connection,
        path: String? = null,
    ): DirList
}
