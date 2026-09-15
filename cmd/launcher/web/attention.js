(() => {
  "use strict";
  const K = window.KLU;

  const parseDeviceCode = (input) => input?.match(/code:\s*([A-Z0-9-]+)/i)?.[1]?.toUpperCase()
    || input?.match(/\b[A-Z0-9]{4,}(?:-[A-Z0-9]{3,})+\b/i)?.[0]?.toUpperCase() || "";

  K.signInKilo = async () => {
    if (K.state.connectedProviders.has("kilo")) return K.showError("Kilo is already connected on this computer.");
    K.showError("");
    K.state.authController?.abort();
    K.state.authController = new AbortController();
    K.state.authURL = "";
    K.els.authOpen.disabled = true;
    K.els.authInstructions.textContent = "Starting secure device authorization…";
    K.els.authCode.textContent = "";
    K.els.authCodeWrap.classList.add("hidden");
    K.els.authDialog.showModal();

    try {
      const info = await K.api.oauth.authorizeKilo() || {};
      K.state.authURL = info.url || "";
      K.els.authInstructions.textContent = info.instructions || "Complete the Kilo sign-in in your browser.";
      const code = parseDeviceCode(info.instructions);
      if (code) {
        K.els.authCode.textContent = code;
        K.els.authCodeWrap.classList.remove("hidden");
      }
      K.els.authOpen.disabled = !K.state.authURL;
      if (K.state.authURL) window.open(K.state.authURL, "_blank", "noopener,noreferrer");

      await K.api.oauth.callbackKilo(K.state.authController.signal);
      K.state.authController = null;
      K.els.authInstructions.textContent = "Signed in successfully.";
      await K.loadCatalog();
      window.setTimeout(() => { if (K.els.authDialog.open) K.els.authDialog.close(); }, 650);
    } catch (err) {
      if (err?.name !== "AbortError") K.els.authInstructions.textContent = `Sign-in failed: ${err.message || String(err)}`;
    }
  };

  K.cancelAuth = () => {
    K.state.authController?.abort();
    K.state.authController = null;
    if (K.els.authDialog.open) K.els.authDialog.close();
  };

  K.copyAuthCode = async () => {
    try { if (K.els.authCode.textContent) await navigator.clipboard.writeText(K.els.authCode.textContent); }
    catch {}
  };

  const actionButton = (label, className, fn) => {
    const button = document.createElement("button");
    button.type = "button";
    button.className = className;
    button.textContent = label;
    button.addEventListener("click", fn);
    return button;
  };

  const reset = (title) => {
    K.els.attentionTitle.textContent = title;
    K.els.attentionBody.textContent = "";
    K.els.attentionActions.textContent = "";
  };

  K.loadAttention = async () => {
    if (!K.state.session) return;
    const [p, q] = await Promise.allSettled([
      K.api.permissions.list(K.state.session.id),
      K.api.questions.list(K.state.session.id),
    ]);
    const permissions = p.status === "fulfilled" ? p.value : [];
    const questions = q.status === "fulfilled" ? q.value : [];
    if (permissions[0]) return showPermission(permissions[0]);
    if (questions[0]) return showQuestion(questions[0]);
    if (K.els.attentionDialog.open) {
      K.els.attentionDialog.close();
      K.state.attentionKey = "";
    }
  };

  const showPermission = (item) => {
    const key = `permission:${item.id}`;
    if (K.state.attentionKey === key && K.els.attentionDialog.open) return;
    K.state.attentionKey = key;
    reset("Permission request");

    const text = document.createElement("div");
    const meta = document.createElement("div");
    text.textContent = `Kilo wants permission to ${item.permission || item.action || "perform an action"}.`;
    meta.className = "attention-meta";
    const patterns = Array.isArray(item.patterns) ? item.patterns : Array.isArray(item.resources) ? item.resources : [];
    const always = Array.isArray(item.always) && item.always.length ? `Can save as always allowed:\n${item.always.join("\n")}` : "";
    meta.textContent = [
      patterns.length ? patterns.join("\n") : "",
      item.metadata && Object.keys(item.metadata).length ? JSON.stringify(item.metadata, null, 2) : "",
      always,
    ].filter(Boolean).join("\n\n") || "No additional details.";
    K.els.attentionBody.append(text, meta);
    K.els.attentionActions.append(
      actionButton("Reject", "ghost", () => replyPermission(item, "reject")),
      actionButton("Allow once", "ghost", () => replyPermission(item, "once")),
      actionButton("Always allow", "primary", () => replyPermission(item, "always")),
    );
    if (!K.els.attentionDialog.open) K.els.attentionDialog.showModal();
  };

  const replyPermission = async (item, reply) => {
    try {
      await K.api.permissions.reply(K.state.session.id, item.id, reply);
      K.state.attentionKey = "";
      if (K.els.attentionDialog.open) K.els.attentionDialog.close();
      await K.loadAttention();
    } catch (err) { K.showError(err.message || String(err)); }
  };

  const showQuestion = (item) => {
    const key = `question:${item.id}`;
    if (K.state.attentionKey === key && K.els.attentionDialog.open) return;
    K.state.attentionKey = key;
    reset("Kilo has a question");

    const blocks = [];
    for (const [index, question] of (item.questions || []).entries()) {
      const block = document.createElement("div");
      const header = document.createElement("div");
      const title = document.createElement("strong");
      block.className = "question-block";
      header.className = "attention-kicker";
      header.textContent = question.header || `Question ${index + 1}`;
      title.textContent = question.question || `Question ${index + 1}`;
      block.append(header, title);

      const controls = [];
      const name = `q-${item.id}-${index}`;
      for (const option of question.options || []) {
        const label = document.createElement("label");
        const input = document.createElement("input");
        const span = document.createElement("span");
        label.className = "question-option";
        input.type = question.multiple ? "checkbox" : "radio";
        input.name = name;
        input.value = option.label;
        if (!question.multiple && question.default && option.label === question.default) input.checked = true;
        span.textContent = option.description ? `${option.label} — ${option.description}` : option.label;
        label.append(input, span);
        block.appendChild(label);
        controls.push(input);
      }

      let custom = null;
      if (question.custom !== false) {
        custom = document.createElement("input");
        custom.className = "question-custom";
        custom.placeholder = "Or type a custom answer";
        block.appendChild(custom);
      }
      blocks.push({ controls, custom });
      K.els.attentionBody.appendChild(block);
    }

    K.els.attentionActions.append(
      actionButton("Reject", "ghost", () => rejectQuestion(item)),
      actionButton("Answer", "primary", () => answerQuestion(item, blocks)),
    );
    if (!K.els.attentionDialog.open) K.els.attentionDialog.showModal();
  };

  const answerQuestion = async (item, blocks) => {
    const answers = blocks.map(({ controls, custom }) => {
      const selected = controls.filter((input) => input.checked).map((input) => input.value);
      const typed = custom?.value.trim();
      if (typed) selected.push(typed);
      return selected;
    });
    if (answers.some((answer) => !answer.length)) return K.showError("Answer every Kilo question before continuing.");

    try {
      await K.api.questions.reply(K.state.session.id, item.id, answers);
      K.state.attentionKey = "";
      if (K.els.attentionDialog.open) K.els.attentionDialog.close();
      await K.loadAttention();
    } catch (err) { K.showError(err.message || String(err)); }
  };

  const rejectQuestion = async (item) => {
    try {
      await K.api.questions.reject(K.state.session.id, item.id);
      K.state.attentionKey = "";
      if (K.els.attentionDialog.open) K.els.attentionDialog.close();
      await K.loadAttention();
    } catch (err) { K.showError(err.message || String(err)); }
  };
})();
