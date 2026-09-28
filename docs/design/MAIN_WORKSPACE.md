# TL Studio — Main Workspace

> Durable product-design reference for the real TL Studio application. The repository implementation is the final source of truth. This document separates current implementation from future possibilities.

## Purpose

TL Studio is an AI-native development workspace, not a chat application with an editor attached.

Core principle: **Build software yourself, with AI, or both.**

The Editor remains the visual center of gravity. TL Agent is a first-class participant that can expand for collaboration and collapse when manual coding is primary.

## Current information hierarchy

TL Studio currently composes one persistent workspace with these regions:

- Global Top Bar: product, project, command entry, Local status, Preview, appearance, refresh and hosted account.
- Activity Rail: Explorer, Search, Sessions, Changes and Settings.
- Context Sidebar: the real selected activity.
- Editor Workspace: file tabs, Monaco Editor and the bottom Terminal.
- TL Agent: Collapsed, Compact and Focused presentation states.
- Floating Live Preview.
- compact Status Bar.

The redesign changes product composition and interaction hierarchy. The current product architecture is TL Studio-native across workspace, model, permission, question, session, and Agent execution.

## Global Top Bar

The top bar reuses real product controls and state:

- TL Studio identity.
- the real current-project button and operating-system project picker.
- Command Palette trigger.
- real local TL Studio service status.
- real Live Preview trigger.
- appearance shortcut connected to the existing preference.
- existing refresh action.
- existing hosted-account control.

Low-frequency actions remain contextual or in Settings/Command Palette.

## Activity Rail and Context Sidebar

The Activity Rail selects which real feature is presented in the Context Sidebar without destroying Editor, Terminal, Agent or Preview state.

### Explorer

The Explorer preserves the real filesystem implementation:

- project-root confinement.
- directory navigation.
- create file and folder.
- rename and delete.
- Show in Folder.
- opening files.
- symlink restrictions.
- selected-entry state.

The redesign changes placement and density, not filesystem semantics.

### Search

Project Search preserves:

- project-scoped search.
- include and exclude filters.
- Match Case.
- stale-query cancellation.
- exact result navigation.
- Ctrl/Cmd + Shift + F.

Replace in Files is not implemented.

### Sessions

Sessions reuse TL Studio's semantic session model and current create/select/rename/delete/persisted-history behavior.

### Changes

Changes uses real semantic session change data: file path, additions, deletions and patch data where available. It does not invent source-control or authorship information that TL Studio cannot reliably determine.

## Editor

The real locally bundled Monaco Editor remains the primary editor.

Preserved behavior includes:

- lazy local Monaco loading and textarea fallback.
- language selection by file.
- one model per open file.
- multiple tabs and tab closing.
- Ctrl/Cmd + S.
- direct local save.
- SHA-256 revision protection.
- save-conflict flow.
- dirty state.
- external-change reconciliation.
- supported preview-only media tabs.
- exact Search navigation.
- existing editor font/theme preferences.

The layout protects an approximately 420px minimum useful Editor width.

## File states

### Clean

The buffer matches its last known disk revision.

### Dirty / unsaved

The buffer differs from saved content. Marker/text accompanies color.

### Agent modified

When the current semantic Agent session reports a changed path matching an open tab, that tab receives an explicit A marker. This means changed in the current Agent session; it is not a general Git authorship system.

### Externally modified

Open tabs reconcile against TL Studio workspace file events. A dirty buffer whose disk SHA changes is marked externally changed.

### Deleted externally

Existing reconciliation remains authoritative: clean removed files may leave the tab set; dirty removed files remain protected as an external-change condition.

### Conflict

Saving with a stale expected SHA keeps the existing conflict/overwrite decision path.

## Resizable workspace panels

Three splitters use document-level pointer tracking after pointerdown:

1. Context Sidebar right edge.
2. TL Agent left edge.
3. Terminal top edge.

Current constraints:

- Context Sidebar minimum: 180px.
- expanded TL Agent minimum: 280px.
- Editor useful-width target: about 420px.
- expanded Terminal minimum: 96px.

Maximum sizes adapt to the viewport and the Editor's remaining space.

Panel geometry/state is persisted under the namespaced Browser preference key:

tl-studio.workspace-layout.v1

Stored values include Context Sidebar width, TL Agent width, Terminal height, Context Sidebar collapsed state and TL Agent presentation state.

## Panel collapse behavior

The Context Sidebar can collapse and restore its previous width.

TL Agent has Collapsed, Compact and Focused states.

Terminal keeps its real open/close behavior and reuses the persisted height when reopened.

## TL Agent

The Agent redesign is presentation-only. It keeps the real session/Agent implementation.

### Collapsed

A narrow rail communicates Agent presence/state while maximizing Editor space.

### Compact

Normal collaboration state with real conversation, tool activity, permissions/questions, attachments, composer, model selection, Agent selection, Stop/Abort, errors and status.

### Focused

A wider Agent workspace for planning/review while the Editor remains available.

The workspace uses the single TL Studio Native Agent execution path.

## Terminal

The Terminal is the real project-scoped command runner integrated as a resizable bottom panel.

Preserved behavior:

- project working directory.
- Run.
- Stop.
- Clear.
- command history.
- process output.
- bounded transcript.
- process lifecycle.
- Windows process-tree termination.

It is not a full PTY and does not claim interactive TUI, Debug Console, Ports or debugger features.

## Live Preview

The real capability-driven Preview remains a floating tool window and keeps the repository's existing support for HTML, SVG/raster image, PDF, video, audio, Markdown, text and supported development-server previews.

Existing local security/origin behavior remains authoritative.

The window is draggable and resizable from N, NE, E, SE, S, SW, W and NW. Resize and drag use document-level pointer tracking after pointerdown, with viewport constraints and persisted geometry.

Preview position and size remain stored under:

tl-studio.preview-window

Saved geometry is clamped back into a usable viewport region when necessary.

## Command Palette

Primary shortcut: Ctrl/Cmd + K.

Current commands expose real behavior only:

- Open project.
- Open file / Explorer.
- Search project.
- Switch Session.
- Focus, Compact or Collapse TL Agent.
- Open Live Preview.
- Toggle Terminal.
- Change Model.
- Open Settings.

No future/fake command is exposed.

## Project switching

The top project control reuses the real project picker.

Existing safeguards remain:

- unsaved Editor buffers prompt before switching.
- Terminal follows existing project-switch cleanup.
- Preview follows existing project-switch cleanup.
- Plugin and project lifecycle remains in existing TL Studio hooks.
- workspace file/editor state reloads against the selected project.

## Local status and Status Bar

Local execution remains quietly visible in the top and bottom workspace chrome. It uses real TL Studio local service state.

The Status Bar currently exposes reliable state only:

- local service status.
- change count.
- Agent state.
- selected model.
- editor cursor position.
- Terminal entry.

A Git branch is intentionally not shown until TL Studio has a reliable branch data source.

## Keyboard interactions

Current important workspace shortcuts:

- Ctrl/Cmd + K — Command Palette.
- Ctrl/Cmd + Shift + F — Project Search.
- Ctrl/Cmd + S — save active editor file.
- Escape — close the Command Palette and existing transient surfaces where supported.

Monaco retains normal editor keyboard behavior.

## Responsive desktop behavior

Primary target: 1440 × 900.

The implementation has explicit desktop adaptations around 1350px, 1120px and 880px.

Collapse priority:

1. reduce secondary presentation density.
2. remove Context Sidebar from constrained layouts.
3. collapse TL Agent to its rail.
4. preserve Editor space.

TL Studio remains desktop-first.

## Runtime and security boundaries

The redesign does not change TL Studio's runtime architecture.

The Browser continues to use existing TL Studio-owned semantic /local/* and /runtime/* boundaries. It does not introduce direct Browser coupling to bundled-engine private APIs.

Filesystem, permission, credential, process and Preview security boundaries remain owned by existing launcher/runtime components.

## Current limitations

The current implementation intentionally does not expose:

- full PTY or interactive TUI terminal.
- Replace in Files.
- full Git client or unsupported Git UI.
- Git branch status without a reliable data source.
- Problems panel.
- Debug Console.
- Ports panel.
- debugger integration.
- cloud collaboration or hosted workspace.
- deployment platform.
- remote agents.
- Extensions marketplace.

The Changes view is session-change oriented rather than a complete source-control UI.

## Future possibility

Possible future extension points include richer source-control state, optional PTY infrastructure, richer Focused-Agent planning surfaces, additional documented provider adapters, richer Plugin presentation, and optional remote/cloud services if product direction changes.

Future workspace work should preserve the Editor-centered hierarchy and read this document together with TL_STUDIO_DESIGN_SYSTEM.md.
