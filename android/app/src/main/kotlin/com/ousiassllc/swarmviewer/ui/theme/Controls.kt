package com.ousiassllc.swarmviewer.ui.theme

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.pulltorefresh.PullToRefreshState
import androidx.compose.material3.pulltorefresh.pullToRefreshIndicator
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.unit.dp

/** スイッチ（ガイド `.sw2`）のトラックとつまみの大きさ、つまみの内側の余白。 */
private val SwitchTrackWidth = 36.dp
private val SwitchTrackHeight = 20.dp
private val SwitchThumbSize = 14.dp
private val SwitchThumbInset = 2.dp

/** ラジオ（ガイド `.chk input[type=radio]`）の円と、選択時の中の点の大きさ。 */
private val RadioSize = 18.dp
private val RadioDotSize = 6.dp

/** 設定行（ラジオ・スイッチ）の最小の高さ（ガイド `--list-row-h`）。 */
val ListRowMinHeight = 48.dp

/**
 * スイッチ（ガイド `.sw2`）: 36×20dp・角丸 full のトラックに 14dp のつまみ。
 * オフはトラック accent-soft（surfaceVariant）・1dp の border の枠・つまみ surface、オンはトラック accent（primary）・つまみ bone（background）。
 * Material の `Switch`（52×32dp）はガイドの見本より大きいので、`Box` で組む。M3 の既定はオフのつまみもトラックも border / accent-soft と
 * 同じ値になって溶けるので、色も明示する。独自のモーションは持たない（NFR-DSG-6）。
 * 上下に 6dp の余白を持ち、行の高さは M3 の `Switch`（32dp）と同じ。
 * 操作は呼び出し側の行（toggleable）が受ける。
 */
@Composable
fun AppSwitch(
    checked: Boolean,
    modifier: Modifier = Modifier,
) {
    val scheme = MaterialTheme.colorScheme
    Box(modifier = modifier.padding(vertical = 6.dp)) {
        Box(
            modifier =
                Modifier
                    .size(SwitchTrackWidth, SwitchTrackHeight)
                    .background(if (checked) scheme.primary else scheme.surfaceVariant, CircleShape)
                    .border(1.dp, if (checked) scheme.primary else scheme.outline, CircleShape)
                    .padding(SwitchThumbInset),
            contentAlignment = if (checked) Alignment.CenterEnd else Alignment.CenterStart,
        ) {
            Box(
                modifier =
                    Modifier
                        .size(SwitchThumbSize)
                        .background(if (checked) scheme.background else scheme.surface, CircleShape),
            )
        }
    }
}

/**
 * ラジオ（ガイド `.chk input[type=radio]`）: 18dp の円・1dp の mute（onSurfaceVariant）の枠。選択時は accent（primary）の塗りに 6dp の bone（background）の点。
 * Material の `RadioButton`（2dp の輪）と違う形なので `Box` で組む。周囲に 3dp の余白を持ち、行の高さは M3 の `RadioButton`（24dp）と同じ。
 * 操作は呼び出し側の行（selectable）が受ける。
 */
@Composable
fun AppRadio(
    selected: Boolean,
    modifier: Modifier = Modifier,
) {
    val scheme = MaterialTheme.colorScheme
    Box(modifier = modifier.padding(3.dp)) {
        Box(
            modifier =
                Modifier
                    .size(RadioSize)
                    .background(if (selected) scheme.primary else Color.Transparent, CircleShape)
                    .border(1.dp, if (selected) scheme.primary else scheme.onSurfaceVariant, CircleShape),
            contentAlignment = Alignment.Center,
        ) {
            if (selected) {
                Box(modifier = Modifier.size(RadioDotSize).background(scheme.background, CircleShape))
            }
        }
    }
}

/** 引っ張って更新のインジケーターの外径（M3 の `SpinnerContainerSize` と同じ）と、中の円形進捗の径。 */
private val RefreshIndicatorSize = 40.dp
private val RefreshSpinnerSize = 24.dp

/**
 * 引っ張って更新のインジケーター（ガイド S3・§6: 影はモーダルだけ）: surface の円・1dp の border・影なし・中は primary の円形進捗。
 * M3 の既定（`PullToRefreshDefaults.Indicator`）は引っ張り始めから更新中まで 2 段の影を付けるので、影を 0 にした自前の面に置き換える。
 * 引っ張っている間は進捗が引っ張り量に追従し、更新中は不定の回転になる。`PullToRefreshBox` の `indicator` に渡す。
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun BoxScope.AppPullToRefreshIndicator(
    state: PullToRefreshState,
    isRefreshing: Boolean,
) {
    val scheme = MaterialTheme.colorScheme
    Box(
        modifier =
            Modifier
                .align(Alignment.TopCenter)
                .size(RefreshIndicatorSize)
                .pullToRefreshIndicator(
                    state = state,
                    isRefreshing = isRefreshing,
                    shape = CircleShape,
                    containerColor = scheme.surface,
                    elevation = 0.dp,
                ).border(1.dp, scheme.outline, CircleShape),
        contentAlignment = Alignment.Center,
    ) {
        val spinnerModifier = Modifier.size(RefreshSpinnerSize)
        if (isRefreshing) {
            CircularProgressIndicator(
                modifier = spinnerModifier,
                color = scheme.primary,
                strokeWidth = 2.dp,
                trackColor = Color.Transparent,
                strokeCap = StrokeCap.Round,
            )
        } else {
            CircularProgressIndicator(
                progress = { state.distanceFraction.coerceIn(0f, 1f) },
                modifier = spinnerModifier,
                color = scheme.primary,
                strokeWidth = 2.dp,
                trackColor = Color.Transparent,
                strokeCap = StrokeCap.Round,
            )
        }
    }
}
