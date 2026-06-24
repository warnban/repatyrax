package com.tyrax.presentation.screens.payment

import android.net.Uri
import androidx.browser.customtabs.CustomTabsIntent
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
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.tyrax.R
import com.tyrax.presentation.components.TyraxButton
import com.tyrax.presentation.components.TyraxDialog
import com.tyrax.presentation.theme.TyraxColors
import com.tyrax.presentation.theme.TyraxTypography

private val PERIOD_OPTIONS = listOf(1, 3, 6, 12)

// (label res, glyph res, backend method code)
private data class PayMethod(val labelRes: Int, val glyphRes: Int, val code: String)

private val METHODS = listOf(
    PayMethod(R.string.payment_method_card, R.string.payment_glyph_card, "CARD_RF"),
    PayMethod(R.string.payment_method_sbp, R.string.payment_glyph_sbp, "SBP"),
    PayMethod(R.string.payment_method_crypto, R.string.payment_glyph_crypto, "CRYPTO"),
)

@Composable
fun PaymentScreen(
    onNavigateBack: () -> Unit,
    onPaymentComplete: () -> Unit,
    viewModel: PaymentViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsStateWithLifecycle()
    val context = androidx.compose.ui.platform.LocalContext.current

    LaunchedEffect(Unit) {
        viewModel.events.collect { event ->
            when (event) {
                is PaymentEvent.OpenUrl -> {
                    // CustomTabs handles all providers; the URL opens the chosen flow.
                    CustomTabsIntent.Builder().build().launchUrl(context, Uri.parse(event.url))
                }
            }
        }
    }

    if (uiState.paymentSuccess) {
        TyraxDialog(
            title       = stringResource(R.string.payment_success_title),
            body        = stringResource(R.string.payment_success_body, uiState.tier),
            confirmText = stringResource(R.string.payment_back_to_main),
            cancelText  = "",
            onConfirm   = onPaymentComplete,
            onDismiss   = onPaymentComplete,
        )
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(TyraxColors.Black)
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
            Text(text = stringResource(R.string.payment_title), style = TyraxTypography.headline)
        }

        Spacer(modifier = Modifier.height(32.dp))

        // ── Section 1 — period selector ─────────────────────────────────────────
        Row(
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            modifier              = Modifier.fillMaxWidth(),
        ) {
            PERIOD_OPTIONS.forEach { months ->
                val isActive = uiState.selectedMonths == months
                val discountText = when (months) {
                    3  -> stringResource(R.string.discount_3mo)
                    6  -> stringResource(R.string.discount_6mo)
                    12 -> stringResource(R.string.discount_12mo)
                    else -> null
                }
                val chipText = if (discountText != null) {
                    stringResource(R.string.label_months_discount, months, discountText)
                } else {
                    stringResource(R.string.label_months, months)
                }
                Box(
                    contentAlignment = Alignment.Center,
                    modifier = Modifier
                        .weight(1f)
                        .then(
                            if (isActive) Modifier.background(TyraxColors.Red)
                            else Modifier.border(1.dp, TyraxColors.Red)
                        )
                        .clickable { viewModel.selectMonths(months) }
                        .padding(vertical = 8.dp),
                ) {
                    Text(
                        text      = chipText,
                        style     = TyraxTypography.label,
                        color     = TyraxColors.White,
                        textAlign = TextAlign.Center,
                    )
                }
            }
        }

        Spacer(modifier = Modifier.height(40.dp))

        // ── Section 2 — price display ───────────────────────────────────────────
        Column(
            horizontalAlignment = Alignment.CenterHorizontally,
            modifier            = Modifier.fillMaxWidth(),
        ) {
            Text(
                text  = stringResource(R.string.payment_price, uiState.total),
                style = TyraxTypography.display,
                color = TyraxColors.White,
            )
            Spacer(modifier = Modifier.height(8.dp))
            val subLabel = when (uiState.selectedMonths) {
                1    -> stringResource(R.string.payment_for_1)
                3    -> stringResource(R.string.payment_for_3, uiState.saving)
                6    -> stringResource(R.string.payment_for_6, uiState.saving)
                else -> stringResource(R.string.payment_for_12, uiState.saving)
            }
            Text(text = subLabel, style = TyraxTypography.label, color = TyraxColors.SubText)
        }

        Spacer(modifier = Modifier.height(40.dp))

        // ── Section 3 — payment method ──────────────────────────────────────────
        Row(
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            modifier              = Modifier.fillMaxWidth(),
        ) {
            METHODS.forEach { method ->
                val isActive = uiState.selectedMethod == method.code
                Column(
                    horizontalAlignment = Alignment.CenterHorizontally,
                    modifier = Modifier
                        .weight(1f)
                        .border(
                            width = if (isActive) 1.5.dp else 1.dp,
                            color = if (isActive) TyraxColors.Red else TyraxColors.MidGray,
                        )
                        .clickable { viewModel.selectMethod(method.code) }
                        .padding(vertical = 16.dp),
                ) {
                    Text(text = stringResource(method.glyphRes), style = TyraxTypography.headline)
                    Spacer(modifier = Modifier.height(8.dp))
                    Text(
                        text      = stringResource(method.labelRes),
                        style     = TyraxTypography.label,
                        color     = TyraxColors.White,
                        textAlign = TextAlign.Center,
                    )
                }
            }
        }

        Spacer(modifier = Modifier.weight(1f))

        // ── Error ───────────────────────────────────────────────────────────────
        uiState.error?.let { err ->
            Text(
                text      = err,
                style     = TyraxTypography.label,
                color     = TyraxColors.Red,
                textAlign = TextAlign.Center,
                modifier  = Modifier.fillMaxWidth(),
            )
            Spacer(modifier = Modifier.height(12.dp))
        }

        // ── Section 4 — CTA ─────────────────────────────────────────────────────
        TyraxButton(
            label    = stringResource(R.string.payment_cta),
            onClick  = { viewModel.pay() },
            loading  = uiState.isLoading,
            modifier = Modifier.fillMaxWidth(),
        )
        Spacer(modifier = Modifier.height(12.dp))
        Text(
            text      = stringResource(R.string.payment_auto_note),
            style     = TyraxTypography.label,
            color     = TyraxColors.SubText,
            textAlign = TextAlign.Center,
            modifier  = Modifier.fillMaxWidth(),
        )

        Spacer(modifier = Modifier.height(40.dp))
    }
}
