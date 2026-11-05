package com.ousiassllc.swarmviewer.ui.theme

import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** 入力欄の高さ（ガイド O8・§4 フォーム: 高さ 40。tokens-preview の `.fld{height:40px}`・Web の `min-h-10`）。M3 の既定は 56dp。 */
private val FieldMinHeight = 40.dp

/** 入力欄の内側の余白（左右 12dp・上下 8dp。文字の行高 [FieldLineHeight] と合わせて 40dp になる）。 */
private val FieldContentPadding = PaddingValues(horizontal = 12.dp, vertical = 8.dp)

/** ラベルと入力欄の間隔（`--gap-label` 6）。 */
private val FieldLabelGap = 6.dp

/** 入力欄の文字の行高。`bodyLarge` の行高（16sp × 1.8 = 28.8sp）だと 8 + 28.8 + 8 = 44.8dp になるので、24sp に抑えて 40dp にする。 */
private val FieldLineHeight = 24.sp

/**
 * 入力欄（ガイド O8・§4 フォーム）: ラベルは入力の上（`.lbl`。M3 の枠内に浮くラベルは使わない）、入力は高さ 40dp・内側 12dp・角丸 8dp（r-md）・1dp の border・surface の地
 * （Web の `bg-surface` と同じ）。フォーカス中の枠と caret はフォーカスリング色（[ExtraColors.focus]）。
 * [isError] のときは枠・caret・ラベルを `--danger`（[ExtraColors.danger]。Web の `FieldInvalidClass`。ガイド O7 の `.fld.bad`）にする。文言は呼び出し側が入力欄の下に出す。
 * ラベルは入力欄の `contentDescription` にも渡し、読み上げでは入力欄と結び付ける（見た目のラベルは読み上げから外して二重に読ませない）。
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun AppTextField(
    value: String,
    onValueChange: (String) -> Unit,
    label: String,
    modifier: Modifier = Modifier,
    visualTransformation: VisualTransformation = VisualTransformation.None,
    keyboardOptions: KeyboardOptions = KeyboardOptions.Default,
    singleLine: Boolean = false,
    isError: Boolean = false,
) {
    val extra = MaterialTheme.extraColors
    val surface = MaterialTheme.colorScheme.surface
    val accent = if (isError) extra.danger else extra.focus
    val interactionSource = remember { MutableInteractionSource() }
    val colors =
        OutlinedTextFieldDefaults.colors(
            focusedContainerColor = surface,
            unfocusedContainerColor = surface,
            focusedBorderColor = extra.focus,
            errorBorderColor = extra.danger,
        )
    Column(modifier = modifier, verticalArrangement = Arrangement.spacedBy(FieldLabelGap)) {
        Text(
            label,
            style = MaterialTheme.typography.bodyMedium,
            color = if (isError) extra.danger else MaterialTheme.colorScheme.onSurface,
            modifier = Modifier.clearAndSetSemantics {},
        )
        BasicTextField(
            value = value,
            onValueChange = onValueChange,
            modifier = Modifier.fillMaxWidth().heightIn(min = FieldMinHeight).semantics { contentDescription = label },
            textStyle = MaterialTheme.typography.bodyLarge.copy(color = MaterialTheme.colorScheme.onSurface, lineHeight = FieldLineHeight),
            keyboardOptions = keyboardOptions,
            singleLine = singleLine,
            visualTransformation = visualTransformation,
            interactionSource = interactionSource,
            cursorBrush = SolidColor(accent),
            decorationBox = { innerTextField ->
                OutlinedTextFieldDefaults.DecorationBox(
                    value = value,
                    innerTextField = innerTextField,
                    enabled = true,
                    singleLine = singleLine,
                    visualTransformation = visualTransformation,
                    interactionSource = interactionSource,
                    isError = isError,
                    colors = colors,
                    contentPadding = FieldContentPadding,
                    container = {
                        OutlinedTextFieldDefaults.Container(
                            enabled = true,
                            isError = isError,
                            interactionSource = interactionSource,
                            colors = colors,
                            shape = MaterialTheme.shapes.medium,
                        )
                    },
                )
            },
        )
    }
}
