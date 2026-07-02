/**
 * TYRAX download URLs — single source for all pages.
 * Deploy the Windows installer to: tyrax-website/download/windows/TYRAX-Setup.exe
 */
(function () {
  'use strict';

  const DOWNLOADS = {
    android: '/download/tyrax.apk',
    androidName: 'tyrax.apk',
    windows: '/download/windows/TYRAX-Setup.exe',
    windowsName: 'TYRAX-Setup.exe',
  };

  function applyDownloadLinks() {
    document.querySelectorAll('[data-dl="android"]').forEach((el) => {
      el.href = DOWNLOADS.android;
      el.setAttribute('download', DOWNLOADS.androidName);
    });

    document.querySelectorAll('[data-dl="windows"]').forEach((el) => {
      el.href = DOWNLOADS.windows;
      el.setAttribute('download', DOWNLOADS.windowsName);
    });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', applyDownloadLinks);
  } else {
    applyDownloadLinks();
  }
})();
