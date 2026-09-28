# TL Studio documentation

- ARCHITECTURE.md describes the current fully native TL Studio launcher, Browser workspace, Agent/session/provider/tool ownership, process/Terminal foundation, Preview isolation, release shape, and security boundaries.
- design/MAIN_WORKSPACE.md documents the Editor-centered workspace hierarchy and interaction model.
- design/TL_STUDIO_DESIGN_SYSTEM.md records the implemented visual tokens, density, states, controls, accessibility, and responsive desktop rules.

Product-facing code should use TL Studio-owned /local/* semantic contracts. The historical compatibility-runtime API is no longer a product boundary.
