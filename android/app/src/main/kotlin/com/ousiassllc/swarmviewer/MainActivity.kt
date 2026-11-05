package com.ousiassllc.swarmviewer

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Surface
import androidx.compose.ui.Modifier
import com.ousiassllc.swarmviewer.ui.theme.SwarmViewerTheme

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            SwarmViewerTheme {
                Surface(modifier = Modifier.fillMaxSize()) {
                    // 画面は後続の子 Issue で実装する。土台では空の画面を表示する。
                    Box(modifier = Modifier.fillMaxSize())
                }
            }
        }
    }
}
