package com.tyrax.di

import dagger.Module
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent

@Module
@InstallIn(SingletonComponent::class)
object AppModule
// DataStore is provided via @Inject constructor in TokenDataStore.
// Network and repository bindings are added in future steps.
