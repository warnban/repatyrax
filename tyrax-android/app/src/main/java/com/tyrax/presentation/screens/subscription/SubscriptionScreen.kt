package com.tyrax.presentation.screens.subscription

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.tyrax.R
import com.tyrax.presentation.components.TyraxButton
import com.tyrax.presentation.theme.TyraxColors
import com.tyrax.presentation.theme.TyraxTypography

private val TIERS = listOf("CORE", "SHADOW", "DOMINION")
private val BASE_PRICES = mapOf("CORE" to 199, "SHADOW" to 349, "DOMINION" to 649)

@Composable
fun SubscriptionScreen(
    onNavigateBack: () -> Unit,
    onNavigateToPayment: (String) -> Unit,
    viewModel: SubscriptionViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsStateWithLifecycle()

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(TyraxColors.Black)
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 24.dp),
    ) {
        Spacer(modifier = Modifier.height(52.dp))

        Row(
            verticalAlignment = Alignment.CenterVertically,
            modifier = Modifier.fillMaxWidth(),
        ) {
            Text(
                text     = stringResource(R.string.glyph_back),
                style    = TyraxTypography.headline,
                modifier = Modifier.clickable { onNavigateBack() }.padding(end = 16.dp),
            )
            Text(text = stringResource(R.string.nav_control), style = TyraxTypography.headline)
        }

        Spacer(modifier = Modifier.height(32.dp))

        TIERS.forEach { tier ->
            TierCard(
                tier        = tier,
                currentTier = uiState.currentTier ?: "",
                onUnlock    = { onNavigateToPayment(tier) },
            )
            Spacer(modifier = Modifier.height(16.dp))
        }

        Spacer(modifier = Modifier.height(24.dp))
    }
}

@Composable
private fun TierCard(
    tier: String,
    currentTier: String,
    onUnlock: () -> Unit,
) {
    val isCurrent  = tier.equals(currentTier, ignoreCase = true)
    val borderColor = if (isCurrent) TyraxColors.Red else TyraxColors.White
    val nameColor   = if (isCurrent) TyraxColors.Red else TyraxColors.White
    val basePrice   = BASE_PRICES[tier] ?: 0

    val featuresRes = when (tier) {
        "CORE"     -> R.string.sub_features_core
        "SHADOW"   -> R.string.sub_features_shadow
        "DOMINION" -> R.string.sub_features_dominion
        else       -> R.string.sub_features_core
    }

    Box(
        modifier = Modifier
            .fillMaxWidth()
            .border(1.dp, borderColor)
            .padding(16.dp),
    ) {
        Column {
            Text(text = tier, style = TyraxTypography.headline, color = nameColor)
            Spacer(modifier = Modifier.height(8.dp))
            Text(
                text  = stringResource(featuresRes),
                style = TyraxTypography.body,
                color = TyraxColors.SubText,
            )
            Spacer(modifier = Modifier.height(12.dp))
            Text(
                text  = stringResource(R.string.sub_price_from, basePrice),
                style = TyraxTypography.body,
                color = TyraxColors.White,
            )
            Spacer(modifier = Modifier.height(16.dp))

            if (isCurrent) {
                // Current tier — show a badge instead of the unlock button.
                Box(
                    contentAlignment = Alignment.Center,
                    modifier = Modifier
                        .fillMaxWidth()
                        .border(1.dp, TyraxColors.Red)
                        .padding(vertical = 12.dp),
                ) {
                    Text(
                        text  = stringResource(R.string.sub_badge_current),
                        style = TyraxTypography.label,
                        color = TyraxColors.Red,
                    )
                }
            } else {
                TyraxButton(
                    label    = stringResource(R.string.sub_btn_unlock),
                    onClick  = onUnlock,
                    modifier = Modifier.fillMaxWidth(),
                )
            }
        }
    }
}
