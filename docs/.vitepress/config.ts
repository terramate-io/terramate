import type { HeadConfig } from 'vitepress'
import { defineConfig } from 'vitepress'
import { glob } from 'glob'

function getPath(path: string) {
  const uri = path.replace(/(?:(^|\/)index)?\.md$/, '$1')

  return uri === 'index' ? '' : uri
}

const ignoreFiles = [
  'cli/functions/index.md',
]

// dynamically constructs the sidebar for the Terraform functions.
const tfFunctionLinks = glob.sync('cli/functions/*.md', { ignore: ignoreFiles }).map((f: string): object => {
  const name = f.replace('.md', '').replace('cli/functions/', '')

  return {
    text: name,
    link: `/cli/functions/${name}`,
  }
}).sort().reverse()

// https://vitepress.dev/reference/site-config
export default defineConfig({
  title: 'Terramate',
  // titleTemplate: ':title - Terramate',
  description:
    'Terramate adds powerful capabilities such as code generation, stacks, orchestration, change detection, data sharing and more to Terraform.',
  sitemap: {
    hostname: 'https://terramate.io',
  },
  cleanUrls: true,
  transformHead: async ({ pageData, siteData }) => {
    const head: HeadConfig[] = [
      [
        'link',
        {
          rel: 'canonical',
          href: `https://terramate.io/docs/${getPath(
            pageData.relativePath,
          )}`,
        },
      ],
      // Google Tag Manager
      [
        'script',
        { id: 'register-gtm' },

        `(function(w,d,s,l,i){w[l]=w[l]||[];w[l].push({'gtm.start': new Date().getTime(),event:'gtm.js'});var f=d.getElementsByTagName(s)[0], j=d.createElement(s),dl=l!='dataLayer'?'&l='+l:'';j.async=true;j.src= 'https://www.googletagmanager.com/gtm.js?id='+i+dl;f.parentNode.insertBefore(j,f); })(window,document,'script','dataLayer','GTM-5KZT64L');`,
      ],
      [
        'link',
        {
          rel: 'icon',
          sizes: '48x48',
          href: `${siteData.base}favicons/favicon.ico`,
        },
      ],
      [
        'link',
        {
          rel: 'icon',
          type: 'image/svg+xml',
          sizes: 'any',
          href: `${siteData.base}favicons/favicon.svg`,
        },
      ],
      [
        'link',
        {
          rel: 'apple-touch-icon',
          href: `${siteData.base}favicons/apple-touch-icon.png`,
        },
      ],
      // [
      //   'link',
      //   {
      //     rel: 'manifest',
      //     href: `${siteData.base}favicons/site.webmanifest`,
      //   },
      // ],
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
      { text: 'Docs', link: '/cli/introduction' },
      { text: 'Blog', link: 'https://blog.terramate.io/' },
      { text: 'We are hiring!', link: 'https://jobs.ashbyhq.com/terramate' },
      {
        text: 'Releases',
        link: 'https://github.com/terramate-io/terramate/releases',
      },
      { text: 'Playground', link: 'https://play.terramate.io/' },
      { text: 'Terramate Cloud', link: 'https://terramate.io/' },
    ],

    sidebar: {
      // This sidebar gets displayed when a user
      // is on `cli` directory.
      '/': [
        {
          text: '👋 Get Started',
          collapsed: false,
          items: [
            { text: 'Introduction', link: '/cli/introduction' },
            { text: 'Installation', link: '/cli/installation' },
            { text: 'Quickstart', link: '/cli/getting-started/' },
          ],
        },
        {
          text: '⚙️ Configuration',
          collapsed: false,
          items: [
            { text: 'Terramate Configuration', link: '/cli/configuration/' },
            { text: 'Project Setup', link: '/cli/configuration/project-setup' },
            {
              text: 'Project Configuration',
              link: '/cli/configuration/project-config',
            },
            { text: 'Upgrade Check', link: '/cli/configuration/upgrade-check' },
          ],
        },
        {
          text: '📚 Stacks',
          collapsed: false,
          items: [
            { text: 'About Stacks', link: '/cli/about-stacks' },
            { text: 'Stack Configuration', link: '/cli/stacks/' },
            { text: 'Orchestration', link: '/cli/orchestration/' },
            { text: 'Tag Filter', link: '/cli/tag-filter' },
          ],
        },
        {
          text: '🕵️ Change Detection',
          collapsed: false,
          items: [
            { text: 'Stacks', link: '/cli/change-detection/#change-detection' },
            {
              text: 'Modules',
              link: '/cli/change-detection/#module-change-detection',
            },
            {
              text: 'Arbitrary Files',
              link: '/cli/change-detection/#arbitrary-files-change-detection',
            },
          ],
        },
        {
          text: '🔗 Sharing Data',
          collapsed: false,
          items: [
            { text: 'Overview', link: '/cli/data-sharing/' },
            { text: 'Globals', link: '/cli/data-sharing/globals' },
            { text: 'Metadata', link: '/cli/data-sharing/metadata' },
            { text: 'Map', link: '/cli/map' },
          ],
        },
        {
          text: '😍 Code Generation',
          collapsed: false,
          items: [
            { text: 'Overview', link: '/cli/code-generation/' },
            { text: 'Generate HCL', link: '/cli/code-generation/generate-hcl' },
            { text: 'Generate File', link: '/cli/code-generation/generate-file' },
          ],
        },
        {
          text: '🔧 Functions',
          collapsed: false,
          items: [
            {
              text: 'Overview',
              link: '/cli/functions/index',
            },
            {
              text: 'Terramate specific functions',
              items: [
                {
                  text: 'tm_ternary',
                  link: '/cli/functions/terramate-builtin/tm_ternary',
                },
                {
                  text: 'tm_hcl_expression',
                  link: '/cli/functions/terramate-builtin/tm_hcl_expression',
                },
                {
                  text: 'tm_version_match',
                  link: '/cli/functions/terramate-builtin/tm_version_match',
                },
                {
                  text: 'Experimental Functions',
                  items: [
                    {
                      text: 'tm_vendor',
                      link: '/cli/functions/terramate-builtin/tm_vendor',
                    },
                  ],
                },
              ],
            },
            {
              text: 'Terraform Functions',
              collapsed: true,
              items: tfFunctionLinks,
            },
          ],
        },
        {
          text: '💻 Commands (CLI)',
          collapsed: false,
          items: [
            { text: 'Overview', link: '/cli/cmdline/index' },
            { text: 'clone', link: '/cli/cmdline/clone' },
            { text: 'cloud login', link: '/cli/cmdline/cloud-login' },
            { text: 'cloud info', link: '/cli/cmdline/cloud-info' },
            { text: 'create', link: '/cli/cmdline/create' },
            { text: 'eval', link: '/cli/cmdline/eval' },
            { text: 'fmt', link: '/cli/cmdline/fmt' },
            { text: 'generate', link: '/cli/cmdline/generate' },
            { text: 'get-config-value', link: '/cli/cmdline/get-config-value' },
            { text: 'globals', link: '/cli/cmdline/globals' },
            { text: 'install-completions', link: '/cli/cmdline/install-completions' },
            { text: 'list', link: '/cli/cmdline/list' },
            { text: 'metadata', link: '/cli/cmdline/metadata' },
            { text: 'partial-eval', link: '/cli/cmdline/partial-eval' },
            { text: 'run-env', link: '/cli/cmdline/run-env' },
            { text: 'run-graph', link: '/cli/cmdline/run-graph' },
            { text: 'run-order', link: '/cli/cmdline/run-order' },
            { text: 'run', link: '/cli/cmdline/run' },
            { text: 'trigger', link: '/cli/cmdline/trigger' },
            { text: 'vendor download', link: '/cli/cmdline/vendor-download' },
            { text: 'version', link: '/cli/cmdline/version' },
          ],
        },
        {
          text: '💻 Editor Plugins',
          collapsed: false,
          items: [
            { text: 'Overview', link: '/cli/editor-plugins/index' },
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
          text: '🤓 Guides & Examples',
          link: '/cli/guides/',
        },
      ],
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/terramate-io/terramate' },
      { icon: 'discord', link: 'https://terramate.io/discord' },
      { icon: 'twitter', link: 'https://twitter.com/terramateio' },
      { icon: 'linkedin', link: 'https://www.linkedin.com/company/terramate-io/' },
    ],
  },
})
