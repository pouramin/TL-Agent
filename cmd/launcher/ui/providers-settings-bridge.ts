import { K } from "./kernel";

(() => {
  "use strict";

  
  if (!K || K.__providersSettingsBridgeInstalled) return;
  K.__providersSettingsBridgeInstalled = true;

  const dialog = document.getElementById("settingsDialog");
  const panel = dialog?.querySelector('[data-settings-panel="providers"]');
  const settingsWindow = dialog?.querySelector<HTMLElement>(".settings-window");
  if (!dialog || !panel || !settingsWindow) return;

  const syncProviderWindowMode = () => {
    settingsWindow.classList.toggle("settings-window-providers", !panel.classList.contains("hidden"));
  };
  const panelObserver = new MutationObserver(syncProviderWindowMode);
  panelObserver.observe(panel, { attributes: true, attributeFilter: ["class"] });
  syncProviderWindowMode();

  // product-ui.js captures the original General/About panel list before the
  // Providers extension is injected. Keep the dynamically added panel in sync
  // when the user navigates back to an original settings section.
  for (const button of dialog.querySelectorAll<HTMLElement>("[data-settings-section]")) {
    if (button.dataset.settingsSection === "providers") continue;
    button.addEventListener("click", () => panel.classList.add("hidden"));
  }

  document.getElementById("settingsButton")?.addEventListener("click", () => {
    panel.classList.add("hidden");
  });

  const style = document.createElement("style");
  style.id = "tl-providers-settings-bridge-style";
  style.textContent = `
    .settings-window.settings-window-providers {
      width: min(980px, calc(100vw - 36px));
    }
    .providers-settings-panel {
      max-height: min(72vh, 690px);
      overflow-y: auto;
      padding-right: 7px;
      scrollbar-gutter: stable;
    }
    @media(max-width:920px){
      .settings-window.settings-window-providers {
        width: min(820px, calc(100vw - 24px));
      }
    }
  `;
  document.head.appendChild(style);
})();
