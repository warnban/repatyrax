package com.tyrax.presentation.screens.onboarding

import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import android.content.Intent
import android.net.Uri
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringArrayResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import com.tyrax.R
import com.tyrax.presentation.components.TyraxButton
import com.tyrax.presentation.theme.TyraxColors
import com.tyrax.presentation.theme.TyraxTypography
import kotlinx.coroutines.delay

@Composable
fun OnboardingScreen(
    onNavigateToLogin: () -> Unit,
) {
    val pagerState = rememberPagerState(pageCount = { 3 })

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(TyraxColors.Black),
    ) {
        HorizontalPager(
            state    = pagerState,
            modifier = Modifier.fillMaxSize(),
        ) { page ->
            when (page) {
                0 -> SlideOne()
                1 -> SlideTwo()
                2 -> SlideThree(onNavigateToLogin = onNavigateToLogin)
            }
        }

        // Three-line page indicators, pinned to bottom
        Row(
            horizontalArrangement = Arrangement.spacedBy(6.dp),
            modifier = Modifier
                .align(Alignment.BottomCenter)
                .padding(bottom = 40.dp),
        ) {
            repeat(3) { index ->
                val isActive = pagerState.currentPage == index
                Box(
                    modifier = Modifier
                        .width(if (isActive) 24.dp else 12.dp)
                        .height(2.dp)
                        .background(if (isActive) TyraxColors.Red else TyraxColors.SubText),
                )
            }
        }
    }
}

@Composable
private fun SlideOne() {
    // Staggered word fade-in: each line appears 200ms after the previous.
    val words = stringArrayResource(R.array.onboarding_slide1_words)
    val alphas = words.indices.map { remember { mutableFloatStateOf(0f) } }

    val animatedAlphas = alphas.map { state ->
        animateFloatAsState(
            targetValue   = state.floatValue,
            animationSpec = tween(durationMillis = 400),
            label         = "word_alpha",
        ).value
    }

    LaunchedEffect(Unit) {
        alphas.forEachIndexed { i, state ->
            delay(200L * i)
            state.floatValue = 1f
        }
    }

    Column(
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center,
        modifier = Modifier
            .fillMaxSize()
            .padding(horizontal = 32.dp),
    ) {
        words.forEachIndexed { i, word ->
            Text(
                text      = word,
                style     = TyraxTypography.display,
                textAlign = TextAlign.Center,
                modifier  = Modifier.alpha(animatedAlphas[i]),
            )
        }
    }
}

@Composable
private fun SlideTwo() {
    Column(
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center,
        modifier = Modifier
            .fillMaxSize()
            .padding(horizontal = 32.dp),
    ) {
        Text(
            text      = stringResource(R.string.onboarding_slide2_title),
            style     = TyraxTypography.display,
            textAlign = TextAlign.Center,
        )
    }
}

@Composable
private fun SlideThree(onNavigateToLogin: () -> Unit) {
    val context = LocalContext.current

    val botHint = buildAnnotatedString {
        append(stringResource(R.string.onboarding_bot_step1_prefix))
        append(" ")
        withStyle(SpanStyle(color = TyraxColors.Red)) {
            append(stringResource(R.string.bot_handle))
        }
        append(" ")
        append(stringResource(R.string.onboarding_bot_step1_suffix))
        append("\n")
        append(stringResource(R.string.onboarding_bot_step2))
        append("\n")
        append(stringResource(R.string.onboarding_bot_step3))
    }

    Column(
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center,
        modifier = Modifier
            .fillMaxSize()
            .padding(horizontal = 32.dp),
    ) {
        Text(
            text      = stringResource(R.string.onboarding_slide3_title),
            style     = TyraxTypography.display,
            textAlign = TextAlign.Center,
        )

        Spacer(modifier = Modifier.height(48.dp))

        TyraxButton(
            label    = stringResource(R.string.onboarding_get_access),
            onClick  = onNavigateToLogin,
            modifier = Modifier.fillMaxWidth(),
        )

        Spacer(modifier = Modifier.height(20.dp))

        // Bot onboarding steps. Tapping anywhere opens the bot deep link.
        Text(
            text      = botHint,
            style     = TyraxTypography.label,
            color     = TyraxColors.SubText,
            textAlign = TextAlign.Start,
            modifier  = Modifier
                .fillMaxWidth()
                .clickable { openTelegramBot(context) }
                .padding(vertical = 4.dp),
        )
    }
}

private fun openTelegramBot(context: android.content.Context) {
    context.startActivity(
        Intent(Intent.ACTION_VIEW, Uri.parse("https://t.me/tyraxvpnbot")).apply {
            flags = Intent.FLAG_ACTIVITY_NEW_TASK
        }
    )
}
