package com.tyrax.domain.usecase

import com.tyrax.data.remote.AndroidUpdateDto
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Test

class CheckUpdateUseCaseTest {

    private fun dto(code: Int, mandatory: Boolean = false) = AndroidUpdateDto(
        version = "9.9.9",
        versionCode = code,
        url = "https://tyrax.tech/download/android/TYRAX.apk",
        mandatory = mandatory,
        notes = "COLD NOTES",
    )

    @Test
    fun `newer version returns update info`() {
        val info = CheckUpdateUseCase.resolveUpdate(dto(code = 10), currentCode = 5, dismissedCode = 0)
        assertNotNull(info)
        assertEquals(10, info!!.versionCode)
        assertEquals("9.9.9", info.version)
    }

    @Test
    fun `same or older version returns null`() {
        assertNull(CheckUpdateUseCase.resolveUpdate(dto(code = 5), currentCode = 5, dismissedCode = 0))
        assertNull(CheckUpdateUseCase.resolveUpdate(dto(code = 4), currentCode = 5, dismissedCode = 0))
    }

    @Test
    fun `newer but dismissed and not mandatory returns null`() {
        assertNull(CheckUpdateUseCase.resolveUpdate(dto(code = 10), currentCode = 5, dismissedCode = 10))
    }

    @Test
    fun `newer dismissed but mandatory returns update info`() {
        val info = CheckUpdateUseCase.resolveUpdate(
            dto(code = 10, mandatory = true), currentCode = 5, dismissedCode = 10,
        )
        assertNotNull(info)
        assertEquals(true, info!!.mandatory)
    }
}
