import type { HeadConfig } from 'vitepress'
import { defineConfig } from 'vitepress'

function getPath(path: string) {
  const uri = path.replace(/(?:(^|\/)index)?\.md$/, '$1')

  return uri === 'index' ? '' : uri
}

// https://vitepress.dev/reference/site-config
export default defineConfig({
  title: 'Terramate',
  // titleTemplate: ':title - Terramate',
  description:
    'Terramate adds powerful capabilities such as code generation, stacks, orchestration, change detection, data sharing and more to Terraform.',
  cleanUrls: true,
  transformHead: ({ pageData, siteData }) => {
    const head: HeadConfig[] = [
      [
        'link',
        {
          rel: 'canonical',
          href: `https://terramate.io/docs/cli/${getPath(
            pageData.relativePath,
          )}`,
        },
      ],
      // Google Tag Manager
      [
        'script',
        { id: 'register-gtm' },
        // eslint-disable-next-line @typescript-eslint/quotes
        `(function(w,d,s,l,i){w[l]=w[l]||[];w[l].push({'gtm.start': new Date().getTime(),event:'gtm.js'});var f=d.getElementsByTagName(s)[0], j=d.createElement(s),dl=l!='dataLayer'?'&l='+l:'';j.async=true;j.src= 'https://www.googletagmanager.com/gtm.js?id='+i+dl;f.parentNode.insertBefore(j,f); })(window,document,'script','dataLayer','GTM-5KZT64L');`,
      ],
      [
        'link',
        {
          rel: 'apple-touch-icon',
          sizes: '180x180',
          href: `${siteData.base}favicons/apple-touch-icon.png`,
        },
      ],
      [
        'link',
        {
          rel: 'icon',
          type: 'image/png',
          sizes: '32x32',
          href: `${siteData.base}favicons/favicon-32x32.png`,
        },
      ],
      [
        'link',
        {
          rel: 'icon',
          type: 'image/png',
          sizes: '16x16',
          href: `${siteData.base}favicons/favicon-16x16.png`,
        },
      ],
      [
        'link',
        {
          rel: 'manifest',
          href: `${siteData.base}favicons/site.webmanifest`,
        },
      ],
      [
        'link',
        {
          rel: 'mask-icon',
          href: `${siteData.base}favicons/safari-pinned-tab.svg`,
          color: '#3a0839',
        },
      ],
      [
        'link',
        {
          rel: 'shortcut icon',
          href: `${siteData.base}favicons/favicon.ico`,
        },
      ],
      // ["meta", { name: "msapplication-TileColor", content: "#3a0839" }],
      // [
      //   "meta",
      //   {
      //     name: "msapplication-config",
      //     content: "/favicons/browserconfig.xml",
      //   },
      // ],
      // ["meta", { name: "theme-color", content: "#ffffff" }],
    ]

    return head
  },
  // https://vitepress.dev/reference/site-config#appearance
  appearance: 'dark',
  themeConfig: {
    // https://vitepress.dev/reference/default-theme-config#sitetitle
    // siteTitle: '🦄 Terramate',
    siteTitle: false,
    // https://vitepress.dev/reference/default-theme-config#logo
    logo: {
      light: '/logo-white.svg',
      dark: '/logo-dark.svg',
      alt: 'Terramate',
    },

    // https://vitepress.dev/reference/default-theme-search#local-search
    search: {
      provider: 'local',
    },

    // https://vitepress.dev/reference/default-theme-config
    nav: [
      { text: 'Docs', link: '/introduction' },
      { text: 'Blog', link: 'https://blog.terramate.io/' },
      { text: 'We are hiring!', link: 'https://jobs.ashbyhq.com/terramate' },
      {
        text: 'Releases',
        link: 'https://github.com/terramate-io/terramate/releases',
      },
      { text: '💻 Playground', link: 'https://play.terramate.io/' },
    ],

    sidebar: [
      {
        text: '👋 Get Started',
        collapsed: false,
        items: [
          { text: 'Introduction', link: 'introduction' },
          { text: 'Installation', link: 'installation' },
          { text: 'Quickstart', link: 'getting-started/' },
        ],
      },
      {
        text: '📚 Stacks',
        collapsed: false,
        items: [
          { text: 'About Stacks', link: 'about-stacks' },
          { text: 'Stack Configuration', link: 'stacks/' },
          { text: 'Orchestration', link: 'orchestration/' },
          { text: 'Tag Filter', link: 'tag-filter' },
        ],
      },
      {
        text: '🕵️ Change Detection',
        collapsed: false,
        items: [
          { text: 'Stacks', link: 'change-detection/#change-detection' },
          {
            text: 'Modules',
            link: 'change-detection/#module-change-detection',
          },
          {
            text: 'Arbitrary Files',
            link: 'change-detection/#arbitrary-files-change-detection',
          },
        ],
      },
      {
        text: '🔗 Sharing Data',
        link: 'data-sharing',
        collapsed: false,
        items: [
          { text: 'Globals', link: 'data-sharing/#globals' },
          { text: 'Lazy Evaluation', link: 'data-sharing/#lazy-evaluation' },
          { text: 'Metadata', link: 'data-sharing/#metadata' },
          { text: 'Map', link: 'map' },
        ],
      },
      {
        text: '😍 Code Generation',
        link: 'code-generation/',
        collapsed: false,
        items: [
          { text: 'Overview', link: 'code-generation/' },
          { text: 'Generate HCL', link: 'code-generation/generate-hcl' },
          { text: 'Generate File', link: 'code-generation/generate-file' },
        ],
      },
      {
        text: '🔧 Functions',
        link: 'functions',
        collapsed: false,
        items: [
          {
            text: 'tm_ternary',
            link: 'functions/#tm-ternary-bool-expr-expr-expr',
          },
          {
            text: 'tm_hcl_expression',
            link: 'functions/#tm-hcl-expression-string-expr',
          },
          {
            text: 'tm_version_match',
            link: 'functions/#tm-version-match-version-string-constraint-string-optional-arg-object',
          },
          {
            text: 'Experimental Functions',
            link: 'functions/#experimental-functions',
          },
        ],
      },
      {
        text: '⚙️ Configuration',
        collapsed: false,
        items: [
          { text: 'Terramate Configuration', link: 'configuration/' },
          { text: 'Project Setup', link: 'configuration/project-setup' },
          {
            text: 'Project Configuration',
            link: 'configuration/project-config',
          },
          { text: 'Upgrade Check', link: 'configuration/upgrade-check' },
        ],
      },
      {
        text: '💻 Commands (CLI)',
        link: 'cmdline/',
        collapsed: false,
        items: [
          { text: 'clone', link: 'cmdline/clone' },
          { text: 'cloud login', link: 'cmdline/cloud-login' },
          { text: 'cloud info', link: 'cmdline/cloud-info' },
          { text: 'create', link: 'cmdline/create' },
          { text: 'eval', link: 'cmdline/eval' },
          { text: 'fmt', link: 'cmdline/fmt' },
          { text: 'generate', link: 'cmdline/generate' },
          { text: 'get-config-value', link: 'cmdline/get-config-value' },
          { text: 'globals', link: 'cmdline/globals' },
          { text: 'install-completions', link: 'cmdline/install-completions' },
          { text: 'list', link: 'cmdline/list' },
          { text: 'partial-eval', link: 'cmdline/partial-eval' },
          { text: 'run-env', link: 'cmdline/run-env' },
          { text: 'run-graph', link: 'cmdline/run-graph' },
          { text: 'run-order', link: 'cmdline/run-order' },
          { text: 'run', link: 'cmdline/run' },
          { text: 'trigger', link: 'cmdline/trigger' },
          { text: 'vendor download', link: 'cmdline/vendor-download' },
          { text: 'version', link: 'cmdline/version' },
        ],
      },
      // {
      //   text: 'Misc',
      //   collapsed: false,
      //   items: [
      //     { text: 'Language Server', link: '' },
      //     { text: 'VSCode Extension', link: '' },
      //   ],
      // },
      {
        text: '🤓 Guides',
        collapsed: false,
        items: [
          {
            text: 'Introducing Terramate — An Orchestrator and Code Generator for Terraform',
            link: 'https://blog.terramate.io/introducing-terramate-an-orchestrator-and-code-generator-for-terraform-5e538c9ee055',
          },
          {
            text: 'Understanding the basics of Terramate',
            link: 'https://blog.terramate.io/understanding-the-basics-of-terramate-e0d8778f5c53',
          },
          {
            text: 'Terramate and Terragrunt',
            link: 'https://blog.terramate.io/terramate-and-terragrunt-f27f2ec4032f',
          },
          {
            text: 'How to keep your Terraform code DRY by using Terramate',
            link: 'https://blog.terramate.io/how-to-keep-your-terraform-code-dry-by-using-terramate-be5807fef8f6',
          },
          {
            text: 'Introducing the Terramate VSCode Extension and Language Server',
            link: 'https://blog.terramate.io/introducing-the-terramate-vscode-extension-and-language-server-d77bd392011c',
          },
        ],
      },
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/terramate-io/terramate' },
      { icon: 'discord', link: 'https://terramate.io/discord' },
      { icon: 'twitter', link: 'https://twitter.com/terramateio' },
      { icon: 'linkedin', link: 'https://www.linkedin.com/company/terramate-io/' },
    ],
  },
})
