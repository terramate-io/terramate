---
layout: home

title: 'Terramate: Supercharge Terraform with Stacks and Code Generation'
titleTemplate: false
description: Terramate adds powerful capabilities such as stacks, code generation, orchestration, change detection, data sharing and more to Terraform.

hero:
  name: Terramate
  text: Code Generator and Orchestrator for Terraform
  tagline: Terramate adds powerful capabilities such as code generation, stacks, orchestration, change detection, data sharing and more to Terraform.
  # image:
  #   src: https://picsum.photos/1000/1000
  #   alt: VitePress
  actions:
    - theme: brand
      text: Get Started
      link: /cli/getting-started/
    - theme: alt
      text: View on GitHub
      link: https://github.com/terramate-io/terramate

features:
  - icon: <svg xmlns="http://www.w3.org/2000/svg" width="256" height="256" viewBox="0 0 256 256"><path fill="currentColor" d="M230.91 172a8 8 0 0 1-2.91 10.91l-96 56a8 8 0 0 1-8.06 0l-96-56A8 8 0 0 1 36 169.09l92 53.65l92-53.65a8 8 0 0 1 10.91 2.91ZM220 121.09l-92 53.65l-92-53.65a8 8 0 0 0-8 13.82l96 56a8 8 0 0 0 8.06 0l96-56a8 8 0 1 0-8.06-13.82ZM24 80a8 8 0 0 1 4-6.91l96-56a8 8 0 0 1 8.06 0l96 56a8 8 0 0 1 0 13.82l-96 56a8 8 0 0 1-8.06 0l-96-56A8 8 0 0 1 24 80Zm23.88 0L128 126.74L208.12 80L128 33.26Z"/></svg>
    title: Stacks
    details: Easily split up your Terraform state into multiple isolated stacks.
  - icon: <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 32 32"><path fill="currentColor" d="M6 13V7.414l9 9V28h2V16.414l9-9V13h2V4h-9v2h5.586L16 14.586L7.414 6H13V4H4v9h2z"/></svg>
    title: Orchestration & Change Detection
    details: Orchestrate your stacks so only stacks that have changed within a specific pull request are executed.
  - icon: <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="currentColor" d="M18 22q-1.25 0-2.125-.875T15 19q0-.175.025-.363t.075-.337l-7.05-4.1q-.425.375-.95.588T6 15q-1.25 0-2.125-.875T3 12q0-1.25.875-2.125T6 9q.575 0 1.1.213t.95.587l7.05-4.1q-.05-.15-.075-.337T15 5q0-1.25.875-2.125T18 2q1.25 0 2.125.875T21 5q0 1.25-.875 2.125T18 8q-.575 0-1.1-.212t-.95-.588L8.9 11.3q.05.15.075.338T9 12q0 .175-.025.363T8.9 12.7l7.05 4.1q.425-.375.95-.587T18 16q1.25 0 2.125.875T21 19q0 1.25-.875 2.125T18 22Z"/></svg>
    title: Data Sharing
    details: Global variables empower you to share data between a bunch of stacks.
  - icon: <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="currentColor" d="M15 5.25A3.25 3.25 0 0 0 18.25 2h1.5A3.25 3.25 0 0 0 23 5.25v1.5A3.25 3.25 0 0 0 19.75 10h-1.5A3.25 3.25 0 0 0 15 6.75v-1.5ZM4 7a2 2 0 0 1 2-2h7V3H6a4 4 0 0 0-4 4v10a4 4 0 0 0 4 4h12a4 4 0 0 0 4-4v-5h-2v5a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V7Z"/></svg>
    title: Code Generation
    details: Generate any Terraform or HCL code to keep your configuration DRY.
  - icon: <svg xmlns="http://www.w3.org/2000/svg" width="36" height="36" viewBox="0 0 36 36"><path id="clarityCiCdLine0" fill="currentColor" d="M23.53 19.81a7.45 7.45 0 0 1-1.65-.18a10.48 10.48 0 0 1 .72 2.13h.93a9.52 9.52 0 0 0 3-.49l-.93-1.81a7.67 7.67 0 0 1-2.07.35Zm-5.17-1.94l-.36-.38a7.4 7.4 0 0 1-2.2-5.92a7.31 7.31 0 0 1 1.54-4L17.26 9a1 1 0 0 0 .91 1h.09a1 1 0 0 0 1-.91L19.6 5a1 1 0 0 0-.29-.79a1 1 0 0 0-.79-.21l-4.09.35a1 1 0 0 0 .17 2l1.29-.11a9.45 9.45 0 0 0-2.05 5.32a9.28 9.28 0 0 0 2.67 7.26l.31.37a7.33 7.33 0 0 1 2.06 4.91a7.39 7.39 0 0 1-.26 2.47l1.8.91a8.76 8.76 0 0 0 .45-3.51a9.28 9.28 0 0 0-2.51-6.1Zm14.04.04l-1.21.09a9.65 9.65 0 0 0-7.66-15.55a9.33 9.33 0 0 0-3 .49l.91 1.8a7.67 7.67 0 0 1 9.76 7.39a7.58 7.58 0 0 1-1.65 4.72l.1-1.54a1 1 0 1 0-2-.13l-.28 4.08a1 1 0 0 0 .31.78a.94.94 0 0 0 .69.28h.1l4.08-.42a1 1 0 0 0 .9-1.1a1 1 0 0 0-1.05-.89ZM4.07 20.44h.08l4.09-.35a1 1 0 1 0-.17-2l-1.39.12a7.63 7.63 0 0 1 4.52-1.49a7.9 7.9 0 0 1 1.63.18a10.23 10.23 0 0 1-.71-2.13h-.92a9.66 9.66 0 0 0-5.9 2l.12-1.31a1 1 0 0 0-.92-1.08a1 1 0 0 0-1.08.91l-.35 4.08a1 1 0 0 0 1 1.08Zm14.35 7.79l-4.09.27a1 1 0 0 0 .13 2l1.54-.11a7.71 7.71 0 0 1-12.54-6a7.6 7.6 0 0 1 .29-2L2 21.46a9.59 9.59 0 0 0-.47 2.95A9.7 9.7 0 0 0 17.19 32l-.12 1.18a1 1 0 0 0 .89 1.1h.11a1 1 0 0 0 1-.9l.42-4.06a1 1 0 0 0-1.06-1.1Z"/></svg>
    title: CI/CD (coming soon)
    details: Sophisticated and collaborative CI/CD workflows. Review changes in Pull Requests.
  - icon: <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><g fill="currentColor"><path fill-rule="evenodd" d="M14.364 13.121c.924.924 1.12 2.3.586 3.415l1.535 1.535a1 1 0 0 1-1.414 1.414l-1.535-1.535a3.001 3.001 0 0 1-3.415-4.829a3 3 0 0 1 4.243 0ZM12.95 15.95a1 1 0 1 0-1.414-1.414a1 1 0 0 0 1.414 1.414Z" clip-rule="evenodd"/><path d="M8 5h8v2H8V5Zm8 4H8v2h8V9Z"/><path fill-rule="evenodd" d="M4 4a3 3 0 0 1 3-3h10a3 3 0 0 1 3 3v16a3 3 0 0 1-3 3H7a3 3 0 0 1-3-3V4Zm3-1h10a1 1 0 0 1 1 1v16a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1Z" clip-rule="evenodd"/></g></svg>
    title: Asset Inventory Management (coming soon)
    details: Connect all your cloud accounts and identify non-IaC managed resources.
  - icon: <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="currentColor" d="M2 9V5q0-.825.588-1.413T4 3h16q.825 0 1.413.588T22 5v4h-2V5H4v4H2Zm2 9q-.825 0-1.413-.588T2 16v-5h2v5h16v-5h2v5q0 .825-.588 1.413T20 18H4Zm-3 3v-2h22v2H1Zm11-10.5ZM2 11V9h6q.275 0 .525.15t.375.4l1.175 2.325L13.15 6.5q.125-.225.35-.363T14 6q.275 0 .525.138t.375.412L16.125 9H22v2h-6.5q-.275 0-.525-.138t-.375-.412l-.65-1.325l-3.075 5.375q-.125.25-.375.375T9.975 15q-.275 0-.513-.15t-.362-.4L7.375 11H2Z"/></svg>
    title: Observability (coming soon)
    details: Detect and resolve drifts. Observe deployment metrics, stack health and create actionable alerts.
  - icon: <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="currentColor" d="m9.6 15.6l1.4-1.425L8.825 12L11 9.825L9.6 8.4L6 12l3.6 3.6Zm4.8 0L18 12l-3.6-3.6L13 9.825L15.175 12L13 14.175l1.4 1.425ZM5 21q-.825 0-1.413-.588T3 19V5q0-.825.588-1.413T5 3h14q.825 0 1.413.588T21 5v14q0 .825-.588 1.413T19 21H5Zm0-2h14V5H5v14ZM5 5v14V5Z"/></svg>
    title: Codify existing Infrastructure (coming soon)
    details: Codify and import cloud resources into Terraform.
---
