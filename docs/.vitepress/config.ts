/**
 * Copyright 2023 Mineiros GmbH
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import type { HeadConfig } from 'vitepress'
import { defineConfig } from 'vitepress'

function getPath(path: string) {
  const uri = path.replace(/(?:(^|\/)index)?\.md$/, '$1')

  return uri === 'index' ? '' : uri
}

// https://vitepress.dev/reference/site-config
export default defineConfig({
  title: 'Terramate',
  description: 'Terramate adds powerful capabilities such as code generation, stacks, orchestration, change detection, data sharing and more to Terraform.',
  cleanUrls: true,
  transformHead: ({ pageData }) => {
    const head: HeadConfig[] = []
    head.push(['link', { rel: 'canonical', href: `https://terramate.io/docs/cli/${getPath(pageData.relativePath)}` }])

    return head
  },
  themeConfig: {
    // https://vitepress.dev/reference/default-theme-config#sitetitle
    siteTitle: '🦄 Terramate',
    // https://vitepress.dev/reference/default-theme-config#logo
    // logo: {
    //   light: '/assets/logo-white.svg',
    //   dark: '/assets/logo-dark.svg',
    //   alt: 'Terramate',
    // },

    // https://vitepress.dev/reference/site-config#appearance
    appearance: 'dark',

    // https://vitepress.dev/reference/default-theme-search#local-search
    search: {
      provider: 'local',
    },

    // https://vitepress.dev/reference/default-theme-config
    nav: [
      { text: 'Docs', link: '/about-stacks' },
      { text: 'Blog', link: 'https://blog.mineiros.io/' },
      { text: 'We are hiring!', link: 'https://jobs.ashbyhq.com/mineiros.io/' },
      { text: 'Releases', link: 'https://github.com/mineiros-io/terramate/releases' },
      { text: '💻 Playground', link: 'https://play.terramate.io/' },

    ],

    sidebar: [
      {
        text: '👋 What is Terramate',
        collapsed: false,
        items: [
          { text: 'Overview', link: '/' },
          { text: 'About Stacks', link: 'about-stacks' },
        ],
      },
      {
        text: '🛠️ Getting Started',
        collapsed: false,
        items: [
          { text: 'Installation', link: 'installation' },
          { text: 'Quick Start', link: 'getting-started/' },
        ],
      },
      {
        text: '📚 Stacks',
        collapsed: false,
        items: [
          { text: 'Stack Configuration', link: 'stacks/' },
          { text: 'Orchestration', link: 'orchestration/' },
          { text: 'Tag Filter', link: 'tag-filter' },
        ],
      },
      {
        text: '🕵️ Change Detection',
        collapses: false,
        items: [
          { text: 'Stacks', link: 'change-detection/#change-detection' },
          { text: 'Modules', link: 'change-detection/#module-change-detection' },
          { text: 'Arbitrary Files', link: 'change-detection/#arbitrary-files-change-detection' },
        ]
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
        items: [
          { text: 'tm_ternary', link: 'functions/#tm-ternary-bool-expr-expr-expr' },
          { text: 'tm_hcl_expression', link: 'functions/#tm-hcl-expression-string-expr' },
          { text: 'tm_version_match', link: 'functions/#tm-version-match-version-string-constraint-string-optional-arg-object' },
          { text: 'Experimental Functions', link: 'functions/#experimental-functions' },
        ],
      },
      {
        text: '⚙️ Configuration',
        collapsed: false,
        items: [
          { text: 'Terramate Configuration', link: 'configuration/' },
          { text: 'Project Setup', link: 'configuration/project-setup' },
          { text: 'Project Configuration', link: 'configuration/project-config' },
          { text: 'Upgrade Check', link: 'configuration/upgrade-check' },
        ],
      },
      {
        text: '🤓 Guides',
        collapsed: false,
        items: [
          { text: 'Introducing Terramate — An Orchestrator and Code Generator for Terraform', link: 'https://blog.mineiros.io/introducing-terramate-an-orchestrator-and-code-generator-for-terraform-5e538c9ee055' },
          { text: 'Understanding the basics of Terramate', link: 'https://blog.mineiros.io/understanding-the-basics-of-terramate-e0d8778f5c53' },
          { text: 'Terramate and Terragrunt', link: 'https://blog.mineiros.io/terramate-and-terragrunt-f27f2ec4032f' },
          { text: 'How to keep your Terraform code DRY by using Terramate', link: 'https://blog.mineiros.io/how-to-keep-your-terraform-code-dry-by-using-terramate-be5807fef8f6' },
          { text: 'Introducing the Terramate VSCode Extension and Language Server', link: 'https://blog.mineiros.io/introducing-the-terramate-vscode-extension-and-language-server-d77bd392011c' },
        ],
      },
      {
        items: [
          { text: '💬 Discord', link: 'https://terramate.io/discord' },
        ],
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
    ],
  },
})
