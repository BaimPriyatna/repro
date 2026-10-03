import { defineConfig } from 'vitepress'
import { withMermaid } from 'vitepress-plugin-mermaid'

export default withMermaid(
  defineConfig({
  title: 'Repro',
  description: 'Local-first observability and diagnostic system for development environments',
  base: '/repro/',
  cleanUrls: true,
  lastUpdated: true,
  ignoreDeadLinks: true,

  themeConfig: {
    logo: '/logo.svg',
    siteTitle: 'Repro',

    nav: [
      { text: 'Guide', link: '/getting-started/quickstart' },
      { text: 'Installation', link: '/getting-started/installation' },
      { text: 'CLI Reference', link: '/cli/overview' },
      { text: 'Architecture', link: '/concepts/architecture' },
      { text: 'Modules', link: '/modules/engine' },
      { text: 'API & Lib', link: '/api/overview' },
      {
        text: 'v1.1.6',
        items: [
          { text: 'Changelog', link: 'https://github.com/BaimPriyatna/repro/blob/main/CHANGELOG.md' },
          { text: 'Releases', link: 'https://github.com/BaimPriyatna/repro/releases' }
        ]
      }
    ],

    sidebar: [
      {
        text: 'Getting Started',
        collapsed: false,
        items: [
          { text: 'Introduction', link: '/' },
          { text: 'Installation', link: '/getting-started/installation' },
          { text: 'Quick Start', link: '/getting-started/quickstart' }
        ]
      },
      {
        text: 'Core Concepts',
        collapsed: false,
        items: [
          { text: 'Architecture', link: '/concepts/architecture' },
          { text: 'Snapshots & Events', link: '/concepts/snapshots' },
          { text: 'WhyBroken Diagnostic Chain', link: '/concepts/why-broken' }
        ]
      },
      {
        text: 'CLI Reference',
        collapsed: false,
        items: [
          { text: 'CLI Overview & Flags', link: '/cli/overview' },
          { text: 'Project Management', link: '/cli/project' },
          { text: 'Capture & Snapshots', link: '/cli/capture' },
          { text: 'Diff, History & Drift', link: '/cli/analysis' },
          { text: 'Root Cause & Operations', link: '/cli/operations' }
        ]
      },
      {
        text: 'Library & Integration',
        collapsed: false,
        items: [
          { text: 'Library Overview', link: '/api/overview' },
          { text: 'Usage Examples', link: '/api/examples' },
          { text: 'Error Codes', link: '/api/error-codes' }
        ]
      },
      {
        text: 'Modules',
        collapsed: true,
        items: [
          { text: 'Snapshot Engine', link: '/modules/engine' },
          { text: 'Repro Capture', link: '/modules/repro' },
          { text: 'TimeCapsule', link: '/modules/timecapsule' },
          { text: 'Watchdog', link: '/modules/watchdog' },
          { text: 'Depspy', link: '/modules/depspy' },
          { text: 'RepairMap', link: '/modules/repairmap' },
          { text: 'BeforeAfter', link: '/modules/beforeafter' },
          { text: 'Drift', link: '/modules/drift' },
          { text: 'Absent', link: '/modules/absent' },
          { text: 'ChangeMap', link: '/modules/changemap' },
          { text: 'Impact', link: '/modules/impact' },
          { text: 'DeadConfig', link: '/modules/deadconfig' },
          { text: 'Orphan', link: '/modules/orphan' },
          { text: 'GhostFile', link: '/modules/ghostfile' },
          { text: 'WhyBroken', link: '/modules/whybroken' },
          { text: 'ExplainDiff', link: '/modules/explaindiff' },
          { text: 'HumanReadable', link: '/modules/humanreadable' },
          { text: 'ManualTrace', link: '/modules/manualtrace' },
          { text: 'ConfigMerge', link: '/modules/configmerge' },
          { text: 'Central Store', link: '/modules/central' }
        ]
      }
    ],

    search: {
      provider: 'local',
      options: {
        detailedView: true
      }
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/BaimPriyatna/repro' }
    ],

    footer: {
      message: 'Released under the Apache 2.0 License.',
      copyright: 'Copyright © 2026 Repro Contributors'
    },

    editLink: {
      pattern: 'https://github.com/BaimPriyatna/repro/edit/main/docs/:path',
      text: 'Edit this page on GitHub'
    }
  })
)
