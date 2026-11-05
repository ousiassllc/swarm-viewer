package com.ousiassllc.swarmviewer.ui.main

import com.ousiassllc.swarmviewer.domain.Attention
import com.ousiassllc.swarmviewer.domain.Layout
import com.ousiassllc.swarmviewer.domain.Pane

/**
 * 表示するセッションを選ぶ（デスクトップと同じ規則）。最後に選んだセッション [last] が許可セッション [allowed] にあればそれ、
 * なければ先頭の許可セッション。許可セッションが無ければ null。
 */
fun pickSession(
    allowed: List<String>,
    last: String?,
): String? = allowed.firstOrNull { it == last } ?: allowed.firstOrNull()

/**
 * [layout] に従ってペインを段に分ける。ペインが 0 件、または layout が成立しない（欠落・不整合）ときは null
 * （呼び出し側がペイン順のリストで出す。FR-PANE-1）。
 */
fun gridRows(
    layout: Layout,
    panes: List<Pane>,
): List<List<Pane>>? {
    if (panes.isEmpty() || !layout.valid(panes.size)) return null
    var start = 0
    return layout.cols.map { n -> panes.subList(start, start + n).also { start += n } }
}

/** [attention] のうち、`pane` が [pos] で `kind` が [kinds] のどれかに一致する項目があるか。[pos] が空なら常に false。 */
fun hasAttention(
    attention: List<Attention>,
    pos: String,
    vararg kinds: String,
): Boolean = pos.isNotEmpty() && attention.any { it.pane == pos && it.kind in kinds }

/** 1 エージェント分のログイン関連の要対応（API が返した順）。 */
data class AgentAttention(
    val agent: String,
    val items: List<Attention>,
)

/** 要対応バーの 2 グループ（機能要件 §7）。 */
data class AttentionGroups(
    /** ペイン関連（`pane` が付く項目。`auth_*` を除く）。ペイン位置順（T1…T4、B1…B4）。 */
    val panes: List<Attention>,
    /** エージェント・ログイン関連。エージェント名順で、同じエージェントの項目は 1 ブロックにまとめる。 */
    val agents: List<AgentAttention>,
)

private val authKinds = setOf("auth_expired", "auth_expiring")

/** `auth_expired` / `auth_expiring` か（ペインに紐づけない）。 */
fun isAuthKind(kind: String): Boolean = kind in authKinds

/**
 * `attention[]` を 2 グループに分ける。API が返す順は保証がないので並べ替える（機能要件 §7）。
 * `auth_*` は `pane` に関わらずログイン関連。`pane` の無い項目（未知の `kind` を含む）もログイン関連に入る。
 */
fun groupAttention(attention: List<Attention>): AttentionGroups {
    val (panes, others) = attention.partition { it.pane.isNotEmpty() && !isAuthKind(it.kind) }
    val agents =
        others
            .groupBy { it.detail }
            .map { (agent, items) -> AgentAttention(agent, items) }
            .sortedBy { it.agent }
    return AttentionGroups(panes.sortedWith(compareBy({ panePosKey(it.pane).first }, { panePosKey(it.pane).second }, { it.pane })), agents)
}

private const val RANK_OTHER = 2

/** ペイン位置の並び順のキー（段の順位, 番号）。T → B → その他（番号が読めない位置も含む）の順で、同じ段なら番号順。 */
private fun panePosKey(pos: String): Pair<Int, Int> {
    val rank =
        when (pos.firstOrNull()) {
            'T' -> 0
            'B' -> 1
            else -> return RANK_OTHER to 0
        }
    val number = pos.drop(1).toIntOrNull() ?: return RANK_OTHER to 0
    return rank to number
}
