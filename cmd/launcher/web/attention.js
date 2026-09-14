(() => {
  "use strict";
  const K = window.KLU;

  const parseDeviceCode = (input) => input?.match(/code:\s*([A-Z0-9-]+)/i)?.[1]?.toUpperCase()
    || input?.match(/\b[A-Z0-9]{4,}(?:-[A-Z0-9]{3,})+\b/i)?.[0]?.toUpperCase() || "";

  K.signInKilo = async () => {
    if (K.state.connectedProviders.has("kilo")) return K.showError("Kilo is already connected on this computer.");
    K.showError(""); K.state.authController?.abort(); K.state.authController = new AbortController(); K.state.authURL = "";
    Object.assign(K.els.authOpen, { disabled: true });
    K.els.authInstructions.textContent = "Starting secure device authorization…"; K.els.authCode.textContent = ""; K.els.authCodeWrap.classList.add("hidden"); K.els.authDialog.showModal();
    try {
      const info = K.unwrap(await K.kilo("/provider/kilo/oauth/authorize", { method: "POST", body: JSON.stringify({ method: 0 }) })) || {};
      K.state.authURL = info.url || ""; K.els.authInstructions.textContent = info.instructions || "Complete the Kilo sign-in in your browser.";
      const code = parseDeviceCode(info.instructions);
      if (code) { K.els.authCode.textContent = code; K.els.authCodeWrap.classList.remove("hidden"); }
      K.els.authOpen.disabled = !K.state.authURL;
      if (K.state.authURL) window.open(K.state.authURL, "_blank", "noopener,noreferrer");
      await K.kilo("/provider/kilo/oauth/callback", { method: "POST", body: JSON.stringify({ method: 0 }), signal: K.state.authController.signal });
      K.state.authController = null; K.els.authInstructions.textContent = "Signed in successfully."; await K.loadAgentsAndModels();
      window.setTimeout(() => { if (K.els.authDialog.open) K.els.authDialog.close(); }, 650);
    } catch (err) { if (err?.name !== "AbortError") K.els.authInstructions.textContent = `Sign-in failed: ${err.message || String(err)}`; }
  };

  K.cancelAuth = () => { K.state.authController?.abort(); K.state.authController = null; if (K.els.authDialog.open) K.els.authDialog.close(); };
  K.copyAuthCode = async () => { try { if (K.els.authCode.textContent) await navigator.clipboard.writeText(K.els.authCode.textContent); } catch {} };

  const actionButton = (label, className, fn) => {
    const button = document.createElement("button"); button.type = "button"; button.className = className; button.textContent = label; button.addEventListener("click", fn); return button;
  };
  const reset = (title) => { K.els.attentionTitle.textContent = title; K.els.attentionBody.textContent = ""; K.els.attentionActions.textContent = ""; };

  K.loadAttention = async () => {
    if (!K.state.session) return;
    const id = encodeURIComponent(K.state.session.id);
    const [p, q] = await Promise.allSettled([K.kilo(`/api/session/${id}/permission`), K.kilo(`/api/session/${id}/question`)]);
    const permissions = p.status === "fulfilled" && Array.isArray(p.value?.data) ? p.value.data : [];
    const questions = q.status === "fulfilled" && Array.isArray(q.value?.data) ? q.value.data : [];
    if (permissions[0]) return showPermission(permissions[0]);
    if (questions[0]) return showQuestion(questions[0]);
    if (K.els.attentionDialog.open) { K.els.attentionDialog.close(); K.state.attentionKey = ""; }
  };

  const showPermission = (item) => {
    const key = `permission:${item.id}`; if (K.state.attentionKey === key && K.els.attentionDialog.open) return; K.state.attentionKey = key; reset("Permission request");
    const text = document.createElement("div"), meta = document.createElement("div");
    text.textContent = `Kilo wants permission to ${item.action || "perform an action"}.`; meta.className = "attention-meta";
    meta.textContent = [Array.isArray(item.resources) ? item.resources.join("\n") : "", item.metadata ? JSON.stringify(item.metadata, null, 2) : ""].filter(Boolean).join("\n\n") || "No additional details.";
    K.els.attentionBody.append(text, meta);
    K.els.attentionActions.append(actionButton("Reject", "ghost", () => replyPermission(item, "reject")), actionButton("Allow once", "ghost", () => replyPermission(item, "once")), actionButton("Always allow", "primary", () => replyPermission(item, "always")));
    if (!K.els.attentionDialog.open) K.els.attentionDialog.showModal();
  };

  const replyPermission = async (item, reply) => {
    try {
      await K.kilo(`/api/session/${encodeURIComponent(K.state.session.id)}/permission/${encodeURIComponent(item.id)}/reply`, { method: "POST", body: JSON.stringify({ reply }) });
      K.state.attentionKey = ""; if (K.els.attentionDialog.open) K.els.attentionDialog.close(); await K.loadAttention(); K.startPolling();
    } catch (err) { K.showError(err.message || String(err)); }
  };

  const showQuestion = (item) => {
    const key = `question:${item.id}`; if (K.state.attentionKey === key && K.els.attentionDialog.open) return; K.state.attentionKey = key; reset("Kilo has a question");
    const blocks = [];
    for (const [index, question] of (item.questions || []).entries()) {
      const block = document.createElement("div"), title = document.createElement("strong"); block.className = "question-block"; title.textContent = question.question || question.header || `Question ${index + 1}`; block.appendChild(title);
      const controls = [], name = `q-${item.id}-${index}`;
      for (const option of question.options || []) {
        const label = document.createElement("label"), input = document.createElement("input"), span = document.createElement("span");
        label.className = "question-option"; input.type = question.multiple ? "checkbox" : "radio"; input.name = name; input.value = option.label;
        span.textContent = option.description ? `${option.label} — ${option.description}` : option.label; label.append(input, span); block.appendChild(label); controls.push(input);
      }
      let custom = null;
      if (question.custom !== false) { custom = document.createElement("input"); custom.className = "question-custom"; custom.placeholder = "Or type a custom answer"; block.appendChild(custom); }
      blocks.push({ controls, custom }); K.els.attentionBody.appendChild(block);
    }
    K.els.attentionActions.append(actionButton("Reject", "ghost", () => rejectQuestion(item)), actionButton("Answer", "primary", () => answerQuestion(item, blocks)));
    if (!K.els.attentionDialog.open) K.els.attentionDialog.showModal();
  };

  const answerQuestion = async (item, blocks) => {
    const answers = blocks.map(({ controls, custom }) => {
      const selected = controls.filter((i) => i.checked).map((i) => i.value), typed = custom?.value.trim(); if (typed) selected.push(typed); return selected;
    });
    if (answers.some((a) => !a.length)) return K.showError("Answer every Kilo question before continuing.");
    try {
      await K.kilo(`/api/session/${encodeURIComponent(K.state.session.id)}/question/${encodeURIComponent(item.id)}/reply`, { method: "POST", body: JSON.stringify({ answers }) });
      K.state.attentionKey = ""; if (K.els.attentionDialog.open) K.els.attentionDialog.close(); await K.loadAttention(); K.startPolling();
    } catch (err) { K.showError(err.message || String(err)); }
  };

  const rejectQuestion = async (item) => {
    try {
      await K.kilo(`/api/session/${encodeURIComponent(K.state.session.id)}/question/${encodeURIComponent(item.id)}/reject`, { method: "POST" });
      K.state.attentionKey = ""; if (K.els.attentionDialog.open) K.els.attentionDialog.close(); await K.loadAttention(); K.startPolling();
    } catch (err) { K.showError(err.message || String(err)); }
  };
})();
