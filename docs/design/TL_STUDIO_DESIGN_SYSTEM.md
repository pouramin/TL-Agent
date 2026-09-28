# TL Studio — Design System

> Product-design reference for the real TL Studio workspace. Exact implementation values below are aligned with the current main-workspace redesign. The repository implementation remains authoritative.

## Brand foundation

Canonical brand anchors:

| Token | Value | Role |
| --- | --- | --- |
| Brand Navy | #212E4E | product identity, status bar, selected/elevated brand surfaces |
| Brand Green | #008036 | primary action and focus accent |
| Brand Green Hover | #009B43 | primary-action hover |
| Black | #000000 | brand asset support |
| White | #FFFFFF | brand asset support and light surfaces |

Green is controlled. It is never the dominant application background.

## Dark theme tokens

| Token | Value |
| --- | --- |
| Background | #0B0F17 |
| Background Secondary | #101725 |
| Panel | #151D2D |
| Elevated Panel | #1A2438 |
| Elevated Strong | #202B41 |
| Border | #29344A |
| Text Primary | #F5F7FA |
| Text Secondary | #9BA6B5 |
| Text Muted | #697587 |
| Success | #4BC579 |
| Warning | #D5A94C |
| Error | #E46B73 |
| Agent contextual | #6F9FD8 |

Pure black is not the primary dark workspace surface.

## Light theme tokens

| Token | Value |
| --- | --- |
| Background | #F4F6FA |
| Background Secondary | #EDF1F7 |
| Panel | #FFFFFF |
| Elevated Panel | #F7F9FC |
| Elevated Strong | #E9EEF6 |
| Border | #D7DEEA |
| Text Primary | #172033 |
| Text Secondary | #58657A |
| Text Muted | #7B8798 |
| Agent contextual | #4779B4 |
| Warning | #9A6B16 |
| Error | #B94751 |

The light theme uses cool navy-influenced neutrals rather than disconnected white/gray styling.

## Surfaces

Persistent workspace surfaces remain flat and structural:

- Global Top Bar: Background Secondary.
- Activity Rail: Background Secondary.
- Context Sidebar: Background Secondary.
- Editor: Background.
- Agent: Background Secondary.
- Terminal: Background / Background Secondary.
- Status Bar: Brand Navy.

Elevation/shadow is reserved primarily for floating surfaces such as Preview, Command Palette and modal dialogs.

## Typography

### UI

System-first compact UI stack:

Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif.

Existing user UI-font preferences remain authoritative where the product already applies them.

### Code

Monaco preserves the real editor font preference and editor configuration.

### Terminal

Terminal typography remains monospace and distinct from general UI typography.

Typical fallback family:

ui-monospace, SFMono-Regular, Consolas, monospace.

## Density

TL Studio is designed for long-running development work and uses moderate-to-high information density.

Current principles:

- top bar: 44px.
- status bar: 22px.
- activity rail: 54px.
- common compact controls: roughly 24–32px.
- compact metadata.
- limited decorative whitespace.
- Editor space is preferred over secondary panels.

## Spacing

The workspace primarily uses a compact 3–12px spacing rhythm.

Typical values:

- 3–5px for tightly related controls.
- 6–8px for internal compact padding.
- 9–12px for panel/header content.
- larger spacing is reserved for modal/empty-state content.

## Borders

Structural separation uses 1px semantic borders.

Primary dark border: #29344A.

Selected states should combine fill/indicator/text where practical rather than relying only on a brighter border.

## Radii

The redesign intentionally avoids oversized SaaS-style rounding.

Typical values:

- 3–5px for markers and small controls.
- 5–7px for buttons, inputs and persistent panels.
- 8px for floating Preview, Command Palette and modal surfaces.

## Shadows and elevation

Persistent workspace panels normally have no large shadow.

Current elevated surfaces use restrained dark shadows:

- floating Preview.
- Command Palette.
- Settings and other dialogs.

The Command Palette currently uses an approximately 22px/70px soft black elevation.

## Focus

Keyboard focus uses a visible green-derived outline in both themes.

Focus state is not communicated only through color change in content.

## Hover

Hover generally uses the next elevated neutral surface.

Primary green actions use #009B43 on hover.

No glow, neon or animated AI decoration is used.

## Active and selected

### Activity Rail

Active activity:

- navy-derived selected fill.
- explicit green left edge.
- brighter foreground.

### Editor tabs

Active tab:

- Editor background.
- explicit green top indicator.

### File rows

Selected Explorer row:

- navy-derived fill.
- green-influenced border.

### Agent modes

Compact/Focused header controls use neutral active fill and readable text.

## Disabled

Disabled product controls preserve their shape but reduce visual emphasis and pointer affordance.

Important disabled/error state is not represented by opacity alone when explanatory text is necessary.

## Loading

Existing feature-level loading behavior remains authoritative:

- Explorer/search loading text.
- Monaco loading hint.
- Preview starting state.
- Agent working state.

The redesign does not add decorative global spinners.

## Success, warning and error

Status colors are restrained:

- success/local: green.
- warning/waiting/dirty: amber.
- error/conflict/failure: restrained red.
- Agent context: cool blue.

Critical state must also include text, an icon/marker, or another non-color signal.

## Agent states

### Collapsed

48px presentation rail with Agent identity/state.

### Compact

Normal working width, persisted by the user. Minimum expanded width is 280px.

### Focused

A wider planning/review presentation while preserving Editor access.

The state affects presentation only, not Agent/runtime semantics.

## File and change states

### Dirty

Uses the existing dirty tab marker plus editor status text.

### External change

Uses the existing external-change marker and warning status.

### Agent modified

Open tabs that match current semantic session change paths receive an explicit A marker and Agent contextual treatment.

### Conflict

Save conflict continues to use explicit text/decision flow rather than color-only indication.

### Deleted

Existing reconciliation behavior is retained; the redesign does not invent a new unsupported filesystem state model.

## Icons

The current real implementation uses compact textual/iconographic controls already present in the app plus restrained workspace symbols.

Rules:

- icons must have accessible names/tooltips when meaning is not visible in text.
- no robot, brain or sparkle imagery.
- no decorative AI iconography.
- activity/status badges remain small.

A future dedicated icon set may replace temporary textual symbols without changing workspace architecture.

## Buttons

### Primary

Brand Green fill, white text.

Used for high-confidence primary actions such as Send and existing confirmed actions.

### Secondary

Neutral surface + border.

### Ghost

Transparent/neutral controls for contextual operations.

### Dangerous

Use existing restrained red/danger treatment only for genuinely destructive/reject actions.

## Icon buttons

Common workspace icon buttons are approximately 25–28px.

They use neutral surfaces and must retain a visible focus state.

## Inputs and search controls

Inputs use:

- semantic panel background.
- 1px border.
- restrained 5–6px radius.
- visible green-derived focus treatment.

Project Search remains dense enough to coexist with the Editor.

## Selectors

Agent/model selectors remain real HTML selects connected to existing state.

They are compact in the Agent composer and preserve current provider/model behavior.

## Menus and Command Palette

The Command Palette is a floating elevated surface.

Current width: up to about 600px.

It contains real actions only and supports Ctrl/Cmd + K plus Escape dismissal.

## Tabs

Editor tabs remain horizontally scrollable and preserve active/dirty/external state.

Active tab uses a green top indicator.

Agent-modified tabs add a small explicit A marker.

## File rows

File rows prioritize:

1. readable name/hierarchy.
2. selected state.
3. filesystem type/state.
4. secondary size/type metadata.

Rows stay compact to preserve vertical scanning density.

## Panels and panel headers

Persistent panels are flat and separated by structural borders.

Typical header height is around 32–43px depending on context.

Secondary actions remain compact and contextual.

## Splitters

Current splitters:

- Context Sidebar vertical splitter.
- TL Agent vertical splitter.
- Terminal horizontal splitter.

Visual hit area is 6px.

Pointer lifecycle continues at document level after pointerdown.

Hover/active state uses a controlled green line.

Minimums:

- Context Sidebar: 180px.
- TL Agent: 280px expanded.
- Terminal: 96px.
- Editor target: approximately 420px useful width.

## Status Bar

Height: 22px.

Background: Brand Navy.

It displays quiet reliable workspace state and avoids large badges/cards.

## Agent messages

Conversation remains semantically identical to the real Agent implementation.

Presentation goals:

- user/assistant hierarchy is obvious.
- tool/reasoning cards remain compact.
- final response remains more prominent than low-level activity.
- error state stays readable without taking over the workspace.

## Activity and tool rows

Tool/reasoning activity remains expandable and uses TL Studio Tool Registry metadata when available.

Operational detail is secondary to the user-facing response.

## Permission cards and questions

Existing real permission/question semantics remain authoritative.

Cards should answer:

- what is requested.
- what it affects.
- whether it is sensitive.
- what choices the user has.

Destructive/sensitive requests receive stronger treatment only when justified by the underlying permission semantics.

## Terminal

The Terminal is integrated visually with the workspace rather than presented as a detached floating card.

Characteristics:

- monospace.
- visible current project path.
- restrained output colors.
- real Run/Stop/Clear/history behavior.
- no fake PTY/debugger affordances.

## Live Preview

Preview remains a floating elevated tool window.

Characteristics:

- real capability-driven content.
- draggable title/header.
- N/NE/E/SE/S/SW/W/NW resizing.
- persisted position and size.
- viewport clamping.
- local URL/status presentation.
- existing Start/Stop/Reload/Open controls.

Its resize handles use controlled green feedback only during hover/resize.

## Empty, error and loading states

Empty states should be compact and task-oriented.

Error states must provide actionable text.

Loading states should preserve layout and avoid large skeleton/dashboard treatments.

## Motion

Allowed motion is functional:

- panel size changes.
- Preview movement/resize.
- transient status feedback.
- short open/close transitions where already present.

Avoid continuous decorative animation, glow or AI sparkle effects.

## Accessibility

The workspace preserves or improves:

- keyboard navigation.
- visible focus.
- readable contrast.
- reasonable hit areas.
- accessible labels/tooltips.
- semantic buttons/inputs.
- state not communicated by color alone.

## Responsive desktop behavior

Primary target: 1440 × 900.

Current CSS includes desktop breakpoints at approximately:

- 1350px: reduce top-bar/secondary density.
- 1120px: hide Context Sidebar to preserve Editor.
- 880px: force Agent rail/collapsed presentation and simplify top bar.

TL Studio remains desktop-first. Mobile is not the primary workspace target.

## Visual styles intentionally avoided

Do not introduce:

- neon/cyberpunk styling.
- purple AI gradients.
- excessive glassmorphism.
- glow-heavy UI.
- oversized SaaS cards.
- giant rounded containers.
- robot/brain/sparkle AI decoration.
- noisy gradients.
- permanent oversized chat dominance.
