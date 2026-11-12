(function () {
  'use strict';

  const REPO = 'Tenkayro/Vylos';
  let version = '1.1.1';
  let current = 'parent';

  const editions = {
    parent: {
      file: 'Vylos-Parent.msi',
      hint: 'Install on the Parent PC. Manage screen time, bedtime, app rules, and pair Child PCs over your home Wi-Fi/LAN.',
      url: `https://github.com/${REPO}/releases/latest/download/Vylos-Parent.msi`,
      altText: 'Need the Child PC installer? Download Vylos-Child.msi →',
      altUrl: `https://github.com/${REPO}/releases/latest/download/Vylos-Child.msi`,
    },
    child: {
      file: 'Vylos-Child.msi',
      hint: 'Install on your Child’s PC (set up by a parent). Enforces limits & DNS filtering in the background — even offline.',
      url: `https://github.com/${REPO}/releases/latest/download/Vylos-Child.msi`,
      altText: 'Need the Parent Dashboard? Download Vylos-Parent.msi →',
      altUrl: `https://github.com/${REPO}/releases/latest/download/Vylos-Parent.msi`,
    },
  };

  function updateUI() {
    const ed = editions[current];
    document.getElementById('tab-parent')?.classList.toggle('active', current === 'parent');
    document.getElementById('tab-child')?.classList.toggle('active', current === 'child');

    const hintEl = document.getElementById('ed-hint');
    const btnEl = document.getElementById('dl-btn');
    const btnLabel = document.getElementById('dl-btn-label');
    const altEl = document.getElementById('dl-alt');
    const verEl = document.getElementById('dl-ver');

    if (hintEl) hintEl.textContent = ed.hint;
    if (btnEl) btnEl.setAttribute('href', ed.url);
    if (btnLabel) btnLabel.textContent = `Download ${ed.file}`;
    if (altEl) {
      altEl.textContent = ed.altText;
      altEl.setAttribute('href', ed.altUrl);
    }
    if (verEl) verEl.textContent = `v${version} · Windows 10/11`;
  }

  async function loadLatestVersion() {
    try {
      const res = await fetch(`https://api.github.com/repos/${REPO}/releases/latest`);
      if (!res.ok) return;
      const data = await res.json();
      if (data?.tag_name) {
        version = String(data.tag_name).replace(/^v/i, '');
        updateUI();
      }
    } catch {
      // keep default version
    }
  }

  document.addEventListener('DOMContentLoaded', () => {
    document.getElementById('tab-parent')?.addEventListener('click', () => {
      current = 'parent';
      updateUI();
    });
    document.getElementById('tab-child')?.addEventListener('click', () => {
      current = 'child';
      updateUI();
    });

    updateUI();
    loadLatestVersion();
  });
})();
