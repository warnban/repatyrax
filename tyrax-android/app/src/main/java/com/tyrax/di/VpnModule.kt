package com.tyrax.di

import com.tyrax.data.repository.VpnRepositoryImpl
import com.tyrax.domain.repository.VpnRepository
import dagger.Binds
import dagger.Module
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton

@Module
@InstallIn(SingletonComponent::class)
abstract class VpnModule {

    @Binds
    @Singleton
    abstract fun bindVpnRepository(impl: VpnRepositoryImpl): VpnRepository
}
