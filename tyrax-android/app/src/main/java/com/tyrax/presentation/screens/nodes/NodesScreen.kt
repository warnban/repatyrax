package com.tyrax.presentation.screens.nodes

import androidx.compose.foundation.background
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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.HorizontalDivider
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
import com.tyrax.domain.model.Node
import com.tyrax.domain.model.NodeStatus
import com.tyrax.presentation.components.StatusBadge
import com.tyrax.presentation.theme.TyraxColors
import com.tyrax.presentation.theme.TyraxTypography

@Composable
fun NodesScreen(
    onNavigateBack: () -> Unit,
    viewModel: NodesViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsStateWithLifecycle()

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(TyraxColors.Black)
            .padding(horizontal = 24.dp),
    ) {
        // ── Header ─────────────────────────────────────────────────────────────
        Spacer(modifier = Modifier.height(52.dp))

        Row(
            verticalAlignment = Alignment.CenterVertically,
            modifier = Modifier.fillMaxWidth(),
        ) {
            Text(
                text     = stringResource(R.string.glyph_back),
                style    = TyraxTypography.headline,
                color    = TyraxColors.White,
                modifier = Modifier
                    .clickable { onNavigateBack() }
                    .padding(end = 16.dp),
            )
            Text(
                text  = stringResource(R.string.nav_nodes),
                style = TyraxTypography.headline,
            )
        }

        Spacer(modifier = Modifier.height(32.dp))

        // ── Content ────────────────────────────────────────────────────────────
        when (val state = uiState) {
            is NodesUiState.Loading -> {
                Box(
                    contentAlignment = Alignment.Center,
                    modifier = Modifier.fillMaxSize(),
                ) {
                    Text(
                        text  = stringResource(R.string.status_scanning_nodes),
                        style = TyraxTypography.label,
                    )
                }
            }

            is NodesUiState.Success -> {
                if (state.nodes.isEmpty()) {
                    Box(
                        contentAlignment = Alignment.Center,
                        modifier = Modifier.fillMaxSize(),
                    ) {
                        Text(
                            text  = stringResource(R.string.status_no_nodes),
                            style = TyraxTypography.body,
                            color = TyraxColors.SubText,
                        )
                    }
                } else {
                    LazyColumn {
                        items(state.nodes, key = { it.id }) { node ->
                            NodeCard(node = node)
                            HorizontalDivider(
                                thickness = 0.5.dp,
                                color     = TyraxColors.MidGray,
                            )
                        }
                    }
                }
            }

            is NodesUiState.Error -> {
                Box(
                    contentAlignment = Alignment.Center,
                    modifier = Modifier.fillMaxSize(),
                ) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Text(
                            text  = state.message,
                            style = TyraxTypography.label,
                            color = TyraxColors.Red,
                        )
                        Spacer(modifier = Modifier.height(16.dp))
                        Text(
                            text     = stringResource(R.string.btn_retry),
                            style    = TyraxTypography.accent,
                            modifier = Modifier.clickable { viewModel.loadNodes() },
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun NodeCard(node: Node) {
    Row(
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment     = Alignment.CenterVertically,
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 16.dp),
    ) {
        // Left: codename + country
        Column {
            Text(
                text  = node.codename,
                style = TyraxTypography.headline,
                color = TyraxColors.White,
            )
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text  = node.country,
                style = TyraxTypography.label,
            )
        }

        // Right: status badge + ping
        Column(horizontalAlignment = Alignment.End) {
            StatusBadge(status = node.status.name)
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text  = stringResource(R.string.label_ping_ms, node.pingMs),
                style = TyraxTypography.accent,
            )
        }
    }
}
