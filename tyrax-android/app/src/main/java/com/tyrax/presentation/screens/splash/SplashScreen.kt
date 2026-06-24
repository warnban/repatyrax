package com.tyrax.presentation.screens.splash

import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.res.stringResource
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.tyrax.R
import com.tyrax.presentation.theme.TyraxColors
import com.tyrax.presentation.theme.TyraxTypography
import kotlinx.coroutines.delay

@Composable
fun SplashScreen(
    onNavigateToMain: () -> Unit,
    onNavigateToOnboarding: () -> Unit,
    viewModel: SplashViewModel = hiltViewModel(),
) {
    val hasToken by viewModel.hasToken.collectAsStateWithLifecycle()

    // Three-frame glitch: 1f → 0f → 1f → 0f → navigate
    var alpha by remember { mutableFloatStateOf(1f) }
    var glitching by remember { mutableStateOf(false) }

    val animatedAlpha by animateFloatAsState(
        targetValue  = alpha,
        animationSpec = tween(durationMillis = 50),
        label        = "glitch_alpha",
    )

    LaunchedEffect(hasToken) {
        if (hasToken == null) return@LaunchedEffect // still loading DataStore

        delay(1_200)

        // Glitch: 3 rapid flickers over ~150ms then navigate
        glitching = true
        repeat(3) {
            alpha = 0f
            delay(50)
            alpha = 1f
            delay(50)
        }

        if (hasToken == true) {
            onNavigateToMain()
        } else {
            onNavigateToOnboarding()
        }
    }

    Box(
        contentAlignment = Alignment.Center,
        modifier = Modifier
            .fillMaxSize()
            .background(TyraxColors.Black),
    ) {
        Text(
            text     = stringResource(R.string.app_name),
            style    = TyraxTypography.display,
            modifier = Modifier.alpha(animatedAlpha),
        )
    }
}
