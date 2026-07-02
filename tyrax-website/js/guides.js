(function () {
  'use strict';

  const tabs = document.querySelectorAll('.device-tab');
  const panels = document.querySelectorAll('.device-panel');
  if (!tabs.length || !panels.length) return;

  const validTargets = ['android', 'windows', 'ios', 'mac'];

  function activate(target) {
    if (!validTargets.includes(target)) {
      target = 'android';
    }

    tabs.forEach((tab) => {
      const isActive = tab.dataset.target === target;
      tab.classList.toggle('active', isActive);
      tab.setAttribute('aria-selected', isActive ? 'true' : 'false');
    });

    panels.forEach((panel) => {
      const isActive = panel.id === 'panel-' + target;
      panel.classList.toggle('active', isActive);
      panel.hidden = !isActive;
    });

    if (history.replaceState) {
      history.replaceState(null, '', '#' + target);
    }
  }

  tabs.forEach((tab) => {
    tab.addEventListener('click', () => activate(tab.dataset.target));
  });

  const hash = location.hash.replace('#', '');
  activate(validTargets.includes(hash) ? hash : 'android');

  window.addEventListener('hashchange', () => {
    const next = location.hash.replace('#', '');
    if (validTargets.includes(next)) {
      activate(next);
    }
  });
})();
