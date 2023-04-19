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
        text: 'Docs', link: '/about-stacks'},
      {
        text: 'Playground', link: 'https://play.terramate.io',
      },
    ],

    sidebar: [
      {
        text: '👋 What is Terramate',
        items: [
          { text: 'Overview', link: '/' },
          { text: 'About Stacks', link: 'about-stacks' },
        ]
      },
      {
        text: '🛠️ Getting Started',
        collapsed: false,
        items: [
          { text: 'Installation', link: 'installation' },
          { text: 'Quick Start', link: 'getting-started' }
        ]
      },
      {
        text: '📚 Core Concepts',
        collapsed: false,
        items: [
          { text: 'Stacks', link: 'stacks' },
          { text: 'Orchestration', link: 'orchestration' },
          { text: 'Change Detection', link: 'change-detection' },
        ]
      },
      {
        text: '🔗 Sharing Data', link: 'sharing-data',
        collapsed: false,
        items : [
          {text: 'Globals', link: 'sharing-data#globals'},
          {text: 'Lazy Evaluation', link: 'sharing-data#lazy-evaluation'},
          {text: 'Metadata', link: 'sharing-data#metadata'},
          {text: 'Map', link: 'map'},
        ]
      },
      {
        text: '😍 Code Generation', link: 'code-generation/',
        collapsed: false,
        items: [
           { text: 'Overview', link: 'code-generation/' },
           { text: 'Generate HCL', link: 'code-generation/generate-hcl' },
           { text: 'Generate File', link: 'code-generation/generate-file' },
          ]
      },
      {
        text: '🔧 Functions', link: 'functions',
        items : [
          {text: 'tm_ternary', link: 'functions#tm-ternary-bool-expr-expr-expr'},
          {text: 'tm_hcl_expression', link: 'functions#tm-hcl-expression-string-expr'},
          {text: 'tm_version_match', link: 'functions#tm-version-match-version-string-constraint-string-optional-arg-object'},
          {text: 'Experimental Functions',link: 'functions#experimental-functions'},
        ]
      },
      {
        text: '⚙️ Configuration',
        collapsed: false,
        items: [
          { text: 'Configuring Terramate', link: '/configuration' },
          { text: 'Project Configuration', link: '/project-config' },
          { text: 'Upgrade Check', link: '/upgrade-check' },
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
