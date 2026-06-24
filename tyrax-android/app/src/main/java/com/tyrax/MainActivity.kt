package com.tyrax

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.SystemBarStyle
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.toArgb
import androidx.navigation.compose.rememberNavController
import com.tyrax.presentation.navigation.TyraxNavGraph
import com.tyrax.presentation.theme.TyraxColors
import com.tyrax.presentation.theme.TyraxTheme
import dagger.hilt.android.AndroidEntryPoint
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box

@AndroidEntryPoint
class MainActivity : ComponentActivity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        // Transparent status bar, white icons (light-on-dark appearance).
        enableEdgeToEdge(
            statusBarStyle = SystemBarStyle.dark(
                scrim = Color.Transparent.toArgb(),
            ),
            navigationBarStyle = SystemBarStyle.dark(
                scrim = Color.Transparent.toArgb(),
            ),
        )

        setContent {
            TyraxTheme {
                val navController = rememberNavController()
                Box(
                    modifier = Modifier
                        .fillMaxSize()
                        .background(TyraxColors.Black),
                ) {
                    TyraxNavGraph(navController = navController)
                }
            }
        }
    }
}
