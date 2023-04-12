import { defineConfig } from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
  title: "Terramate",
  description: "A VitePress Site",
  themeConfig: {
    // https://vitepress.dev/reference/default-theme-config
    nav: [
      { text: 'Home', link: '/' },
      /* { text: 'Examples', link: '/markdown-examples' } */
    ],

    sidebar: [
      {
        text: '👋 What is Terramate',
        items: [
          { text: 'Overview', link: '/' },
        ]
      },
      {
        text: '🛠️ Getting Started',
        items: [
          { text: 'Installation', link: 'installation.md' },
          { text: 'Quick Start', link: 'getting-started.md' }
        ]
      },
      {
        text: '📚 Core Concepts',
        items: [
          { text: 'Stacks', link: 'stacks.md' },
          { text: 'Orchestration', link: 'orchestration.md' },
          { text: 'Change Detection', link: 'change-detection.md' },
          { text: 'Code Generation', items: [
             { text: 'Overview', link: 'codegen/overview.md' },
             { text: 'Generate HCL', link: 'codegen/generate-hcl.md' },
             { text: 'Generate File', link: 'codegen/generate-file.md' },
            ]
          },
          { text: 'Sharing Data', link: 'sharing-data.md' },
          { text: 'Functions', link: 'functions.md' },
          { text: 'Observability (coming soon)', link: '/' },
          /* { text: 'Import existing resources (coming soon)', link: '/' }, */
        ]
      },
      {
        text: '⚙️ Configuration',
        items: [
          { text: 'Telemetry', link: '/telemetry' },
        ]
      },
      {
        text: '🤓 Guides',
        items: [
          { text: 'Markdown Examples', link: '/markdown-examples' },
          { text: 'Runtime API Examples', link: '/api-examples' }
        ]
      },
      {
        text: '🥰 Community',
        items: [
          { text: 'Discord', link: 'https://terramate.io/discord' },
        ]
      },
      {
        text: '🤝🙇 Contributions',
        items: [
        ]
      }
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/mineiros-io/terramate' },
      { icon: 'discord', link: 'https://terramate.io/discord' },
      { icon: 'twitter', link: 'https://twitter.com/mineirosio' },
      { icon: 'linkedin', link: 'https://www.linkedin.com/company/terramate' },
    ]
  }
})
