---
title: Tag Filter
description: The tag filter returns a list of stacks containing tags that satisfy the filter query.
---

# Tag Filter

The **Tag Filter** can be used in multiple Terramate features:

- [stack.after](../stacks/configuration.md#stackafter-setstringoptional)
- [stack.before](../stacks/configuration.md#stackbefore-setstringoptional)
- `terramate <cmd> --tags <filter>`

The filter returns a list of stacks containing `tags` that satisfy the filter
query. The query language is best explained with some examples but a formal
definition can be found [here](#filter-grammar).

Let's say the project has multiple stacks and some of them having the tag `abc`,
others having the tag `xyz` and some having of them having both.

Then:

- `abc` selects the stacks containing the tag `abc`.
- `xyz` selects the stacks containing the tag `xyz`.
- `abc:xyz` selects the stacks containing both `abc` **and** `xyz` tags.
- `abc,xyz` selects the stacks containing `abc` **or** `xyz` tags.

The `:` character defines the **AND** operation and the `,` character the **OR**
operation. They can be freely combined but no explicit grouping is supported (yet).

Examples:

- `tf,pulumi,cfn` selects the stacks containing the tags `tf` or `pulumi` or `cfn`.
- `app:k8s:frontend` selects only stacks containing the three tags: `app` && `k8s` && `frontend`.
- `app:k8s,app:nomad` selects only stacks containing the both the tags
`app` **AND** `k8s` or stacks containing both the tags `app` **AND** `nomad`.

## Filter Grammar

Below is the formal grammar definition:

```txt
query         ::= or_term {',' or_term}
or_term       ::= and_term {':' and_term}
and_term      ::= tagname
tagname       ::= ident
ident         ::= allowedchars { allowedchars } | allowedchars
allowedchars  ::= lowercase | digit | '-' | '_'
digit         ::= '0' ... '9'
lowercase     ::= 'a' | 'b' | ... | 'z'
```

The `ident` definition is a simplification and you should refer to
[stack.tags](../stacks/configuration.md#stacktags-setstringoptional) for the correct definition
(in prose) for the expected declaration of tag names.

