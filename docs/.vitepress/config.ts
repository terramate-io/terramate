import type { HeadConfig } from 'vitepress'
import { defineConfig } from 'vitepress'

function getPath(path: string) {
  const uri = path.replace(/(?:(^|\/)index)?\.md$/, '$1')

  return uri === 'index' ? '' : uri
}

// https://vitepress.dev/reference/site-config
export default defineConfig({
  title: 'Terramate Docs',
  // titleTemplate: ':title - Terramate',
  description: 'Terramate CLI is an open-source Infrastructure as Code (IaC) orchestration tool for Terraform, OpenTofu, Terragrunt, Kubernetes, Pulumi, AWS Cloud Formation, AWS Cloud Development Kit (CDK), Azure Resource Manager (ARM), Biceps, and others',
  sitemap: {
    hostname: 'https://terramate.io/docs/',
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
      [
        'meta',
        {
          name: 'og:type',
          content: 'website'
        },
      ],
      [
        'meta',
        {
          name: 'og:image',
          content: `${siteData.base}terramate-og_linkedin.png`
        },
      ],
      [
        'meta',
        {
          name: 'twitter:image',
          content: `${siteData.base}terramate-og_twitter.png`
        },
      ],
      [
        'meta',
        {
          name: 'twitter:card',
          content: 'summary_large_image'
        },
      ],
      [
        'meta',
        {
          name: 'twitter:creator',
          content: '@terramateio'
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
      { text: 'Docs', link: '/introduction' },
      { text: 'Blog', link: 'https://terramate.io/rethinking-iac' },
      { text: 'We are hiring!', link: 'https://jobs.ashbyhq.com/terramate' },
      {
        text: 'Releases',
        link: 'https://github.com/terramate-io/terramate/releases',
      },
      { text: 'Playground', link: 'https://play.terramate.io/' },
      { text: 'Terramate Cloud', link: 'https://cloud.terramate.io/' },
    ],

    sidebar: {
      // '/cloud': [
      // ],
      // This sidebar gets displayed when a user
      // is on `cli` directory.
      '/': [
        {
          text: '👋 Introduction',
          items: [
            { text: 'About Terramate', link: '/introduction' },
            { text: 'How it works', link: '/how-it-works' },
            { text: 'Why Terramate', link: '/why-terramate' },
            { text: 'Product Roadmap', link: '/product-roadmap/' },
            { text: 'Community', link: 'https://terramate.io/discord' },
          ],
        },
        {
          text: '🚀 Getting Started',
          items: [
            { text: 'Installation', link: '/cli/installation' },
            { text: 'Quickstart', link: '/cli/getting-started/' },
            // { text: 'AWS Terraform Quickstart', link: '/cli/getting-started/' },
          ],
        },
        {
          text: '⭐ Concepts',
          items: [
            {
              text: 'Configuration',
              link: '/cli/configuration/',
            },
            {
              text: 'Projects',
              link: '/cli/projects/',
              collapsed: true,
              items: [
                { text: 'Create Projects', link: '/cli/projects/create' },
                { text: 'Configure a project', link: '/cli/projects/configuration' },
                { text: 'Start with Terraform', link: '/cli/projects/terraform' },
                { text: 'Start with Terragrunt', link: '/cli/projects/terragrunt' },
              ],
            },
            {
              text: 'Stacks',
              collapsed: true,
              link: '/cli/stacks/',
              items: [
                { text: 'Create Stacks', link: '/cli/stacks/create' },
                { text: 'Configure Stacks', link: '/cli/stacks/configuration' },
                { text: 'Clone Stacks', link: '/cli/stacks/clone' },
                { text: 'Manage Stacks', link: '/cli/stacks/manage' },
                { text: 'Delete Stacks', link: '/cli/stacks/delete' },
                { text: 'Nesting Stacks', link: '/cli/stacks/nesting' },
              ],
            },
            {
              text: 'Orchestration',
              collapsed: true,
              link: '/cli/orchestration/',
              items: [
                { text: 'Run Commands in Stacks', link: '/cli/orchestration/run-commands-in-stacks' },
                { text: 'Workflows', link: '/cli/orchestration/scripts' },
                { text: 'Tag Filter', link: '/cli/orchestration/tag-filter' },
                { text: 'Runtime Environment Configuration', link: '/cli/orchestration/runtime-configuration' },
                { text: 'Safe Guards', link: '/cli/orchestration/safeguards' },
              ],
            },
            {
              text: 'Change Detection',
              collapsed: true,
              link: '/cli/change-detection/',
              items: [
                {
                  text: 'Integrations',
                  collapsed: false,
                  items: [
                    { text: 'Git Integration', link: '/cli/change-detection/integrations/git' },
                    { text: 'Terraform Integration', link: '/cli/change-detection/integrations/terraform' },
                    { text: 'OpenTofu Integration', link: '/cli/change-detection/integrations/opentofu' },
                    { text: 'Terragrunt Integration', link: '/cli/change-detection/integrations/terragrunt' },
                  ],
                },
                { text: 'File Watchers', link: '/cli/change-detection/file-watchers' },
              ],
            },
            {
              text: 'Code Generation',
              collapsed: true,
              link: '/cli/code-generation/',
              items: [
                { text: 'Generate HCL', link: '/cli/code-generation/generate-hcl' },
                { text: 'Generate File', link: '/cli/code-generation/generate-file' },
                {
                  text: 'Variables',
                  collapsed: true,
                  items: [
                    { text: 'Overview', link: '/cli/code-generation/variables/' },
                    { text: 'Globals', link: '/cli/code-generation/variables/globals' },
                    { text: 'Lets', link: '/cli/code-generation/variables/lets' },
                    { text: 'Metadata', link: '/cli/code-generation/variables/metadata' },
                    { text: 'Map', link: '/cli/code-generation/variables/map' },
                  ],
                },
                {
                  text: 'Functions',
                  collapsed: true,
                  items: [
                    {
                      text: 'Overview',
                      link: '/cli/code-generation/functions/',
                    },
                    {
                      text: 'Terramate',
                      collapsed: true,
                      items: [
                        {
                          text: 'tm_ternary',
                          link: '/cli/code-generation/functions/tm_ternary',
                        },
                        {
                          text: 'tm_hcl_expression',
                          link: '/cli/code-generation/functions/tm_hcl_expression',
                        },
                        {
                          text: 'tm_version_match',
                          link: '/cli/code-generation/functions/tm_version_match',
                        },
                      ],
                    },
                    {
                      text: 'Numeric Functions',
                      collapsed: true,
                      items: [
                        {
                          text: 'tm_abs',
                          link: '/cli/code-generation/functions/tm_abs',
                        },
                        {
                          text: 'tm_ceil',
                          link: '/cli/code-generation/functions/tm_ceil',
                        },
                        {
                          text: 'tm_floor',
                          link: '/cli/code-generation/functions/tm_floor',
                        },
                        {
                          text: 'tm_log',
                          link: '/cli/code-generation/functions/tm_log',
                        },
                        {
                          text: 'tm_max',
                          link: '/cli/code-generation/functions/tm_max',
                        },
                        {
                          text: 'tm_min',
                          link: '/cli/code-generation/functions/tm_min',
                        },
                        {
                          text: 'tm_parseint',
                          link: '/cli/code-generation/functions/tm_parseint',
                        },
                        {
                          text: 'tm_pow',
                          link: '/cli/code-generation/functions/tm_pow',
                        },
                        {
                          text: 'tm_signum',
                          link: '/cli/code-generation/functions/tm_signum',
                        },
                      ],
                    },
                    {
                      text: 'String Functions',
                      collapsed: true,
                      items: [
                        {
                          text: 'tm_chomp',
                          link: '/cli/code-generation/functions/tm_chomp',
                        },
                        {
                          text: 'tm_format',
                          link: '/cli/code-generation/functions/tm_format',
                        },
                        {
                          text: 'tm_formatlist',
                          link: '/cli/code-generation/functions/tm_formatlist',
                        },
                        {
                          text: 'tm_indent',
                          link: '/cli/code-generation/functions/tm_indent',
                        },
                        {
                          text: 'tm_join',
                          link: '/cli/code-generation/functions/tm_join',
                        },
                        {
                          text: 'tm_lower',
                          link: '/cli/code-generation/functions/tm_lower',
                        },
                        {
                          text: 'tm_regex',
                          link: '/cli/code-generation/functions/tm_regex',
                        },
                        {
                          text: 'tm_regexall',
                          link: '/cli/code-generation/functions/tm_regexall',
                        },
                        {
                          text: 'tm_replace',
                          link: '/cli/code-generation/functions/tm_replace',
                        },
                        {
                          text: 'tm_split',
                          link: '/cli/code-generation/functions/tm_split',
                        },
                        {
                          text: 'tm_strrev',
                          link: '/cli/code-generation/functions/tm_strrev',
                        },
                        {
                          text: 'tm_substr',
                          link: '/cli/code-generation/functions/tm_substr',
                        },
                        {
                          text: 'tm_title',
                          link: '/cli/code-generation/functions/tm_title',
                        },
                        {
                          text: 'tm_trim',
                          link: '/cli/code-generation/functions/tm_trim',
                        },
                        {
                          text: 'tm_trimprefix',
                          link: '/cli/code-generation/functions/tm_trimprefix',
                        },
                        {
                          text: 'tm_trimsuffix',
                          link: '/cli/code-generation/functions/tm_trimsuffix',
                        },
                        {
                          text: 'tm_trimspace',
                          link: '/cli/code-generation/functions/tm_trimspace',
                        },
                        {
                          text: 'tm_upper',
                          link: '/cli/code-generation/functions/tm_upper',
                        },
                      ],
                    },
                    {
                      text: 'Collection Functions',
                      collapsed: true,
                      items: [
                        {
                          text: 'tm_alltrue',
                          link: '/cli/code-generation/functions/tm_alltrue',
                        },
                        {
                          text: 'tm_anytrue',
                          link: '/cli/code-generation/functions/tm_anytrue',
                        },
                        {
                          text: 'tm_chunklist',
                          link: '/cli/code-generation/functions/tm_chunklist',
                        },
                        {
                          text: 'tm_coalesce',
                          link: '/cli/code-generation/functions/tm_coalesce',
                        },
                        {
                          text: 'tm_coalescelist',
                          link: '/cli/code-generation/functions/tm_coalescelist',
                        },
                        {
                          text: 'tm_compact',
                          link: '/cli/code-generation/functions/tm_compact',
                        },
                        {
                          text: 'tm_concat',
                          link: '/cli/code-generation/functions/tm_concat',
                        },
                        {
                          text: 'tm_contains',
                          link: '/cli/code-generation/functions/tm_contains',
                        },
                        {
                          text: 'tm_distinct',
                          link: '/cli/code-generation/functions/tm_distinct',
                        },
                        {
                          text: 'tm_element',
                          link: '/cli/code-generation/functions/tm_element',
                        },
                        {
                          text: 'tm_flatten',
                          link: '/cli/code-generation/functions/tm_flatten',
                        },
                        {
                          text: 'tm_index',
                          link: '/cli/code-generation/functions/tm_index',
                        },
                        {
                          text: 'tm_keys',
                          link: '/cli/code-generation/functions/tm_keys',
                        },
                        {
                          text: 'tm_length',
                          link: '/cli/code-generation/functions/tm_length',
                        },
                        {
                          text: 'tm_lookup',
                          link: '/cli/code-generation/functions/tm_lookup',
                        },
                        {
                          text: 'tm_matchkeys',
                          link: '/cli/code-generation/functions/tm_matchkeys',
                        },
                        {
                          text: 'tm_merge',
                          link: '/cli/code-generation/functions/tm_merge',
                        },
                        {
                          text: 'tm_one',
                          link: '/cli/code-generation/functions/tm_one',
                        },
                        {
                          text: 'tm_range',
                          link: '/cli/code-generation/functions/tm_range',
                        },
                        {
                          text: 'tm_reverse',
                          link: '/cli/code-generation/functions/tm_reverse',
                        },
                        {
                          text: 'tm_setintersection',
                          link: '/cli/code-generation/functions/tm_setintersection',
                        },
                        {
                          text: 'tm_setproduct',
                          link: '/cli/code-generation/functions/tm_setproduct',
                        },
                        {
                          text: 'tm_setsubtract',
                          link: '/cli/code-generation/functions/tm_setsubtract',
                        },
                        {
                          text: 'tm_setunion',
                          link: '/cli/code-generation/functions/tm_setunion',
                        },
                        {
                          text: 'tm_slice',
                          link: '/cli/code-generation/functions/tm_slice',
                        },
                        {
                          text: 'tm_sort',
                          link: '/cli/code-generation/functions/tm_sort',
                        },
                        {
                          text: 'tm_sum',
                          link: '/cli/code-generation/functions/tm_sum',
                        },
                        {
                          text: 'tm_transpose',
                          link: '/cli/code-generation/functions/tm_transpose',
                        },
                        {
                          text: 'tm_values',
                          link: '/cli/code-generation/functions/tm_values',
                        },
                        {
                          text: 'tm_zipmap',
                          link: '/cli/code-generation/functions/tm_zipmap',
                        },
                      ],
                    },
                    {
                      text: 'Encoding Functions',
                      collapsed: true,
                      items: [
                        {
                          text: 'tm_base64decode',
                          link: '/cli/code-generation/functions/tm_base64decode',
                        },
                        {
                          text: 'tm_base64encode',
                          link: '/cli/code-generation/functions/tm_base64encode',
                        },
                        {
                          text: 'tm_base64gzip',
                          link: '/cli/code-generation/functions/tm_base64gzip',
                        },
                        {
                          text: 'tm_csvdecode',
                          link: '/cli/code-generation/functions/tm_csvdecode',
                        },
                        {
                          text: 'tm_jsondecode',
                          link: '/cli/code-generation/functions/tm_jsondecode',
                        },
                        {
                          text: 'tm_jsonencode',
                          link: '/cli/code-generation/functions/tm_jsonencode',
                        },
                        {
                          text: 'tm_textdecodebase64',
                          link: '/cli/code-generation/functions/tm_textdecodebase64',
                        },
                        {
                          text: 'tm_textencodebase64',
                          link: '/cli/code-generation/functions/tm_textencodebase64',
                        },
                        {
                          text: 'tm_urlencode',
                          link: '/cli/code-generation/functions/tm_urlencode',
                        },
                        {
                          text: 'tm_yamldecode',
                          link: '/cli/code-generation/functions/tm_yamldecode',
                        },
                        {
                          text: 'tm_yamlencode',
                          link: '/cli/code-generation/functions/tm_yamlencode',
                        },
                      ],
                    },
                    {
                      text: 'Filesystem Functions',
                      collapsed: true,
                      items: [
                        {
                          text: 'tm_abspath',
                          link: '/cli/code-generation/functions/tm_abspath',
                        },
                        {
                          text: 'tm_dirname',
                          link: '/cli/code-generation/functions/tm_dirname',
                        },
                        {
                          text: 'tm_pathexpand',
                          link: '/cli/code-generation/functions/tm_pathexpand',
                        },
                        {
                          text: 'tm_basename',
                          link: '/cli/code-generation/functions/tm_basename',
                        },
                        {
                          text: 'tm_file',
                          link: '/cli/code-generation/functions/tm_file',
                        },
                        {
                          text: 'tm_fileexists',
                          link: '/cli/code-generation/functions/tm_fileexists',
                        },
                        {
                          text: 'tm_fileset',
                          link: '/cli/code-generation/functions/tm_fileset',
                        },
                        {
                          text: 'tm_filebase64',
                          link: '/cli/code-generation/functions/tm_filebase64',
                        },
                        {
                          text: 'tm_templatefile',
                          link: '/cli/code-generation/functions/tm_templatefile',
                        },
                      ],
                    },
                    {
                      text: 'Date and Time Functions',
                      collapsed: true,
                      items: [
                        {
                          text: 'tm_formatdate',
                          link: '/cli/code-generation/functions/tm_formatdate',
                        },
                        {
                          text: 'tm_timeadd',
                          link: '/cli/code-generation/functions/tm_timeadd',
                        },
                        {
                          text: 'tm_timestamp',
                          link: '/cli/code-generation/functions/tm_timestamp',
                        },
                      ],
                    },
                    {
                      text: 'Hash and Crypto Functions',
                      collapsed: true,
                      items: [
                        {
                          text: 'tm_base64sha256',
                          link: '/cli/code-generation/functions/tm_base64sha256',
                        },
                        {
                          text: 'tm_base64sha512',
                          link: '/cli/code-generation/functions/tm_base64sha512',
                        },
                        {
                          text: 'tm_bcrypt',
                          link: '/cli/code-generation/functions/tm_bcrypt',
                        },
                        {
                          text: 'tm_filebase64sha256',
                          link: '/cli/code-generation/functions/tm_filebase64sha256',
                        },
                        {
                          text: 'tm_filebase64sha512',
                          link: '/cli/code-generation/functions/tm_filebase64sha512',
                        },
                        {
                          text: 'tm_filemd5',
                          link: '/cli/code-generation/functions/tm_filemd5',
                        },
                        {
                          text: 'tm_filesha1',
                          link: '/cli/code-generation/functions/tm_filesha1',
                        },
                        {
                          text: 'tm_filesha256',
                          link: '/cli/code-generation/functions/tm_filesha256',
                        },
                        {
                          text: 'tm_filesha512',
                          link: '/cli/code-generation/functions/tm_filesha512',
                        },
                        {
                          text: 'tm_md5',
                          link: '/cli/code-generation/functions/tm_md5',
                        },
                        {
                          text: 'tm_rsadecrypt',
                          link: '/cli/code-generation/functions/tm_rsadecrypt',
                        },
                        {
                          text: 'tm_sha1',
                          link: '/cli/code-generation/functions/tm_sha1',
                        },
                        {
                          text: 'tm_sha256',
                          link: '/cli/code-generation/functions/tm_sha256',
                        },
                        {
                          text: 'tm_sha512',
                          link: '/cli/code-generation/functions/tm_sha512',
                        },
                        {
                          text: 'tm_uuid',
                          link: '/cli/code-generation/functions/tm_uuid',
                        },
                        {
                          text: 'tm_uuidv5',
                          link: '/cli/code-generation/functions/tm_uuidv5',
                        },
                      ],
                    },
                    {
                      text: 'IP Network Functions',
                      collapsed: true,
                      items: [
                        {
                          text: 'tm_cidrhost',
                          link: '/cli/code-generation/functions/tm_cidrhost',
                        },
                        {
                          text: 'tm_cidrnetmask',
                          link: '/cli/code-generation/functions/tm_cidrnetmask',
                        },
                        {
                          text: 'tm_cidrsubnet',
                          link: '/cli/code-generation/functions/tm_cidrsubnet',
                        },
                        {
                          text: 'tm_cidrsubnets',
                          link: '/cli/code-generation/functions/tm_cidrsubnets',
                        },
                      ],
                    },
                    {
                      text: 'Type Conversion Functions',
                      collapsed: true,
                      items: [
                        {
                          text: 'tm_can',
                          link: '/cli/code-generation/functions/tm_can',
                        },
                        {
                          text: 'tm_tobool',
                          link: '/cli/code-generation/functions/tm_tobool',
                        },
                        {
                          text: 'tm_tolist',
                          link: '/cli/code-generation/functions/tm_tolist',
                        },
                        {
                          text: 'tm_tomap',
                          link: '/cli/code-generation/functions/tm_tomap',
                        },
                        {
                          text: 'tm_tonumber',
                          link: '/cli/code-generation/functions/tm_tonumber',
                        },
                        {
                          text: 'tm_toset',
                          link: '/cli/code-generation/functions/tm_toset',
                        },
                        {
                          text: 'tm_tostring',
                          link: '/cli/code-generation/functions/tm_tostring',
                        },
                        {
                          text: 'tm_try',
                          link: '/cli/code-generation/functions/tm_try',
                        },
                      ],
                    },
                  ],
                },
              ],
            },
          ],
        },
        {
          text: '🤖 Automating Terramate',
          link: '/cli/automation/',
          items: [
            { text: 'GitOps for Infrastructure', link: '/cli/automation/github-actions' },
            { text: 'GitHub Actions', link: '/cli/automation/github-actions' },
            { text: 'GitLab CI/CD (cooming soon)', link: '/cli/automation/gitlab' },
            { text: 'BitBucket Pipelines (cooming soon)', link: '/cli/automation/bitbucket' },
            { text: 'Azure DevOps (cooming soon)', link: '/cli/automation/azure-devops' },
          ],
        },
        {
          text: '☁️ Terramate Cloud',
          link: '/cloud/',
          items: [
            {
              text: 'Dashboard',
              link: '/cloud/dashboard/',
            },
            {
              text: 'Stacks',
              link: '/cloud/stacks/',
              collapsed: true,
              items: [
                { text: 'Stacks List', link: '/cloud/stacks/list' },
                { text: 'Stack Details', link: '/cloud/stacks/details' },
                { text: 'Stack Status', link: '/cloud/stacks/status' },
                { text: 'Synchronize Stacks', link: '/cloud/stacks/sync' },
              ],
            },
            {
              text: 'Deployments',
              link: '/cloud/deployments/',
              collapsed: true,
              items: [
                { text: 'Synchronize Deployments', link: '/cloud/deployments/sync' },
              ],
            },
            {
              text: 'Drift Management',
              collapsed: true,
              items: [
                // { text: 'Introduction', link: '/cli/introduction' },
              ],
            },
            {
              text: 'Notifications',
              collapsed: false,
              items: [
              // { text: 'Introduction', link: '/cli/introduction' },
              ],
            },
            {
              text: 'Organization',
              link: '/cloud/organization/',
              collapsed: true,
              items: [
                { text: 'General Settings', link: '/cloud/organization/settings' },
                { text: 'User Management', link: '/cloud/organization/user-management' },
              ],
            },
            // {
            //   text: '👟 Profile',
            //   collapsed: false,
            //   items: [
            //     // { text: 'Introduction', link: '/cli/introduction' },
            //   ],
            // },
            // {
            //   text: '🏠 Integrations',
            //   collapsed: false,
            //   items: [
            //     { text: 'Slack', link: '/cloud/integrations/slack/' },
            //     { text: 'GitHub Actions', link: '/cloud/integrations/github-actions/' },
            //     { text: 'Gitlab CI/CD', link: '/cloud/integrations/gitlab-cicd/' },
            //     { text: 'BitBucket Pipelines', link: '/cloud/integrations/bitbucket-pipelines/' },
            //   ],
            // },
          ],
        },
        {
          text: '👨🏽‍💻 Dev Tooling',
          collapsed: true,
          items: [
            { text: 'VSCode Extension', link: 'https://marketplace.visualstudio.com/items?itemName=Mineiros.terramate' },
            { text: 'VIM Plugin', link: 'https://github.com/terramate-io/vim-terramate' },
            { text: 'Language Server', link: '/cli/editor-plugins/language-server' },
          ],
        },
        {
          text: '⚙️ Configuration',
          collapsed: true,
          items: [
            { text: 'Terramate Configuration', link: '/cli/configuration/' },
            { text: 'Upgrade Check', link: '/cli/configuration/upgrade-check' },
          ],
        },
        {
          text: '💻 Commands (CLI)',
          link: '/cli/cmdline/',
          collapsed: true,
          items: [
            {
              text: 'Terramate Cloud',
              items: [
                { text: 'cloud login', link: '/cli/cmdline/cloud-login' },
                { text: 'cloud info', link: '/cli/cmdline/cloud-info' },
                { text: 'cloud drift show', link: '/cli/cmdline/cloud-drift-show' },
              ],
            },
            {
              text: 'Stacks',
              items: [
                { text: 'create', link: '/cli/cmdline/create' },
                { text: 'clone', link: '/cli/cmdline/clone' },
                { text: 'list', link: '/cli/cmdline/list' },
                { text: 'get-config-value', link: '/cli/cmdline/get-config-value' },
                { text: 'debug show metadata', link: '/cli/cmdline/metadata' },
              ],
            },
            {
              text: 'Orchestration',
              items: [
                { text: 'run', link: '/cli/cmdline/run' },
                { text: 'trigger', link: '/cli/cmdline/trigger' },
                { text: 'run-graph', link: '/cli/cmdline/run-graph' },
                { text: 'debug show runtime-env', link: '/cli/cmdline/run-env' },
              ],
            },
            {
              text: 'Scripts',
              items: [
                { text: 'script run', link: '/cli/cmdline/script/run' },
                { text: 'script info', link: '/cli/cmdline/script/info' },
                { text: 'script list', link: '/cli/cmdline/script/list' },
                { text: 'script tree', link: '/cli/cmdline/script/tree' },
              ],
            },
            {
              text: 'Code Generation',
              items: [
                { text: 'fmt', link: '/cli/cmdline/fmt' },
                { text: 'generate', link: '/cli/cmdline/generate' },
                { text: 'debug show globals', link: '/cli/cmdline/globals' },
              ],
            },
            {
              text: 'Misc',
              items: [
                { text: 'eval', link: '/cli/cmdline/eval' },
                { text: 'partial-eval', link: '/cli/cmdline/partial-eval' },
                { text: 'vendor download', link: '/cli/cmdline/vendor-download' },
                { text: 'install-completions', link: '/cli/cmdline/install-completions' },
                { text: 'version', link: '/cli/cmdline/version' },
              ],
            },
          ],
        },
        {
          text: '🤓 Tutorials',
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
