import { defineConfig, HeadConfig } from 'vitepress'

const getPath = (path: string) => {
  const uri = path.replace(/(?:(^|\/)index)?\.md$/, '$1')

  return uri === 'index' ? '' : uri
}

// https://vitepress.dev/reference/site-config
export default defineConfig({
  title: "Terramate",
  description: "A VitePress Site",
  cleanUrls: true,
  transformHead: ({ pageData }) => {
    const head: HeadConfig[] = []
    head.push(['link', { rel: 'canonical', content: `https://terramate.io/docs/${getPath(pageData.relativePath)}` }])

    return head
  },
  themeConfig: {
    // https://vitepress.dev/reference/default-theme-config
    nav: [
      {
        text: 'Home', link: '/'},
      {
        text: 'Playground', link: 'https://play.terramate.io',
      },
    ],

    sidebar: [
      {
        text: '👋 What is Terramate',
        items: [
          { text: 'Overview', link: '/' },
          { text: 'About Stacks', link: 'about-stacks.md' },
        ]
      },
      {
        text: '🛠️ Getting Started',
        collapsed: false,
        items: [
          { text: 'Installation', link: 'installation.md' },
          { text: 'Quick Start', link: 'getting-started.md' }
        ]
      },
      {
        text: '📚 Core Concepts',
        collapsed: false,
        items: [
          { text: 'Stacks', link: 'stacks.md' },
          { text: 'Orchestration', link: 'orchestration.md' },
          { text: 'Change Detection', link: 'change-detection.md' },
        ]
      },
      {
        text: '🔗 Sharing Data', link: 'sharing-data.md',
        collapsed: false,
        items : [
          {text: 'Globals', link: 'sharing-data.md#globals'},
          {text: 'Lazy Evaluation', link: 'sharing-data.md#lazy-evaluation'},
          {text: 'Metadata', link: 'sharing-data.md#metadata'},
          {text: 'Map', link: 'map.md'},
        ]
      },
      {
        text: '🔧 Functions', link: 'functions.md',
        items : [
          {text: 'tm_ternary', link: 'functions.md#tm-ternary-bool-expr-expr-expr'},
          {text: 'tm_hcl_expression', link: 'functions.md#tm-hcl-expression-string-expr'},
          {text: 'tm_version_match', link: 'functions.md#tm-version-match-version-string-constraint-string-optional-arg-object'},
          {text: 'Experimental Functions',link: 'functions.md#experimental-functions'},
        ]
      },
      {
        text: '😍 Code Generation',
        collapsed: false,
        items: [
           { text: 'Overview', link: 'codegen/overview.md' },
           { text: 'Generate HCL', link: 'codegen/generate-hcl.md' },
           { text: 'Generate File', link: 'codegen/generate-file.md' },
          ]
      },
      {
        text: '⚙️ Configuration',
        collapsed: false,
        items: [
          { text: 'Configuring Terramate', link: '/configuration.md' },
          { text: 'Project Configuration', link: '/project-config.md' },
          { text: 'Upgrade Check', link: '/upgrade-check.md' },
        ]
      },
      {
        text: '🤓 Guides',
        collapsed: false,
        items: [
          { text: 'Markdown Examples', link: '/markdown-examples' },
          { text: 'Runtime API Examples', link: '/api-examples' }
        ]
      },
      {
        items: [
          { text: '💬 Discord', link: 'https://terramate.io/discord' },
        ]
      },
      // {
      //   text: '🤝🙇 Contributions',
      //   items: [
      //   ]
      // }
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/mineiros-io/terramate' },
      { icon: 'discord', link: 'https://terramate.io/discord' },
      { icon: 'twitter', link: 'https://twitter.com/mineirosio' },
      { icon: 'linkedin', link: 'https://www.linkedin.com/company/terramate' },
    ]
  }
})
