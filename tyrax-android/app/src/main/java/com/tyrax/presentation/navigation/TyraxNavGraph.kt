package com.tyrax.presentation.navigation

import androidx.compose.runtime.Composable
import androidx.navigation.NavHostController
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.navArgument
import com.tyrax.presentation.screens.auth.LoginScreen
import com.tyrax.presentation.screens.auth.RegisterScreen
import com.tyrax.presentation.screens.devices.DevicesScreen
import com.tyrax.presentation.screens.main.MainScreen
import com.tyrax.presentation.screens.nodes.NodesScreen
import com.tyrax.presentation.screens.onboarding.OnboardingScreen
import com.tyrax.presentation.screens.payment.PaymentScreen
import com.tyrax.presentation.screens.settings.SettingsScreen
import com.tyrax.presentation.screens.splash.SplashScreen
import com.tyrax.presentation.screens.subscription.SubscriptionScreen

sealed class Screen(val route: String) {
    object Splash       : Screen("splash")
    object Onboarding   : Screen("onboarding")
    object Login        : Screen("login")
    object Register     : Screen("register")
    object Main         : Screen("main")
    object Nodes        : Screen("nodes")
    object Subscription : Screen("subscription")
    object Devices      : Screen("devices")
    object Settings     : Screen("settings")
    object Payment      : Screen("payment/{tier}") {
        fun create(tier: String): String = "payment/$tier"
    }
}

@Composable
fun TyraxNavGraph(navController: NavHostController) {
    NavHost(
        navController    = navController,
        startDestination = Screen.Splash.route,
    ) {
        composable(Screen.Splash.route) {
            SplashScreen(
                onNavigateToMain       = {
                    navController.navigate(Screen.Main.route) {
                        popUpTo(Screen.Splash.route) { inclusive = true }
                    }
                },
                onNavigateToOnboarding = {
                    navController.navigate(Screen.Onboarding.route) {
                        popUpTo(Screen.Splash.route) { inclusive = true }
                    }
                },
            )
        }

        composable(Screen.Onboarding.route) {
            OnboardingScreen(
                onNavigateToLogin = {
                    navController.navigate(Screen.Login.route) {
                        popUpTo(Screen.Onboarding.route) { inclusive = true }
                    }
                },
            )
        }

        composable(Screen.Login.route) {
            LoginScreen(
                onNavigateToMain = {
                    navController.navigate(Screen.Main.route) {
                        popUpTo(Screen.Login.route) { inclusive = true }
                    }
                },
                onNavigateToRegister = {
                    navController.navigate(Screen.Register.route)
                },
            )
        }

        composable(Screen.Register.route) {
            RegisterScreen(
                onNavigateToMain = {
                    navController.navigate(Screen.Main.route) {
                        popUpTo(Screen.Register.route) { inclusive = true }
                    }
                },
                onNavigateToLogin = {
                    navController.popBackStack()
                },
            )
        }

        composable(Screen.Main.route) {
            MainScreen(
                onNavigateToNodes        = { navController.navigate(Screen.Nodes.route) },
                onNavigateToSubscription = { navController.navigate(Screen.Subscription.route) },
                onNavigateToSettings     = { navController.navigate(Screen.Settings.route) },
            )
        }

        composable(Screen.Nodes.route) {
            NodesScreen(onNavigateBack = { navController.popBackStack() })
        }

        composable(Screen.Subscription.route) {
            SubscriptionScreen(
                onNavigateBack      = { navController.popBackStack() },
                onNavigateToPayment = { tier -> navController.navigate(Screen.Payment.create(tier)) },
            )
        }

        composable(
            route     = Screen.Payment.route,
            arguments = listOf(navArgument("tier") { type = NavType.StringType }),
        ) {
            PaymentScreen(
                onNavigateBack    = { navController.popBackStack() },
                onPaymentComplete = {
                    navController.popBackStack(Screen.Main.route, inclusive = false)
                },
            )
        }

        composable(Screen.Devices.route) {
            DevicesScreen(onNavigateBack = { navController.popBackStack() })
        }

        composable(Screen.Settings.route) {
            SettingsScreen(
                onNavigateBack           = { navController.popBackStack() },
                onNavigateToSubscription = { navController.navigate(Screen.Subscription.route) },
                onNavigateToDevices      = { navController.navigate(Screen.Devices.route) },
                onLoggedOut              = {
                    navController.navigate(Screen.Onboarding.route) {
                        popUpTo(0) { inclusive = true }
                    }
                },
            )
        }
    }
}
