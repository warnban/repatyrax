/**
 * TYRAX digital rain — ported from MatrixRainBackground.kt
 * Faint red glyphs falling top→bottom. Respects prefers-reduced-motion.
 */
(function () {
  'use strict';

  const GLYPHS = '01<>/\\[]{}#*+=$%&:;ﾊﾐﾋｰｳｼﾅﾉﾎｱｶﾀﾃ';
  const CELL_PX = 18;
  const TEXT_PX = 15;
  const TAIL = 14;
  const RED = '255, 30, 30';

  function initMatrixRain() {
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;

    const canvas = document.getElementById('matrix-rain');
    if (!canvas) return;

    const ctx = canvas.getContext('2d');
    let width = 0;
    let height = 0;
    let cols = 0;
    let rows = 0;
    let cycle = 0;
    let rafId = null;
    let startTime = null;

    function resize() {
      width = window.innerWidth;
      height = window.innerHeight;
      canvas.width = width;
      canvas.height = height;
      cols = Math.max(1, Math.floor(width / CELL_PX));
      rows = Math.max(1, Math.floor(height / CELL_PX));
      cycle = rows + TAIL;
    }

    function draw(time) {
      if (!startTime) startTime = time;
      const elapsed = (time - startTime) / 1000;

      ctx.clearRect(0, 0, width, height);
      ctx.font = `${TEXT_PX}px "JetBrains Mono", monospace`;
      ctx.textBaseline = 'top';

      for (let c = 0; c < cols; c++) {
        const speed = 5 + ((c * 37) % 11);
        const phase = (c * 53) % cycle;
        const headRaw = Math.floor(elapsed * speed + phase);
        const headRow = ((headRaw % cycle) + cycle) % cycle;

        for (let i = 0; i < TAIL; i++) {
          const row = headRow - i;
          if (row < 0 || row > rows) continue;

          const alpha = i === 0 ? 0.32 : (1 - i / TAIL) * 0.16;
          ctx.fillStyle = `rgba(${RED}, ${alpha})`;

          const glyphIndex =
            (((c * 31 + row) % GLYPHS.length) + GLYPHS.length) % GLYPHS.length;
          ctx.fillText(GLYPHS[glyphIndex], c * CELL_PX, row * CELL_PX);
        }
      }

      rafId = requestAnimationFrame(draw);
    }

    resize();
    rafId = requestAnimationFrame(draw);

    window.addEventListener('resize', resize);

    document.addEventListener('visibilitychange', () => {
      if (document.hidden) {
        if (rafId) cancelAnimationFrame(rafId);
        rafId = null;
      } else if (!rafId) {
        startTime = null;
        rafId = requestAnimationFrame(draw);
      }
    });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initMatrixRain);
  } else {
    initMatrixRain();
  }
})();
