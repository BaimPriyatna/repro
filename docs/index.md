---
layout: home

hero:
  name: "Repro"
  text: "Local-First Observability & Diagnostics"
  tagline: "Capture state. Compare history. Trace evidence. Explain what broke in development environments — zero cloud required."
  image:
    src: /logo.svg
    alt: "Repro Terminal Logo R>"
  actions:
    - theme: brand
      text: Get Started
      link: /getting-started/quickstart
    - theme: alt
      text: Installation
      link: /getting-started/installation
    - theme: alt
      text: CLI Reference
      link: /cli/overview
    - theme: alt
      text: View on GitHub
      link: https://github.com/BaimPriyatna/repro

features:
  - icon: '<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/><circle cx="12" cy="13" r="4"/></svg>'
    title: Observable Snapshots
    details: Content-hashed, immutable, and comparable environment state snapshots shared across Repro, TimeCapsule, and Watchdog.
  - icon: '<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>'
    title: Evidence-Backed Analysis
    details: Diagnostic findings carry forensic evidence with separate axes for severity and confidence to prevent false certainties.
  - icon: '<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/><line x1="8.59" y1="13.51" x2="15.42" y2="17.49"/><line x1="15.41" y1="6.51" x2="8.59" y2="10.49"/></svg>'
    title: Dependency & Usage Graphs
    details: Structural graph analyzers (Depspy and RepairMap) feed impact assessments, root-cause detection, and configuration drift.
  - icon: '<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>'
    title: Explanation Chain
    details: WhyBroken to ExplainDiff to HumanReadable narrative chain, strictly citing prior evidence without generative hallucinations.
  - icon: '<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>'
    title: Privacy by Default
    details: Environment variables, tokens, and secrets are strictly excluded from state captures unless you explicitly opt in.
  - icon: '<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="2" width="20" height="8" rx="2" ry="2"/><rect x="2" y="14" width="20" height="8" rx="2" ry="2"/><line x1="6" y1="6" x2="6.01" y2="6"/><line x1="6" y1="18" x2="6.01" y2="18"/></svg>'
    title: Multi-Project & Standalone
    details: Central storage with anchor chain resolution, project lifecycle tracking, and zero-config standalone mode for CI/CD pipelines.
---
